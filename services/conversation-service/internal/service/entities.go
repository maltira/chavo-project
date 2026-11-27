package service

import (
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

const (
	// maxEntities - сколько фрагментов форматирования допускается в одном сообщении
	maxEntities            = 100
	maxEntityURLLength     = 2048
	maxEntityLanguageLen   = 32
	maxPreviewTitleRunes   = 300
	maxPreviewDescRunes    = 500
	maxPreviewSiteRunes    = 120
)

func validateHTTPURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > maxEntityURLLength {
		return "", apperror.ErrIncorrectData
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", apperror.ErrIncorrectData
		}
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", apperror.ErrIncorrectData
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "http" && scheme != "https") || u.Host == "" {
		return "", apperror.ErrIncorrectData
	}
	return trimmed, nil
}

func validateLanguage(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > maxEntityLanguageLen {
		return "", apperror.ErrIncorrectData
	}
	for _, r := range trimmed {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '_' || r == '+' || r == '#' || r == '.' || r == '-') {
			return "", apperror.ErrIncorrectData
		}
	}
	return trimmed, nil
}

// normalizeEntities проверяет форматирование текста и приводит его к каноническому виду: фрагменты одного
// вида (и с тем же url/language), которые пересекаются или касаются, склеиваются, список отсортирован.
// Диапазоны задаются в единицах UTF-16 (как индексы строки в браузере) и не должны выходить за текст или
// разрезать суррогатную пару. Пустой результат - nil: сообщение без форматирования
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

	type mergeKey struct {
		typ string
		arg string
	}
	mergeable := map[mergeKey][]models.Entity{}
	var distinct []models.Entity

	for _, e := range in {
		end := e.Offset + e.Length
		if e.Offset < 0 || e.Length <= 0 || end > len(units) || end < e.Offset || splits(e.Offset) || splits(end) {
			return nil, apperror.ErrIncorrectData
		}

		clean := models.Entity{
			Type:   e.Type,
			Offset: e.Offset,
			Length: e.Length,
		}

		switch e.Type {
		case models.EntityBold, models.EntityItalic, models.EntityUnderline,
			models.EntityStrikethrough, models.EntityCode, models.EntitySpoiler, models.EntityBlockquote:
			if e.URL != nil || e.UserID != nil || e.Language != nil {
				return nil, apperror.ErrIncorrectData
			}
			k := mergeKey{typ: e.Type}
			mergeable[k] = append(mergeable[k], clean)

		case models.EntityPre:
			if e.URL != nil || e.UserID != nil {
				return nil, apperror.ErrIncorrectData
			}
			var lang string
			if e.Language != nil {
				var err error
				lang, err = validateLanguage(*e.Language)
				if err != nil {
					return nil, err
				}
				if lang != "" {
					l := lang
					clean.Language = &l
				}
			}
			k := mergeKey{typ: e.Type, arg: lang}
			mergeable[k] = append(mergeable[k], clean)

		case models.EntityTextLink:
			if e.URL == nil || e.UserID != nil || e.Language != nil {
				return nil, apperror.ErrIncorrectData
			}
			validURL, err := validateHTTPURL(*e.URL)
			if err != nil {
				return nil, err
			}
			clean.URL = &validURL
			k := mergeKey{typ: e.Type, arg: validURL}
			mergeable[k] = append(mergeable[k], clean)

		case models.EntityURL:
			if e.URL != nil || e.UserID != nil || e.Language != nil {
				return nil, apperror.ErrIncorrectData
			}
			sliceText := string(utf16.Decode(units[e.Offset:end]))
			if _, err := validateHTTPURL(sliceText); err != nil {
				return nil, apperror.ErrIncorrectData
			}
			distinct = append(distinct, clean)

		case models.EntityMention:
			if e.UserID == nil || e.URL != nil || e.Language != nil {
				return nil, apperror.ErrIncorrectData
			}
			uid, err := uuid.Parse(strings.TrimSpace(*e.UserID))
			if err != nil || uid == uuid.Nil {
				return nil, apperror.ErrIncorrectData
			}
			uidStr := uid.String()
			clean.UserID = &uidStr
			distinct = append(distinct, clean)

		default:
			return nil, apperror.ErrIncorrectData
		}
	}

	var out []models.Entity
	for _, list := range mergeable {
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

	// url и mention не склеиваем, но отбрасываем точные дубли и запрещаем пересечения внутри одного вида
	if len(distinct) > 0 {
		byDistinctType := map[string][]models.Entity{}
		for _, e := range distinct {
			byDistinctType[e.Type] = append(byDistinctType[e.Type], e)
		}
		for _, list := range byDistinctType {
			sort.Slice(list, func(i, j int) bool {
				if list[i].Offset != list[j].Offset {
					return list[i].Offset < list[j].Offset
				}
				return list[i].Length > list[j].Length
			})
			var last *models.Entity
			for i := range list {
				cur := list[i]
				if last != nil {
					if cur.Offset == last.Offset && cur.Length == last.Length && ptrEq(cur.UserID, last.UserID) {
						continue
					}
					if cur.Offset < last.Offset+last.Length {
						return nil, apperror.ErrIncorrectData
					}
				}
				out = append(out, cur)
				last = &out[len(out)-1]
			}
		}
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

func ptrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// extractMentionUserIDs возвращает уникальные UUID упомянутых пользователей в порядке появления
func extractMentionUserIDs(entities []models.Entity) []uuid.UUID {
	var ids []uuid.UUID
	seen := make(map[uuid.UUID]struct{})
	for _, e := range entities {
		if e.Type != models.EntityMention || e.UserID == nil {
			continue
		}
		uid, err := uuid.Parse(*e.UserID)
		if err != nil || uid == uuid.Nil {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		ids = append(ids, uid)
	}
	return ids
}

func trimAndClip(s *string, maxRunes int) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > maxRunes {
		runes := []rune(trimmed)
		trimmed = strings.TrimSpace(string(runes[:maxRunes]))
	}
	return &trimmed
}

// validateLinkPreview проверяет URL-ы превью и обрезает текстовые поля; пустое превью превращается в nil
func validateLinkPreview(lp *models.LinkPreview) (*models.LinkPreview, error) {
	if lp == nil {
		return nil, nil
	}
	validURL, err := validateHTTPURL(lp.URL)
	if err != nil {
		return nil, err
	}
	var imgURL *string
	if lp.ImageURL != nil && strings.TrimSpace(*lp.ImageURL) != "" {
		validImg, err := validateHTTPURL(*lp.ImageURL)
		if err != nil {
			return nil, err
		}
		imgURL = &validImg
	}
	out := &models.LinkPreview{
		URL:         validURL,
		Title:       trimAndClip(lp.Title, maxPreviewTitleRunes),
		Description: trimAndClip(lp.Description, maxPreviewDescRunes),
		ImageURL:    imgURL,
		SiteName:    trimAndClip(lp.SiteName, maxPreviewSiteRunes),
	}
	if out.Title == nil && out.Description == nil && out.ImageURL == nil {
		return nil, nil
	}
	return out, nil
}
