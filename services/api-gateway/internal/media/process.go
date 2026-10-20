package media

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"

	// декодеры: только чистый Go, без нативных библиотек и внешних программ
	_ "image/gif"
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// AvatarSize - сторона аватарки после перекодирования, px
	AvatarSize = 256
	// MaxUploadBytes - потолок загружаемого файла (весь запрос ограничен 1 МиБ ещё и в Gateway и nginx)
	MaxUploadBytes = 1 << 20

	minSide     = 64
	maxSide     = 8000
	maxPixels   = 24_000_000
	jpegQuality = 85
)

var (
	// ErrUnsupportedType - по содержимому это не JPEG/PNG/GIF/WebP (исполняемый файл, скрипт, HTML, SVG...)
	ErrUnsupportedType = errors.New("unsupported image type")
	// ErrInvalidImage - похоже на картинку, но не декодируется, слишком маленькая или огромная
	ErrInvalidImage = errors.New("invalid image")
)

// форматы, которые принимаются: тип по сигнатуре (DetectContentType) -> имя декодера (DecodeConfig)
var allowedFormats = map[string]string{
	"image/jpeg": "jpeg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// ProcessAvatar проверяет загруженный файл и возвращает новую аватарку: квадрат AvatarSize×AvatarSize в JPEG.
//
// Исходные байты дальше не идут: в хранилище попадает только то, что закодировал этот код из
// декодированных пикселей. Поэтому дописанный в конец картинки исполняемый файл или архив («полиглот»),
// EXIF/XMP/ICC и любые вложенные данные не сохраняются. Имя файла и Content-Type от клиента не используются
func ProcessAvatar(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidImage
	}
	if len(data) > MaxUploadBytes {
		return nil, ErrInvalidImage
	}

	// тип - только по содержимому; SVG (там может быть скрипт) и всё нерастровое сюда не проходит
	wantFormat, ok := allowedFormats[http.DetectContentType(data)]
	if !ok {
		return nil, ErrUnsupportedType
	}

	// размеры - до декодирования: заголовок с гигантскими сторонами («бомба») отсекается без выделения памяти
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != wantFormat {
		return nil, ErrInvalidImage
	}
	if cfg.Width < minSide || cfg.Height < minSide || cfg.Width > maxSide || cfg.Height > maxSide ||
		cfg.Width*cfg.Height > maxPixels {
		return nil, ErrInvalidImage
	}

	// GIF: image.Decode берёт только первый кадр
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != wantFormat {
		return nil, ErrInvalidImage
	}

	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	src := image.Rect(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2, 0, 0)
	src.Max = src.Min.Add(image.Pt(side, side))

	// прозрачность (PNG, GIF, WebP) - на белый: в JPEG альфа-канала нет
	dst := image.NewRGBA(image.Rect(0, 0, AvatarSize, AvatarSize))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, src, draw.Over, nil)

	var out bytes.Buffer
	if err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
