package service

import (
	"context"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

type SettingsService interface {
	GetSettings(ctx context.Context, userID uuid.UUID) (*models.Settings, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, updates map[string]any) error
	ShowOnlineStatus(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error)
	ForwardAuthorsHidden(ctx context.Context, userIDs []uuid.UUID) ([]uuid.UUID, error)
}

type settingsService struct {
	repo repository.SettingsRepository
}

func NewSettingsService(repo repository.SettingsRepository) SettingsService {
	return &settingsService{repo: repo}
}

func (s *settingsService) GetSettings(ctx context.Context, userID uuid.UUID) (*models.Settings, error) {
	return s.repo.GetSettings(ctx, userID)
}

func (s *settingsService) UpdateSettings(ctx context.Context, userID uuid.UUID, data map[string]any) error {
	updates := make(map[string]any)

	if v, ok := data["allow_group_invites"]; ok {
		updates["allow_group_invites"] = v
	}

	if v, ok := data["show_online_status"]; ok {
		updates["show_online_status"] = v
	}

	if v, ok := data["hide_forward_author"]; ok {
		updates["hide_forward_author"] = v
	}

	if v, ok := data["favorite_emojis"]; ok {
		list, err := normalizeFavoriteEmojis(v)
		if err != nil {
			return err
		}
		updates["favorite_emojis"] = list
	}

	if len(updates) == 0 {
		return apperror.ErrNoColumnsToUpdate
	}

	return s.repo.UpdateSettings(ctx, userID, updates)
}

func (s *settingsService) ShowOnlineStatus(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	return s.repo.ShowOnlineStatus(ctx, userIDs)
}

func (s *settingsService) ForwardAuthorsHidden(ctx context.Context, userIDs []uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.ForwardAuthorsHidden(ctx, userIDs)
}

// MaxFavoriteEmojis - сколько эмодзи можно держать в избранном
const MaxFavoriteEmojis = 30

// maxEmojiBytes - самая длинная последовательность (флаг, семья, ZWJ-цепочка) укладывается в 32 байта и дальше
const maxEmojiBytes = 32

// normalizeFavoriteEmojis проверяет список избранных: до MaxFavoriteEmojis, без повторов и пустых, каждая
// запись - короткая строка без пробелов и управляющих символов. Какие именно это эмодзи, сервер не решает:
// каталог живёт на клиенте, а чужой или лишний символ никому не вредит
func normalizeFavoriteEmojis(v any) ([]string, error) {
	list, ok := v.([]string)
	if !ok {
		return nil, apperror.ErrIncorrectData
	}
	res := make([]string, 0, len(list))
	seen := make(map[string]bool, len(list))
	for _, e := range list {
		if e == "" || len(e) > maxEmojiBytes || strings.ContainsFunc(e, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r)
		}) {
			return nil, apperror.ErrIncorrectData
		}
		if !seen[e] {
			seen[e] = true
			res = append(res, e)
		}
	}
	if len(res) > MaxFavoriteEmojis {
		return nil, apperror.ErrIncorrectData
	}
	return res, nil
}
