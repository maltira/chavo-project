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
	"slices"
	"sort"
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

// assertStored - в хранилище ровно эти аватарки, по две версии каждой: маленькая и крупная (_l)
func assertStored(t *testing.T, s *memStore, urls ...string) {
	t.Helper()
	var want []string
	for _, url := range urls {
		want = append(want,
			strings.TrimPrefix(url, "/media/"),
			strings.TrimSuffix(strings.TrimPrefix(url, "/media/"), ".jpg")+"_l.jpg")
	}
	keys := s.keys()
	sort.Strings(keys)
	sort.Strings(want)
	if !slices.Equal(keys, want) {
		t.Fatalf("stored %v, want %v", keys, want)
	}
}

func photo(t *testing.T) part {
	return part{field: "file", filename: "me.png", contentType: "image/png", data: pngBytes(t, 640, 600)}
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
	// в хранилище обе версии: маленькая по пути из ответа и крупная с суффиксом _l
	assertStored(t, e.media, first)

	// замена: новый файл становится текущим, прежний остаётся в истории (обе версии каждого)
	w = e.upload(http.MethodPost, "/api/users/me/avatar", token, photo(t))
	expect(t, w, http.StatusOK, "")
	second, _ := decode(t, w)["avatar_url"].(string)
	if second == first {
		t.Fatal("a new upload must get a new name")
	}
	assertStored(t, e.media, first, second)
	// крупная версия отдаётся так же и по тем же правилам
	large := strings.TrimSuffix(second, ".jpg") + "_l.jpg"
	lr := e.do(http.MethodGet, large, "", "")
	if lr.Code != http.StatusOK || lr.Header().Get("Content-Type") != "image/jpeg" ||
		lr.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.HasPrefix(lr.Body.Bytes(), []byte{0xFF, 0xD8}) {
		t.Fatalf("large: %d %v", lr.Code, lr.Header())
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(lr.Body.Bytes()))
	if err != nil || cfg.Width != 600 {
		t.Fatalf("large version of a 640x600 photo must be its 600px square, got %v %+v", err, cfg)
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

	// удаление убирает текущее фото из истории и из хранилища; текущим становится предыдущее
	expect(t, e.do(http.MethodDelete, "/api/users/me/avatar", token, ""), http.StatusOK, "")
	assertStored(t, e.media, first)
	if r = e.do(http.MethodGet, second, "", ""); r.Code != http.StatusNotFound {
		t.Fatalf("deleted file served: %d", r.Code)
	}
	expect(t, e.do(http.MethodDelete, "/api/users/me/avatar", token, ""), http.StatusOK, "")
	if keys := e.media.keys(); len(keys) != 0 {
		t.Fatalf("after deleting everything: %v", keys)
	}
	// удалять больше нечего
	expect(t, e.do(http.MethodDelete, "/api/users/me/avatar", token, ""), http.StatusNotFound, "")
}

func photosOf(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	items, _ := decode(t, w)["items"].([]any)
	out := make([]map[string]any, len(items))
	for i, it := range items {
		out[i], _ = it.(map[string]any)
	}
	return out
}

func TestMyAvatarHistory(t *testing.T) {
	e := newEnv(t)
	me, token := e.login(t, true)
	urls := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		w := e.upload(http.MethodPost, "/api/users/me/avatar", token, photo(t))
		expect(t, w, http.StatusOK, "")
		url, _ := decode(t, w)["avatar_url"].(string)
		urls = append(urls, url)
	}

	// в истории пять последних, текущее первое; шестое вытеснило первое вместе с обоими файлами
	list := photosOf(t, e.do(http.MethodGet, "/api/users/me/avatars", token, ""))
	if len(list) != 5 || list[0]["avatar_url"] != urls[5] || list[0]["is_current"] != true || list[1]["is_current"] != false {
		t.Fatalf("history: %v", list)
	}
	assertStored(t, e.media, urls[1], urls[2], urls[3], urls[4], urls[5])
	if r := e.do(http.MethodGet, urls[0], "", ""); r.Code != http.StatusNotFound {
		t.Fatalf("evicted photo is still served: %d", r.Code)
	}
	// чужую историю видно по id (viewer берётся из токена), свою - и по /me
	if other := photosOf(t, e.do(http.MethodGet, "/api/users/"+me+"/avatars", token, "")); len(other) != 5 {
		t.Fatalf("by id: %v", other)
	}

	// сделать главной: старое фото становится текущим, ничего не удаляется
	oldest := list[4]["id"].(string)
	expect(t, e.do(http.MethodPost, "/api/users/me/avatars/"+oldest+"/select", token, ""), http.StatusOK, "")
	list = photosOf(t, e.do(http.MethodGet, "/api/users/me/avatars", token, ""))
	if list[0]["id"] != oldest || list[0]["avatar_url"] != urls[1] || len(e.media.keys()) != 10 {
		t.Fatalf("after select: %v keys=%d", list, len(e.media.keys()))
	}
	expect(t, e.do(http.MethodPost, "/api/users/me/avatars/"+uuid.NewString()+"/select", token, ""), http.StatusNotFound, "")

	// удалить именно это фото (не текущее): текущее остаётся, файлы удалены
	victim := list[2]
	expect(t, e.do(http.MethodDelete, "/api/users/me/avatars/"+victim["id"].(string), token, ""), http.StatusOK, "")
	after := photosOf(t, e.do(http.MethodGet, "/api/users/me/avatars", token, ""))
	if len(after) != 4 || after[0]["id"] != oldest {
		t.Fatalf("after delete: %v", after)
	}
	if r := e.do(http.MethodGet, victim["avatar_url"].(string), "", ""); r.Code != http.StatusNotFound {
		t.Fatalf("deleted photo is still served: %d", r.Code)
	}
	if len(e.media.keys()) != 8 {
		t.Fatalf("keys after delete: %v", e.media.keys())
	}
}

func TestGroupAvatarHistoryEndpoints(t *testing.T) {
	e := newEnv(t)
	admin, adminToken := e.login(t, true)
	_, memberToken := e.login(t, true)
	group := uuid.NewString()
	e.convs.admins[group] = admin
	base := "/api/conversations/" + group

	var first string
	for i := 0; i < 2; i++ {
		w := e.upload(http.MethodPost, base+"/avatar", adminToken, photo(t))
		expect(t, w, http.StatusOK, "")
		if i == 0 {
			first, _ = decode(t, w)["avatar_url"].(string)
		}
	}
	assertStored(t, e.media, first, e.convs.avatars[group])

	list := photosOf(t, e.do(http.MethodGet, base+"/avatars", memberToken, ""))
	if len(list) != 2 || list[0]["is_current"] != true {
		t.Fatalf("history: %v", list)
	}
	// менять историю - только админ
	older := list[1]["id"].(string)
	expect(t, e.do(http.MethodPost, base+"/avatars/"+older+"/select", memberToken, ""), http.StatusForbidden, "")
	expect(t, e.do(http.MethodDelete, base+"/avatars/"+older, memberToken, ""), http.StatusForbidden, "")
	expect(t, e.do(http.MethodPost, base+"/avatars/"+older+"/select", adminToken, ""), http.StatusOK, "")
	if e.convs.avatars[group] != first {
		t.Fatalf("group avatar after select: %q", e.convs.avatars[group])
	}
	// удаление фото из истории удаляет и файлы
	expect(t, e.do(http.MethodDelete, base+"/avatars/"+older, adminToken, ""), http.StatusOK, "")
	if len(e.media.keys()) != 2 || e.convs.avatars[group] == first {
		t.Fatalf("after delete: keys=%v avatar=%q", e.media.keys(), e.convs.avatars[group])
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
		{"tiny image", []part{{"file", "a.png", "image/png", pngBytes(t, 8, 8)}}, http.StatusBadRequest, "AVATAR_TOO_SMALL"},
		{"almost big enough", []part{{"file", "a.png", "image/png", pngBytes(t, 511, 900)}}, http.StatusBadRequest, "AVATAR_TOO_SMALL"},
		{"no file field", []part{{"avatar", "a.png", "image/png", pngBytes(t, 600, 600)}}, http.StatusBadRequest, "INVALID_REQUEST"},
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
		"/media/avatars/" + strings.Repeat("a", 32) + ".jpg",   // формат верный, но файла нет
		"/media/avatars/" + strings.Repeat("a", 32) + "_l.jpg", // крупная версия тоже по формату, но файла нет
		"/media/avatars/" + strings.Repeat("a", 32) + "_l.png",
		"/media/avatars/" + strings.Repeat("a", 32) + "_x.jpg",
		"/media/avatars/" + strings.Repeat("a", 32) + "_l_l.jpg",
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
