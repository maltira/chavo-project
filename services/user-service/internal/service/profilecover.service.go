package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/profilecover"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

// ProfileCoverService - обложка профиля: только установить новую (целиком) или удалить; только REST, без событий
type ProfileCoverService interface {
	Set(ctx context.Context, userID uuid.UUID, color, pattern string) (*models.ProfileCover, error)
	Clear(ctx context.Context, userID uuid.UUID) error
}

type profileCoverService struct {
	repo repository.ProfileCoverRepository
}

func NewProfileCoverService(repo repository.ProfileCoverRepository) ProfileCoverService {
	return &profileCoverService{repo: repo}
}

func (s *profileCoverService) Set(ctx context.Context, userID uuid.UUID, color, pattern string) (*models.ProfileCover, error) {
	v, err := profilecover.Validate(color, pattern)
	if err != nil {
		return nil, err
	}
	cover := models.ProfileCover{Color: v.Color, Pattern: v.Pattern}
	if err := s.repo.Set(ctx, userID, cover); err != nil {
		return nil, err
	}
	return &cover, nil
}

func (s *profileCoverService) Clear(ctx context.Context, userID uuid.UUID) error {
	return s.repo.Clear(ctx, userID)
}
