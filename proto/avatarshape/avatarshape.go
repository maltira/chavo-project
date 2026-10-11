// Package avatarshape - фигуры позади аватара пользователя или группы: единственный источник истины
// о допустимых фигурах. Ими пользуются user-service, conversation-service (проверка) и Gateway (каталог)
//
// фигура - ключ из фиксированного набора и цвет, который выбирает пользователь; срока нет
package avatarshape

import (
	"errors"

	"github.com/maltira/chavo-project-backend/proto/color"
)

var (
	ErrUnknownShape = errors.New("unknown avatar shape")
	ErrInvalidColor = errors.New("invalid avatar shape color")
)

var shapes = []string{
	"circle", "square", "slanted", "arch", "oval", "pill", "pentagon", "gem",
	"very-sunny", "sunny",
	"4-sided-cookie", "6-sided-cookie", "7-sided-cookie", "9-sided-cookie", "12-sided-cookie",
	"ghost", "4-leaf-clover", "8-leaf-clover",
	"burst", "soft-burst", "boom", "soft-boom",
	"flower", "puffy", "puffy-diamond", "pixel-circle", "bun",
}

// Shapes - все фигуры в порядке показа (копия: каталог снаружи не меняется)
func Shapes() []string { return append([]string(nil), shapes...) }

// Known - false для фигуры, которой нет в наборе (например, убранной после установки):
// такая фигура при чтении считается отсутствующей
func Known(key string) bool {
	for _, s := range shapes {
		if s == key {
			return true
		}
	}
	return false
}

// Valid - проверенная фигура: цвет приведён к #RRGGBB в верхнем регистре
type Valid struct {
	Shape, Color string
}

// Validate проверяет новую фигуру: ключ из набора и цвет #RRGGBB (обязателен)
func Validate(shape, c string) (Valid, error) {
	if !Known(shape) {
		return Valid{}, ErrUnknownShape
	}
	norm, ok := color.Normalize(c)
	if !ok {
		return Valid{}, ErrInvalidColor
	}
	return Valid{Shape: shape, Color: norm}, nil
}
