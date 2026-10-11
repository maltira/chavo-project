package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/avatarshape"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

// AvatarShapeService - фигура позади аватара: только установить новую (целиком) или удалить; только REST, без событий
type AvatarShapeService interface {
	Set(ctx context.Context, userID uuid.UUID, shape, color string) (*models.AvatarShape, error)
	Clear(ctx context.Context, userID uuid.UUID) error
}

type avatarShapeService struct {
	repo repository.AvatarShapeRepository
}

func NewAvatarShapeService(repo repository.AvatarShapeRepository) AvatarShapeService {
	return &avatarShapeService{repo: repo}
}

func (s *avatarShapeService) Set(ctx context.Context, userID uuid.UUID, shape, color string) (*models.AvatarShape, error) {
	v, err := avatarshape.Validate(shape, color)
	if err != nil {
		return nil, err
	}
	sh := models.AvatarShape{Shape: v.Shape, Color: v.Color}
	if err := s.repo.Set(ctx, userID, sh); err != nil {
		return nil, err
	}
	return &sh, nil
}

func (s *avatarShapeService) Clear(ctx context.Context, userID uuid.UUID) error {
	return s.repo.Clear(ctx, userID)
}
