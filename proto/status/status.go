// Package status — каталог статусов пользователей и групп: единственный источник истины о допустимых иконках
// и сроках действия и о правилах проверки статуса. Им пользуются user-service, conversation-service (проверка)
// и Gateway (GET /api/statuses). В proto и БД иконка и срок хранятся строками-ключами отсюда.
//
// Статус — иконка из фиксированного набора, текст, цвет фона и общий цвет текста и иконки (оба выбирает
// пользователь) и необязательный срок.
package status

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/maltira/chavo-project-backend/proto/color"
)

// MaxTextLength — лимит текста статуса в символах (рунах).
const MaxTextLength = 70

var (
	ErrUnknownIcon     = errors.New("unknown status icon")
	ErrUnknownDuration = errors.New("unknown status duration")
	ErrInvalidText     = errors.New("invalid status text")
	ErrInvalidColor    = errors.New("invalid status color")
)

// icons — ключи иконок; клиент сопоставляет ключ с картинкой (shape-<ключ>.svg)
var icons = []string{"moon", "shine", "ok", "flame", "block", "heart"}

// Duration — срок действия; Value 0 — без срока
type Duration struct {
	Key   string
	Value time.Duration
}

var durations = []Duration{
	{Key: "1h", Value: time.Hour},
	{Key: "4h", Value: 4 * time.Hour},
	{Key: "24h", Value: 24 * time.Hour},
	{Key: "7d", Value: 7 * 24 * time.Hour},
	{Key: "never", Value: 0},
}

// Icons — все иконки в порядке показа
func Icons() []string { return append([]string(nil), icons...) }

// Durations — все сроки в порядке показа
func Durations() []Duration { return append([]Duration(nil), durations...) }

// KnownIcon — false для иконки, которой нет в наборе (например, убранной после установки статуса):
// такой статус при чтении считается отсутствующим
func KnownIcon(key string) bool {
	for _, i := range icons {
		if i == key {
			return true
		}
	}
	return false
}

// Input — статус в том виде, в каком его прислал клиент.
type Input struct {
	Icon, Text, Background, Foreground, Duration string
}

// Valid — проверенный статус: текст очищен, цвета приведены к #RRGGBB в верхнем регистре, ExpiresAt nil — без срока.
type Valid struct {
	Icon, Text, Background, Foreground string
	ExpiresAt                          *time.Time
}

// Validate проверяет новый статус. Текст: пробелы по краям обрезаются, 1–MaxTextLength символов,
// одной строкой, без управляющих символов. Цвета обязательны, формат #RRGGBB.
func Validate(in Input, now time.Time) (Valid, error) {
	if !KnownIcon(in.Icon) {
		return Valid{}, ErrUnknownIcon
	}
	var d *Duration
	for i := range durations {
		if durations[i].Key == in.Duration {
			d = &durations[i]
			break
		}
	}
	if d == nil {
		return Valid{}, ErrUnknownDuration
	}
	bg, okBg := color.Normalize(in.Background)
	fg, okFg := color.Normalize(in.Foreground)
	if !okBg || !okFg {
		return Valid{}, ErrInvalidColor
	}

	text := strings.TrimSpace(in.Text)
	if !utf8.ValidString(text) {
		return Valid{}, ErrInvalidText
	}
	if n := utf8.RuneCountInString(text); n == 0 || n > MaxTextLength {
		return Valid{}, ErrInvalidText
	}
	for _, r := range text {
		if unicode.IsControl(r) {
			return Valid{}, ErrInvalidText
		}
	}

	v := Valid{Icon: in.Icon, Text: text, Background: bg, Foreground: fg}
	if d.Value != 0 {
		exp := now.Add(d.Value).UTC()
		v.ExpiresAt = &exp
	}
	return v, nil
}
