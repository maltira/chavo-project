// Package linkpreview загружает OpenGraph-метаданные ссылок с защитой от SSRF и кешем в Redis.
package linkpreview

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

const (
	cacheKeyPrefix   = "linkpreview:"
	positiveCacheTTL = 24 * time.Hour
	negativeCacheTTL = 10 * time.Minute
	fetchTimeout     = 4 * time.Second
	maxBodyBytes     = 1 << 20 // 1 МБ
	maxRedirects     = 3
	maxURLLength     = 2048
	maxTitleRunes    = 300
	maxDescRunes     = 500
	maxSiteRunes     = 120
)

var (
	// ErrInvalidURL - URL не подходит (схема не http/https, пустой хост или логин/пароль в URL)
	ErrInvalidURL = errors.New("invalid preview url")
	// ErrBlockedTarget - целевой хост резолвится в локальный/приватный/зарезервированный IP
	ErrBlockedTarget = errors.New("blocked preview target")
)

// Preview - карточка превью ссылки
type Preview struct {
	URL         string  `json:"url"`
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	ImageURL    *string `json:"image_url,omitempty"`
	SiteName    *string `json:"site_name,omitempty"`
}

// Cache хранит сериализованные превью по ключу URL
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type cachedEntry struct {
	Empty   bool     `json:"empty,omitempty"`
	Preview *Preview `json:"preview,omitempty"`
}

type redisCache struct {
	rdb *redis.Client
}

// NewRedisCache создаёт Redis-кеш превью ссылок
func NewRedisCache(rdb *redis.Client) Cache {
	return &redisCache{rdb: rdb}
}

func (c *redisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (c *redisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Fetcher загружает и кеширует OpenGraph-метаданные
type Fetcher struct {
	client *http.Client
	cache  Cache
	log    *zap.Logger
}

// NewFetcher создаёт Fetcher с жёсткой SSRF-защитой на уровне DialContext и редиректов
func NewFetcher(cache Cache, log *zap.Logger) *Fetcher {
	if log == nil {
		log = zap.NewNop()
	}
	dialer := &net.Dialer{Timeout: 3 * time.Second}
	transport := &http.Transport{
		DialContext:           safeDialContext(dialer, net.DefaultResolver),
		TLSHandshakeTimeout:   3 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		DisableKeepAlives:     true,
		MaxIdleConns:          0,
	}
	return newFetcherWithTransport(transport, cache, log)
}

func newFetcherWithTransport(rt http.RoundTripper, cache Cache, log *zap.Logger) *Fetcher {
	client := &http.Client{
		Transport: rt,
		Timeout:   fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			if _, err := parsePublicURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
	return &Fetcher{client: client, cache: cache, log: log}
}

var extraBlockedPrefixes = func() []netip.Prefix {
	raw := []string{
		"100.64.0.0/10",   // CGNAT (RFC 6598)
		"192.0.0.0/24",    // IETF protocol assignments
		"192.0.2.0/24",    // TEST-NET-1
		"198.18.0.0/15",   // benchmark
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // reserved / broadcast
		"2001:db8::/32",   // IPv6 documentation
	}
	out := make([]netip.Prefix, 0, len(raw))
	for _, s := range raw {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// isBlockedIP запрещает любые локальные, приватные, link-local, multicast и служебные адреса
func isBlockedIP(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, p := range extraBlockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

type ipResolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

func safeDialContext(dialer *net.Dialer, resolver ipResolver) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, ErrInvalidURL
		}
		var addrs []netip.Addr
		if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
			addrs = []netip.Addr{ip}
		} else {
			addrs, err = resolver.LookupNetIP(ctx, "ip", host)
			if err != nil || len(addrs) == 0 {
				return nil, fmt.Errorf("resolve host: %w", err)
			}
		}
		// блокируем хост, если хотя бы один его IP внутренний (защита от DNS rebinding)
		for _, a := range addrs {
			if isBlockedIP(a) {
				return nil, ErrBlockedTarget
			}
		}
		var lastErr error
		for _, a := range addrs {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(a.Unmap().String(), port))
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}
}

func parsePublicURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > maxURLLength {
		return nil, ErrInvalidURL
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return nil, ErrInvalidURL
		}
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return nil, ErrInvalidURL
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Hostname() == "" || u.User != nil {
		return nil, ErrInvalidURL
	}
	u.Scheme = scheme
	u.Fragment = ""
	return u, nil
}

func cacheKey(normalizedURL string) string {
	sum := sha256.Sum256([]byte(normalizedURL))
	return cacheKeyPrefix + hex.EncodeToString(sum[:])
}

// Get возвращает превью для URL из кеша или загружает его; отсутствие превью - (nil, nil)
func (f *Fetcher) Get(ctx context.Context, rawURL string) (*Preview, error) {
	u, err := parsePublicURL(rawURL)
	if err != nil {
		return nil, err
	}
	normalized := u.String()
	key := cacheKey(normalized)

	if f.cache != nil {
		if raw, hit, err := f.cache.Get(ctx, key); err != nil {
			f.log.Warn("link preview cache read failed", zap.Error(err))
		} else if hit {
			var entry cachedEntry
			if err := json.Unmarshal(raw, &entry); err == nil {
				if entry.Empty {
					return nil, nil
				}
				return entry.Preview, nil
			}
		}
	}

	preview, fetchErr := f.fetchRemote(ctx, normalized)
	if fetchErr != nil {
		// блокированный или битый URL тоже кешируем ненадолго, чтобы не долбить его повторно
		f.saveCache(ctx, key, cachedEntry{Empty: true}, negativeCacheTTL)
		return nil, fetchErr
	}
	if preview == nil {
		f.saveCache(ctx, key, cachedEntry{Empty: true}, negativeCacheTTL)
		return nil, nil
	}
	f.saveCache(ctx, key, cachedEntry{Preview: preview}, positiveCacheTTL)
	return preview, nil
}

func (f *Fetcher) saveCache(ctx context.Context, key string, entry cachedEntry, ttl time.Duration) {
	if f.cache == nil {
		return
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return
	}
	if err = f.cache.Set(ctx, key, raw, ttl); err != nil {
		f.log.Warn("link preview cache write failed", zap.Error(err))
	}
}

func (f *Fetcher) fetchRemote(ctx context.Context, targetURL string) (*Preview, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ChavoLinkPreview/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || (mediaType != "text/html" && mediaType != "application/xhtml+xml") {
			return nil, nil
		}
	}

	finalURL := resp.Request.URL
	if finalURL == nil {
		finalURL, _ = url.Parse(targetURL)
	}
	body := io.LimitReader(resp.Body, maxBodyBytes)
	return parseHTMLPreview(body, finalURL), nil
}

func parseHTMLPreview(r io.Reader, pageURL *url.URL) *Preview {
	z := html.NewTokenizer(r)
	var (
		ogTitle, twTitle, docTitle string
		ogDesc, twDesc, metaDesc   string
		ogImage, twImage           string
		ogSiteName                 string
		inTitle                    bool
	)

	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return buildPreview(pageURL, ogTitle, twTitle, docTitle, ogDesc, twDesc, metaDesc, ogImage, twImage, ogSiteName)
		case html.StartTagToken, html.SelfClosingTagToken:
			nameBytes, hasAttr := z.TagName()
			tag := strings.ToLower(string(nameBytes))
			if tag == "body" {
				// метаданные OpenGraph находятся в <head>
				return buildPreview(pageURL, ogTitle, twTitle, docTitle, ogDesc, twDesc, metaDesc, ogImage, twImage, ogSiteName)
			}
			if tag == "title" && tt == html.StartTagToken {
				inTitle = true
				continue
			}
			if tag != "meta" || !hasAttr {
				continue
			}
			var prop, name, content string
			for {
				k, v, more := z.TagAttr()
				key := strings.ToLower(string(k))
				switch key {
				case "property":
					prop = strings.ToLower(strings.TrimSpace(string(v)))
				case "name":
					name = strings.ToLower(strings.TrimSpace(string(v)))
				case "content":
					content = strings.TrimSpace(string(v))
				}
				if !more {
					break
				}
			}
			if content == "" {
				continue
			}
			key := prop
			if key == "" {
				key = name
			}
			switch key {
			case "og:title":
				if ogTitle == "" {
					ogTitle = content
				}
			case "twitter:title":
				if twTitle == "" {
					twTitle = content
				}
			case "og:description":
				if ogDesc == "" {
					ogDesc = content
				}
			case "twitter:description":
				if twDesc == "" {
					twDesc = content
				}
			case "description":
				if metaDesc == "" {
					metaDesc = content
				}
			case "og:image", "og:image:secure_url":
				if ogImage == "" {
					ogImage = content
				}
			case "twitter:image", "twitter:image:src":
				if twImage == "" {
					twImage = content
				}
			case "og:site_name":
				if ogSiteName == "" {
					ogSiteName = content
				}
			}
		case html.EndTagToken:
			nameBytes, _ := z.TagName()
			tag := strings.ToLower(string(nameBytes))
			if tag == "title" {
				inTitle = false
			} else if tag == "head" {
				return buildPreview(pageURL, ogTitle, twTitle, docTitle, ogDesc, twDesc, metaDesc, ogImage, twImage, ogSiteName)
			}
		case html.TextToken:
			if inTitle && docTitle == "" {
				docTitle = strings.TrimSpace(string(z.Text()))
			}
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := cleanText(v); s != "" {
			return s
		}
	}
	return ""
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func clipPtr(s string, maxRunes int) *string {
	s = cleanText(s)
	if s == "" {
		return nil
	}
	if utf8.RuneCountInString(s) > maxRunes {
		runes := []rune(s)
		s = strings.TrimSpace(string(runes[:maxRunes]))
	}
	return &s
}

func resolveImageURL(raw string, base *url.URL) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	if base != nil {
		ref = base.ResolveReference(ref)
	}
	u, err := parsePublicURL(ref.String())
	if err != nil {
		return nil
	}
	s := u.String()
	return &s
}

func buildPreview(pageURL *url.URL, ogTitle, twTitle, docTitle, ogDesc, twDesc, metaDesc, ogImage, twImage, ogSiteName string) *Preview {
	title := clipPtr(firstNonEmpty(ogTitle, twTitle, docTitle), maxTitleRunes)
	desc := clipPtr(firstNonEmpty(ogDesc, twDesc, metaDesc), maxDescRunes)
	img := resolveImageURL(firstNonEmpty(ogImage, twImage), pageURL)
	if title == nil && desc == nil && img == nil {
		return nil
	}
	site := firstNonEmpty(ogSiteName)
	if site == "" && pageURL != nil {
		site = strings.TrimPrefix(strings.ToLower(pageURL.Hostname()), "www.")
	}
	return &Preview{
		URL:         pageURL.String(),
		Title:       title,
		Description: desc,
		ImageURL:    img,
		SiteName:    clipPtr(site, maxSiteRunes),
	}
}

var plainURLRe = regexp.MustCompile(`https?://[^\s<>"]+`)

// ExtractFirstURL находит первую подходящую http/https-ссылку из text_link-сущностей или текста сообщения
func ExtractFirstURL(content string, textLinkURLs []string) string {
	for _, raw := range textLinkURLs {
		if u, err := parsePublicURL(raw); err == nil {
			return u.String()
		}
	}
	for _, match := range plainURLRe.FindAllString(content, 5) {
		cleaned := strings.TrimRight(match, ".,!?:;)'\"]>")
		if u, err := parsePublicURL(cleaned); err == nil {
			return u.String()
		}
	}
	return ""
}
