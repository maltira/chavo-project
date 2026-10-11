// Package color - проверка цветов, которые выбирает пользователь (статусы, фигуры за аватаром)
package color

import (
	"regexp"
	"strings"
)

var hexRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Normalize проверяет формат #RRGGBB и приводит цвет к верхнему регистру; false - формат неверный
func Normalize(c string) (string, bool) {
	if !hexRe.MatchString(c) {
		return "", false
	}
	return strings.ToUpper(c), true
}
