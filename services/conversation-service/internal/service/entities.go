package service

import (
	"sort"
	"unicode/utf16"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

// maxEntities - сколько фрагментов форматирования допускается в одном сообщении
const maxEntities = 100

// normalizeEntities проверяет форматирование текста и приводит его к каноническому виду: фрагменты одного
// вида, которые пересекаются или касаются, склеиваются, список отсортирован. Диапазоны задаются в единицах
// UTF-16 (как индексы строки в браузере) и не должны выходить за текст или разрезать суррогатную пару.
// Пустой результат - nil: сообщение без форматирования
func normalizeEntities(content string, in []models.Entity) ([]models.Entity, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > maxEntities {
		return nil, apperror.ErrIncorrectData
	}
	units := utf16.Encode([]rune(content))
	// граница не должна приходиться на вторую половину суррогатной пары: символ разрезать нельзя
	splits := func(at int) bool {
		return at > 0 && at < len(units) && units[at] >= 0xDC00 && units[at] <= 0xDFFF
	}

	byType := map[string][]models.Entity{}
	for _, e := range in {
		switch e.Type {
		case models.EntityBold, models.EntityItalic, models.EntityUnderline:
		default:
			return nil, apperror.ErrIncorrectData
		}
		end := e.Offset + e.Length
		if e.Offset < 0 || e.Length <= 0 || end > len(units) || end < e.Offset || splits(e.Offset) || splits(end) {
			return nil, apperror.ErrIncorrectData
		}
		byType[e.Type] = append(byType[e.Type], e)
	}

	var out []models.Entity
	for _, list := range byType {
		sort.Slice(list, func(i, j int) bool { return list[i].Offset < list[j].Offset })
		cur := list[0]
		for _, e := range list[1:] {
			if e.Offset <= cur.Offset+cur.Length {
				cur.Length = max(cur.Offset+cur.Length, e.Offset+e.Length) - cur.Offset
				continue
			}
			out = append(out, cur)
			cur = e
		}
		out = append(out, cur)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Offset != b.Offset {
			return a.Offset < b.Offset
		}
		if a.Length != b.Length {
			return a.Length > b.Length
		}
		return a.Type < b.Type
	})
	return out, nil
}
