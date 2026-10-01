package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

var validThemes = map[string]bool{"system": true, "light": true, "dark": true}

type SettingsService interface {
	GetSettings(ctx context.Context, userID uuid.UUID) (*models.Settings, error)
	UpdateSettings(ctx context.Context, userID uuid.UUID, updates map[string]any) error
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

	if v, ok := data["system_language"]; ok {
		str, isStr := v.(string)
		if !isStr || len(str) == 0 || len(str) > 10 {
			return apperror.ErrIncorrectData
		}
		updates["system_language"] = str
	}

	if v, ok := data["theme"]; ok {
		str, isStr := v.(string)
		if !isStr || !validThemes[str] {
			return apperror.ErrIncorrectData
		}
		updates["theme"] = str
	}

	if v, ok := data["allow_group_invites"]; ok {
		updates["allow_group_invites"] = v
	}

	if v, ok := data["show_online_status"]; ok {
		updates["show_online_status"] = v
	}

	if len(updates) == 0 {
		return apperror.ErrNoColumnsToUpdate
	}

	return s.repo.UpdateSettings(ctx, userID, updates)
}
