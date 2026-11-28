// Package profilecover - шапка / обложка профиля пользователя или группы: единственный источник истины
// о допустимых паттернах. Ими пользуются user-service, conversation-service (проверка) и Gateway (каталог)
//
// обложка - обязательный цвет #RRGGBB (из него строится градиент) и необязательный ключ паттерна
// (пустая строка означает только градиент, без символов)
package profilecover

import (
	"errors"

	"github.com/maltira/chavo-project-backend/proto/color"
)

var (
	ErrUnknownPattern = errors.New("unknown profile cover pattern")
	ErrInvalidColor   = errors.New("invalid profile cover color")
)

var patterns = []string{
	"star", "heart", "shine", "moon", "flame", "clover",
	"crown", "bolt", "flower", "music", "paw", "diamond",
}

// Patterns - все символы паттерна в порядке показа (копия: каталог снаружи не меняется)
func Patterns() []string { return append([]string(nil), patterns...) }

// Known - true для ключа паттерна из набора; для пустой строки или убранного ключа - false
func Known(key string) bool {
	for _, p := range patterns {
		if p == key {
			return true
		}
	}
	return false
}

// Valid - проверенная обложка: цвет приведён к #RRGGBB в верхнем регистре, паттерн - либо пустой, либо из каталога
type Valid struct {
	Color, Pattern string
}

// Validate проверяет новую обложку: цвет #RRGGBB обязателен, паттерн опционален
func Validate(c, pattern string) (Valid, error) {
	norm, ok := color.Normalize(c)
	if !ok {
		return Valid{}, ErrInvalidColor
	}
	if pattern != "" && !Known(pattern) {
		return Valid{}, ErrUnknownPattern
	}
	return Valid{Color: norm, Pattern: pattern}, nil
}
