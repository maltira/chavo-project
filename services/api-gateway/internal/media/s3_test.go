package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	exampleAccess = "AKIAIOSFODNN7EXAMPLE"
	exampleSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
)

func signature(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	return auth[strings.LastIndex(auth, "Signature=")+len("Signature="):]
}

// примеры из документации AWS «Signature Calculations for the Authorization Header»
func TestSignV4MatchesAWSExamples(t *testing.T) {
	at := time.Date(2013, 5, 24, 0, 0, 0, 0, time.UTC)
	emptyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	get, _ := http.NewRequest(http.MethodGet, "https://examplebucket.s3.amazonaws.com/test.txt", nil)
	get.Header.Set("Range", "bytes=0-9")
	signV4(get, emptyHash, exampleAccess, exampleSecret, "us-east-1", at)
	if got := signature(get); got != "f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41" {
		t.Fatalf("GET object signature: %s", got)
	}
	if !strings.Contains(get.Header.Get("Authorization"), "SignedHeaders=host;range;x-amz-content-sha256;x-amz-date") {
		t.Fatalf("signed headers: %s", get.Header.Get("Authorization"))
	}

	put, _ := http.NewRequest(http.MethodPut, "https://examplebucket.s3.amazonaws.com/test$file.text", strings.NewReader("Welcome to Amazon S3."))
	put.Header.Set("Date", "Fri, 24 May 2013 00:00:00 GMT")
	put.Header.Set("X-Amz-Storage-Class", "REDUCED_REDUNDANCY")
	signV4(put, "44ce7dd67c959e0d3524ffac1771dfbba87d2b6b4b4e99e42034a8b803f8b072", exampleAccess, exampleSecret, "us-east-1", at)
	if got := signature(put); got != "98ad721746da40c64f1a55b78f14c238d841ea1380cd77a1b5971af0ece108bd" {
		t.Fatalf("PUT object signature: %s", got)
	}
}

// fakeS3 - S3 в памяти: проверяет, что запросы подписаны, и хранит объекты по пути
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
	buckets map[string]bool
}

func newFakeS3(t *testing.T) (*fakeS3, *S3) {
	t.Helper()
	f := &fakeS3{objects: map[string][]byte{}, types: map[string]string{}, buckets: map[string]bool{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	s, err := NewS3(S3Config{Endpoint: srv.URL, Bucket: "media", AccessKey: "ak", SecretKey: "sk"})
	if err != nil {
		t.Fatal(err)
	}
	return f, s
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=ak/") || r.Header.Get("X-Amz-Date") == "" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key, _ := strings.Cut(path, "/")
	if key == "" {
		switch r.Method {
		case http.MethodHead:
			if !f.buckets[bucket] {
				w.WriteHeader(http.StatusNotFound)
			}
		case http.MethodPut:
			f.buckets[bucket] = true
		}
		return
	}
	switch r.Method {
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.objects[path] = body
		f.types[path] = r.Header.Get("Content-Type")
	case http.MethodGet:
		body, ok := f.objects[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(body)
	case http.MethodDelete:
		delete(f.objects, path)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (f *fakeS3) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.objects)
}

func TestS3PutGetDelete(t *testing.T) {
	f, s := newFakeS3(t)
	ctx := context.Background()

	if err := s.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureBucket(ctx); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if err := s.Put(ctx, "avatars/a.jpg", []byte("jpeg"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if f.types["media/avatars/a.jpg"] != "image/jpeg" {
		t.Fatalf("content type: %q", f.types["media/avatars/a.jpg"])
	}
	body, err := s.Get(ctx, "avatars/a.jpg")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(body)
	_ = body.Close()
	if string(got) != "jpeg" {
		t.Fatalf("get: %q", got)
	}
	if err = s.Delete(ctx, "avatars/a.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(ctx, "avatars/a.jpg"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	// удаление отсутствующего - не ошибка
	if err = s.Delete(ctx, "avatars/missing.jpg"); err != nil {
		t.Fatal(err)
	}
}

func TestNewS3RequiresConfig(t *testing.T) {
	for _, cfg := range []S3Config{
		{Endpoint: "minio:9000", Bucket: "b", AccessKey: "a", SecretKey: "s"},
		{Endpoint: "http://minio:9000", AccessKey: "a", SecretKey: "s"},
		{Endpoint: "http://minio:9000", Bucket: "b"},
	} {
		if _, err := NewS3(cfg); err == nil {
			t.Fatalf("config must be rejected: %+v", cfg)
		}
	}
}
