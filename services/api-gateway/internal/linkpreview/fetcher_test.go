package linkpreview

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

type memCache struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (m *memCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.data[key]
	return v, ok, nil
}

func (m *memCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1",
		"127.255.255.254",
		"0.0.0.0",
		"10.0.0.1",
		"172.16.0.1",
		"192.168.1.1",
		"169.254.169.254",
		"100.64.0.1",
		"198.18.0.1",
		"224.0.0.1",
		"::1",
		"::",
		"fc00::1",
		"fe80::1",
		"::ffff:127.0.0.1",
		"::ffff:169.254.169.254",
		"::ffff:10.0.0.5",
		"2001:db8::1",
	}
	for _, s := range blocked {
		ip := netip.MustParseAddr(s)
		if !isBlockedIP(ip) {
			t.Errorf("IP %s must be blocked", s)
		}
	}

	allowed := []string{
		"93.184.216.34",
		"1.1.1.1",
		"8.8.8.8",
		"2606:2800:220:1:248:1893:25c8:1946",
	}
	for _, s := range allowed {
		ip := netip.MustParseAddr(s)
		if isBlockedIP(ip) {
			t.Errorf("public IP %s must be allowed", s)
		}
	}
}

func TestFetcherBlocksSSRFAndInvalidURLs(t *testing.T) {
	f := NewFetcher(&memCache{data: map[string][]byte{}}, zap.NewNop())
	ctx := context.Background()

	// loopback сервер на localhost должен блокироваться до соединения
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("SSRF request must not reach loopback server")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, err := f.Get(ctx, srv.URL); !errors.Is(err, ErrBlockedTarget) {
		t.Fatalf("loopback URL: got %v, want ErrBlockedTarget", err)
	}

	for _, bad := range []string{
		"",
		"javascript:alert(1)",
		"file:///etc/passwd",
		"ftp://example.com/file",
		"https://user:pass@example.com/",
		"http://",
	} {
		if _, err := f.Get(ctx, bad); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("bad url %q: got %v, want ErrInvalidURL", bad, err)
		}
	}
}

func TestFetcherParsesOpenGraphAndCaches(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/og":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head>
				<meta property="og:title" content="  Chavo   Messenger ">
				<meta property="og:description" content="Быстрый чат">
				<meta property="og:image" content="/assets/og.png">
				<meta property="og:site_name" content="Chavo">
			</head><body>ignored</body></html>`))
		case "/fallback":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><title>Заголовок страницы</title><meta name="description" content="Описание"><meta property="og:image" content="javascript:alert(1)"></head></html>`))
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	cache := &memCache{data: map[string][]byte{}}
	// для теста парсинга и кеша используем транспорт тестового сервера
	f := newFetcherWithTransport(srv.Client().Transport, cache, zap.NewNop())
	ctx := context.Background()

	p, err := f.Get(ctx, srv.URL+"/og")
	if err != nil || p == nil {
		t.Fatalf("Get /og = (%+v, %v)", p, err)
	}
	if p.Title == nil || *p.Title != "Chavo Messenger" || p.Description == nil || *p.Description != "Быстрый чат" ||
		p.SiteName == nil || *p.SiteName != "Chavo" || p.ImageURL == nil || *p.ImageURL != srv.URL+"/assets/og.png" {
		t.Fatalf("unexpected preview: %+v", p)
	}

	// второй запрос должен отдаться из кеша без похода на сервер
	p2, err := f.Get(ctx, srv.URL+"/og")
	if err != nil || p2 == nil || hits != 1 {
		t.Fatalf("cache hit failed: hits=%d p=%+v err=%v", hits, p2, err)
	}

	// fallback на <title> и фильтрация javascript: в og:image
	fb, err := f.Get(ctx, srv.URL+"/fallback")
	if err != nil || fb == nil || fb.Title == nil || *fb.Title != "Заголовок страницы" || fb.ImageURL != nil {
		t.Fatalf("fallback preview: %+v %v", fb, err)
	}

	// не-HTML ответ даёт nil и кешируется как пустой
	nonHTML, err := f.Get(ctx, srv.URL+"/json")
	if err != nil || nonHTML != nil {
		t.Fatalf("non-html: %+v %v", nonHTML, err)
	}
	prevHits := hits
	if _, err := f.Get(ctx, srv.URL+"/json"); err != nil || hits != prevHits {
		t.Fatalf("negative cache miss: hits=%d prev=%d err=%v", hits, prevHits, err)
	}
}

func TestParseHTMLPreviewRelativeImage(t *testing.T) {
	base, _ := url.Parse("https://example.com/blog/post")
	p := parseHTMLPreview(strings.NewReader(`<head><meta property="og:title" content="Post"><meta property="og:image" content="../img/cover.jpg"></head>`), base)
	if p == nil || p.ImageURL == nil || *p.ImageURL != "https://example.com/img/cover.jpg" {
		t.Fatalf("resolved image = %+v", p)
	}
}

func TestExtractFirstURL(t *testing.T) {
	if got := ExtractFirstURL("глянь https://example.com/a?x=1, круто!", nil); got != "https://example.com/a?x=1" {
		t.Fatalf("plain url = %q", got)
	}
	if got := ExtractFirstURL("текст без ссылки", []string{"javascript:alert(1)", "https://docs.example.com"}); got != "https://docs.example.com" {
		t.Fatalf("entity url = %q", got)
	}
	if got := ExtractFirstURL("просто текст", nil); got != "" {
		t.Fatalf("no url = %q", got)
	}
}
