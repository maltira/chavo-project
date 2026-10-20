package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

// Проверка клиента на настоящем S3-сервере (MinIO): подпись, создание бакета, put/get/delete.
// Запуск: S3_TEST_ENDPOINT=http://127.0.0.1:9000 S3_TEST_ACCESS_KEY=... S3_TEST_SECRET_KEY=... go test ./internal/media -run S3Real
// Без S3_TEST_ENDPOINT тест пропускается.
func TestS3RealServer(t *testing.T) {
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_TEST_ENDPOINT is not set")
	}
	s, err := NewS3(S3Config{
		Endpoint:  endpoint,
		Bucket:    "chavo-media-test",
		AccessKey: os.Getenv("S3_TEST_ACCESS_KEY"),
		SecretKey: os.Getenv("S3_TEST_SECRET_KEY"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err = s.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}
	if err = s.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket twice: %v", err)
	}

	key := "avatars/" + "0123456789abcdef0123456789abcdef.jpg"
	data := append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{7}, 4096)...)
	if err = s.Put(ctx, key, data, "image/jpeg"); err != nil {
		t.Fatalf("put: %v", err)
	}
	body, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	got, _ := io.ReadAll(body)
	_ = body.Close()
	if !bytes.Equal(got, data) {
		t.Fatalf("content differs: %d bytes", len(got))
	}
	if err = s.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err = s.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	if err = s.Delete(ctx, key); err != nil {
		t.Fatalf("delete missing: %v", err)
	}

	// неверный ключ доступа - отказ, а не молчаливый успех
	bad, _ := NewS3(S3Config{Endpoint: endpoint, Bucket: "chavo-media-test", AccessKey: os.Getenv("S3_TEST_ACCESS_KEY"), SecretKey: "wrong-secret-key"})
	if err = bad.Put(ctx, key, data, "image/jpeg"); err == nil {
		t.Fatal("wrong credentials must be rejected")
	}
}
