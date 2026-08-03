package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

type SettingsService interface {
	GetSettings(ctx context.Context, profileID uuid.UUID) (*models.Settings, error)
	UpdateVisibleStatus(ctx context.Context, userID uuid.UUID, isVisible bool) error
	UpdateVisibleBirthDate(ctx context.Context, userID uuid.UUID, isVisible bool) error
}

type settingsService struct {
	repo repository.SettingsRepository
}

func NewSettingsService(repo repository.SettingsRepository) SettingsService {
	return &settingsService{repo: repo}
}

func (s *settingsService) GetSettings(ctx context.Context, profileID uuid.UUID) (*models.Settings, error) {
	return s.repo.GetSettings(ctx, profileID)
}

func (s *settingsService) UpdateVisibleStatus(ctx context.Context, userID uuid.UUID, isVisible bool) error {
	return s.repo.UpdateVisibleStatus(ctx, userID, isVisible)
}

func (s *settingsService) UpdateVisibleBirthDate(ctx context.Context, userID uuid.UUID, isVisible bool) error {
	return s.repo.UpdateVisibleBirthDate(ctx, userID, isVisible)
}
