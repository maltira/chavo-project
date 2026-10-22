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
	// MinAvatarSide - наименьшая сторона загружаемого фото: меньшее не принимается, чтобы крупная версия
	// для просмотра на весь экран не была размытой
	MinAvatarSide = 512
	// LargeAvatarSize - предел стороны крупной версии для просмотра на весь экран; фото меньше не растягивается
	LargeAvatarSize = 1024
	// MaxUploadBytes - потолок загружаемого файла (весь запрос ограничен 1 МиБ ещё и в Gateway и nginx)
	MaxUploadBytes = 1 << 20

	maxSide     = 8000
	maxPixels   = 24_000_000
	jpegQuality = 85
)

var (
	// ErrUnsupportedType - по содержимому это не JPEG/PNG/GIF/WebP (исполняемый файл, скрипт, HTML, SVG...)
	ErrUnsupportedType = errors.New("unsupported image type")
	// ErrInvalidImage - похоже на картинку, но не декодируется или слишком большая
	ErrInvalidImage = errors.New("invalid image")
	// ErrImageTooSmall - сторона фото меньше MinAvatarSide
	ErrImageTooSmall = errors.New("image is too small")
)

// форматы, которые принимаются: тип по сигнатуре (DetectContentType) -> имя декодера (DecodeConfig)
var allowedFormats = map[string]string{
	"image/jpeg": "jpeg",
	"image/png":  "png",
	"image/gif":  "gif",
	"image/webp": "webp",
}

// Avatars - обе версии новой аватарки: маленькая (лежит в базе и в списках) и крупная (просмотр)
type Avatars struct {
	Small []byte
	Large []byte
}

// ProcessAvatar проверяет загруженный файл и возвращает обе версии аватарки: квадрат AvatarSize×AvatarSize
// и крупный квадрат (не больше LargeAvatarSize, без растягивания) в JPEG.
//
// Исходные байты дальше не идут: в хранилище попадает только то, что закодировал этот код из
// декодированных пикселей. Поэтому дописанный в конец картинки исполняемый файл или архив («полиглот»),
// EXIF/XMP/ICC и любые вложенные данные не сохраняются. Имя файла и Content-Type от клиента не используются
func ProcessAvatar(data []byte) (*Avatars, error) {
	img, src, err := decodeSquare(data)
	if err != nil {
		return nil, err
	}
	small, err := encodeSquare(img, src, AvatarSize)
	if err != nil {
		return nil, err
	}
	large, err := encodeSquare(img, src, min(src.Dx(), LargeAvatarSize))
	if err != nil {
		return nil, err
	}
	return &Avatars{Small: small, Large: large}, nil
}

// decodeSquare проверяет файл, декодирует его и возвращает центральный квадрат
func decodeSquare(data []byte) (image.Image, image.Rectangle, error) {
	if len(data) == 0 || len(data) > MaxUploadBytes {
		return nil, image.Rectangle{}, ErrInvalidImage
	}

	// тип - только по содержимому; SVG (там может быть скрипт) и всё нерастровое сюда не проходит
	wantFormat, ok := allowedFormats[http.DetectContentType(data)]
	if !ok {
		return nil, image.Rectangle{}, ErrUnsupportedType
	}

	// размеры - до декодирования: заголовок с гигантскими сторонами («бомба») отсекается без выделения памяти
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != wantFormat {
		return nil, image.Rectangle{}, ErrInvalidImage
	}
	if cfg.Width > maxSide || cfg.Height > maxSide || cfg.Width*cfg.Height > maxPixels {
		return nil, image.Rectangle{}, ErrInvalidImage
	}
	if cfg.Width < MinAvatarSide || cfg.Height < MinAvatarSide {
		return nil, image.Rectangle{}, ErrImageTooSmall
	}

	// GIF: image.Decode берёт только первый кадр
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != wantFormat {
		return nil, image.Rectangle{}, ErrInvalidImage
	}

	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	src := image.Rect(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2, 0, 0)
	src.Max = src.Min.Add(image.Pt(side, side))
	return img, src, nil
}

// encodeSquare масштабирует квадрат src до size×size и кодирует в JPEG; прозрачность - на белый:
// в JPEG альфа-канала нет
func encodeSquare(img image.Image, src image.Rectangle, size int) ([]byte, error) {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, src, draw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
