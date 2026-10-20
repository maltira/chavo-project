package api_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/media"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/ratelimit"
)

// memStore - хранилище в памяти вместо MinIO
type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *memStore) Put(_ context.Context, key string, data []byte, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = append([]byte(nil), data...)
	return nil
}

func (s *memStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.objects[key]
	if !ok {
		return nil, media.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *memStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

func (s *memStore) keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0, len(s.objects))
	for k := range s.objects {
		keys = append(keys, k)
	}
	return keys
}

var avatarURLRe = regexp.MustCompile(`^/media/avatars/[0-9a-f]{32}\.jpg$`)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{217, 119, 87, 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type part struct {
	field, filename, contentType string
	data                         []byte
}

// upload - multipart-запрос; имя файла и тип части клиент может подделать, Gateway их не использует
func (e *env) upload(method, path, token string, parts ...part) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, p := range parts {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{`form-data; name="` + p.field + `"; filename="` + p.filename + `"`}
		h["Content-Type"] = []string{p.contentType}
		w, _ := mw.CreatePart(h)
		_, _ = w.Write(p.data)
	}
	_ = mw.Close()
	r := httptest.NewRequest(method, path, &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, r)
	return w
}

func photo(t *testing.T) part {
	return part{field: "file", filename: "me.png", contentType: "image/png", data: pngBytes(t, 200, 150)}
}

func TestUploadMyAvatarReplacesTheOldFile(t *testing.T) {
	e := newEnv(t)
	_, token := e.login(t, true)

	w := e.upload(http.MethodPost, "/api/users/me/avatar", token, photo(t))
	expect(t, w, http.StatusOK, "")
	first, _ := decode(t, w)["avatar_url"].(string)
	if !avatarURLRe.MatchString(first) {
		t.Fatalf("avatar_url: %q", first)
	}
	if keys := e.media.keys(); len(keys) != 1 || "/media/"+keys[0] != first {
		t.Fatalf("stored: %v", keys)
	}

	// замена: новый файл, прежний удалён из хранилища
	w = e.upload(http.MethodPost, "/api/users/me/avatar", token, photo(t))
	expect(t, w, http.StatusOK, "")
	second, _ := decode(t, w)["avatar_url"].(string)
	if second == first {
		t.Fatal("a new upload must get a new name")
	}
	if keys := e.media.keys(); len(keys) != 1 || "/media/"+keys[0] != second {
		t.Fatalf("old file must be deleted, stored: %v", keys)
	}

	// отдача: наш JPEG и заголовки, запрещающие исполнение
	r := e.do(http.MethodGet, second, "", "")
	if r.Code != http.StatusOK || r.Header().Get("Content-Type") != "image/jpeg" ||
		r.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(r.Header().Get("Content-Security-Policy"), "default-src 'none'") ||
		!strings.Contains(r.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("serve: %d %v", r.Code, r.Header())
	}
	if !bytes.HasPrefix(r.Body.Bytes(), []byte{0xFF, 0xD8}) {
		t.Fatal("served body must be JPEG")
	}

	// удаление: фото убрано из профиля и из хранилища
	expect(t, e.do(http.MethodDelete, "/api/users/me/avatar", token, ""), http.StatusOK, "")
	if keys := e.media.keys(); len(keys) != 0 {
		t.Fatalf("after delete: %v", keys)
	}
	if r = e.do(http.MethodGet, second, "", ""); r.Code != http.StatusNotFound {
		t.Fatalf("deleted file served: %d", r.Code)
	}
}

func TestUploadRejectsNonImagesWhateverTheyClaim(t *testing.T) {
	e := newEnv(t)
	_, token := e.login(t, true)
	elf := append([]byte("\x7fELF\x02\x01\x01\x00"), bytes.Repeat([]byte{0}, 128)...)

	cases := []struct {
		name   string
		parts  []part
		status int
		reason string
	}{
		{"executable named as photo", []part{{"file", "photo.jpg", "image/jpeg", elf}}, http.StatusUnsupportedMediaType, "AVATAR_TYPE_UNSUPPORTED"},
		{"script", []part{{"file", "a.jpg.sh", "image/jpeg", []byte("#!/bin/sh\nrm -rf ~\n")}}, http.StatusUnsupportedMediaType, "AVATAR_TYPE_UNSUPPORTED"},
		{"html", []part{{"file", "x.png", "image/png", []byte("<html><script>alert(1)</script></html>")}}, http.StatusUnsupportedMediaType, "AVATAR_TYPE_UNSUPPORTED"},
		{"svg", []part{{"file", "x.svg", "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)}}, http.StatusUnsupportedMediaType, "AVATAR_TYPE_UNSUPPORTED"},
		{"tiny image", []part{{"file", "a.png", "image/png", pngBytes(t, 8, 8)}}, http.StatusBadRequest, "AVATAR_INVALID"},
		{"no file field", []part{{"avatar", "a.png", "image/png", pngBytes(t, 100, 100)}}, http.StatusBadRequest, "INVALID_REQUEST"},
		{"too many parts", []part{{"a", "", "text/plain", []byte("1")}, {"b", "", "text/plain", []byte("2")}, {"c", "", "text/plain", []byte("3")}, {"d", "", "text/plain", []byte("4")}, photo(t)}, http.StatusBadRequest, "INVALID_REQUEST"},
		{"too large", []part{{"file", "big.png", "image/png", make([]byte, 2<<20)}}, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE"},
	}
	for _, c := range cases {
		w := e.upload(http.MethodPost, "/api/users/me/avatar", token, c.parts...)
		if w.Code != c.status || decode(t, w)["reason"] != c.reason {
			t.Errorf("%s: %d %s", c.name, w.Code, w.Body.String())
		}
	}
	if keys := e.media.keys(); len(keys) != 0 {
		t.Fatalf("nothing must be stored, got %v", keys)
	}

	// не multipart вообще
	expect(t, e.do(http.MethodPost, "/api/users/me/avatar", token, `{"avatar_url":"https://tracker.example/p.gif"}`), http.StatusBadRequest, "INVALID_REQUEST")
}

func TestGroupAvatarOnlyForAdminAndRolledBackOtherwise(t *testing.T) {
	e := newEnv(t)
	admin, adminToken := e.login(t, true)
	_, memberToken := e.login(t, true)
	group := uuid.NewString()
	e.convs.admins[group] = admin

	// не админ: сервис отказывает, а уже загруженный файл удаляется
	w := e.upload(http.MethodPost, "/api/conversations/"+group+"/avatar", memberToken, photo(t))
	expect(t, w, http.StatusForbidden, "")
	if keys := e.media.keys(); len(keys) != 0 {
		t.Fatalf("rejected upload must be rolled back: %v", keys)
	}

	w = e.upload(http.MethodPost, "/api/conversations/"+group+"/avatar", adminToken, photo(t))
	expect(t, w, http.StatusOK, "")
	url, _ := decode(t, w)["avatar_url"].(string)
	if !avatarURLRe.MatchString(url) || e.convs.avatars[group] != url {
		t.Fatalf("group avatar: %q / %q", url, e.convs.avatars[group])
	}

	expect(t, e.do(http.MethodDelete, "/api/conversations/"+group+"/avatar", adminToken, ""), http.StatusOK, "")
	if keys := e.media.keys(); len(keys) != 0 || e.convs.avatars[group] != "" {
		t.Fatalf("after delete: %v %q", keys, e.convs.avatars[group])
	}

	// чужая или несуществующая группа
	w = e.upload(http.MethodPost, "/api/conversations/"+uuid.NewString()+"/avatar", adminToken, photo(t))
	expect(t, w, http.StatusNotFound, "")
}

func TestServeAvatarRejectsForeignNames(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{
		"/media/avatars/..%2F..%2Fetc%2Fpasswd",
		"/media/avatars/x.jpg",
		"/media/avatars/" + strings.Repeat("a", 32) + ".png",
		"/media/avatars/" + strings.Repeat("a", 32) + ".jpg", // формат верный, но файла нет
	} {
		if w := e.do(http.MethodGet, path, "", ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", path, w.Code)
		}
	}
}

func TestUploadRateLimit(t *testing.T) {
	limits := generousLimits
	limits.Upload = ratelimit.Rule{Name: "upload", Limit: 1, Window: time.Minute}
	e := newEnvWithLimits(t, limits)
	_, token := e.login(t, true)
	expect(t, e.upload(http.MethodPost, "/api/users/me/avatar", token, photo(t)), http.StatusOK, "")
	expect(t, e.upload(http.MethodPost, "/api/users/me/avatar", token, photo(t)), http.StatusTooManyRequests, "RATE_LIMITED")
}

var _ media.Store = (*memStore)(nil)
