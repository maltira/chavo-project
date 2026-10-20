package media

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// S3 - минимальный клиент S3-совместимого хранилища (MinIO): Put, Get, Delete и создание бакета.
// Без SDK: путь в стиле path-style (http://host/<bucket>/<key>), подпись AWS Signature Version 4
type S3 struct {
	endpoint  *url.URL
	bucket    string
	region    string
	accessKey string
	secretKey string
	client    *http.Client
	now       func() time.Time
}

type S3Config struct {
	Endpoint  string // например, http://minio:9000
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

func NewS3(cfg S3Config) (*S3, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid S3 endpoint %q", cfg.Endpoint)
	}
	if cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, fmt.Errorf("S3 bucket and credentials are required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	return &S3{
		endpoint: u, bucket: cfg.Bucket, region: region,
		accessKey: cfg.AccessKey, secretKey: cfg.SecretKey,
		client: &http.Client{Timeout: 15 * time.Second},
		now:    time.Now,
	}, nil
}

func (s *S3) objectURL(key string) string {
	u := *s.endpoint
	u.Path = "/" + s.bucket
	if key != "" {
		u.Path += "/" + key
	}
	return u.String()
}

func (s *S3) do(ctx context.Context, method, key string, body []byte, header http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.objectURL(key), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	sum := sha256.Sum256(body)
	signV4(req, hex.EncodeToString(sum[:]), s.accessKey, s.secretKey, s.region, s.now())
	return s.client.Do(req)
}

func (s *S3) Put(ctx context.Context, key string, data []byte, contentType string) error {
	h := http.Header{}
	h.Set("Content-Type", contentType)
	// имена файлов случайные и никогда не переиспользуются - кешировать можно навсегда
	h.Set("Cache-Control", "public, max-age=31536000, immutable")
	resp, err := s.do(ctx, http.MethodPut, key, data, h)
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("put object: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	resp, err := s.do(ctx, http.MethodGet, key, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusNotFound:
		drain(resp)
		return nil, ErrNotFound
	default:
		drain(resp)
		return nil, fmt.Errorf("get object: unexpected status %d", resp.StatusCode)
	}
}

func (s *S3) Delete(ctx context.Context, key string) error {
	resp, err := s.do(ctx, http.MethodDelete, key, nil, nil)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	defer drain(resp)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete object: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// EnsureBucket создаёт приватный бакет, если его ещё нет (анонимного доступа к нему нет: файлы отдаёт Gateway)
func (s *S3) EnsureBucket(ctx context.Context) error {
	resp, err := s.do(ctx, http.MethodHead, "", nil, nil)
	if err != nil {
		return fmt.Errorf("head bucket: %w", err)
	}
	drain(resp)
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
	default:
		return fmt.Errorf("head bucket: unexpected status %d", resp.StatusCode)
	}

	resp, err = s.do(ctx, http.MethodPut, "", nil, nil)
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	drain(resp)
	// бакет мог создать параллельный запуск - это не ошибка
	if resp.StatusCode == http.StatusOK || (resp.StatusCode == http.StatusConflict && bytes.Contains(body, []byte("BucketAlreadyOwnedByYou"))) {
		return nil
	}
	return fmt.Errorf("create bucket: unexpected status %d", resp.StatusCode)
}

func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
}

// signV4 подписывает запрос по AWS Signature Version 4 (заголовочная подпись, сервис s3):
// подписываются host и все заголовки запроса; payloadHash - hex SHA-256 тела
func signV4(req *http.Request, payloadHash, accessKey, secretKey, region string, now time.Time) {
	t := now.UTC()
	amzDate := t.Format("20060102T150405Z")
	day := t.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	headers := map[string]string{"host": host}
	for k, v := range req.Header {
		headers[strings.ToLower(k)] = strings.Join(trimAll(v), ",")
	}
	names := make([]string, 0, len(headers))
	for k := range headers {
		names = append(names, k)
	}
	sort.Strings(names)
	var canonHeaders strings.Builder
	for _, k := range names {
		canonHeaders.WriteString(k + ":" + headers[k] + "\n")
	}
	signed := strings.Join(names, ";")

	canonical := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL),
		canonicalQuery(req.URL),
		canonHeaders.String(),
		signed,
		payloadHash,
	}, "\n")
	scope := day + "/" + region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(canonical))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])

	key := hmacSHA256([]byte("AWS4"+secretKey), day)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, "s3")
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, toSign))

	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+accessKey+"/"+scope+
		", SignedHeaders="+signed+", Signature="+signature)
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func trimAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.Join(strings.Fields(v), " ")
	}
	return out
}

// canonicalURI - путь, где каждый сегмент закодирован по RFC 3986 (для S3 - один раз, «/» не кодируется)
func canonicalURI(u *url.URL) string {
	p := u.Path
	if p == "" {
		return "/"
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = uriEncode(s)
	}
	return strings.Join(segs, "/")
}

func canonicalQuery(u *url.URL) string {
	q := u.Query()
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		vs := append([]string(nil), q[k]...)
		sort.Strings(vs)
		for _, v := range vs {
			parts = append(parts, uriEncode(k)+"="+uriEncode(v))
		}
	}
	return strings.Join(parts, "&")
}

func uriEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
