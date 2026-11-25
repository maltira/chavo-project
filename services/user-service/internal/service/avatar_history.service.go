package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/media"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

// AvatarHistoryService - история аватарок: до media.MaxAvatarHistory последних фото, текущее - самое новое.
// Файлы хранит и удаляет Gateway: сервис лишь возвращает адреса выпавших и удалённых фото. Событий нет:
// другие клиенты увидят новое фото при следующей загрузке профиля
type AvatarHistoryService interface {
	// List - история владельца для viewer; если владелец заблокировал viewer, она пуста (как и сама аватарка)
	List(ctx context.Context, viewerID, ownerID uuid.UUID) ([]models.AvatarPhoto, error)
	// Add делает загруженное Gateway'ем фото текущим; возвращает адреса, выпавшие из истории
	Add(ctx context.Context, userID uuid.UUID, url string) ([]string, error)
	Select(ctx context.Context, userID, photoID uuid.UUID) error
	// Delete убирает фото (nil - текущее); возвращает его адрес
	Delete(ctx context.Context, userID uuid.UUID, photoID *uuid.UUID) (string, error)
}

type avatarHistoryService struct {
	repo   repository.AvatarHistoryRepository
	blocks repository.BlockRepository
}

func NewAvatarHistoryService(repo repository.AvatarHistoryRepository, blocks repository.BlockRepository) AvatarHistoryService {
	return &avatarHistoryService{repo: repo, blocks: blocks}
}

func (s *avatarHistoryService) List(ctx context.Context, viewerID, ownerID uuid.UUID) ([]models.AvatarPhoto, error) {
	if viewerID != ownerID {
		blockedBy, err := s.blocks.BlockedBy(ctx, viewerID, []uuid.UUID{ownerID})
		if err != nil {
			return nil, err
		}
		if blockedBy[ownerID] {
			return []models.AvatarPhoto{}, nil
		}
	}
	return s.repo.List(ctx, ownerID)
}

func (s *avatarHistoryService) Add(ctx context.Context, userID uuid.UUID, url string) ([]string, error) {
	norm, err := media.NormalizeAvatarURL(url)
	if err != nil {
		return nil, err
	}
	// пустой адрес - не фото: убирать аватарку нужно через Delete
	if norm == nil {
		return nil, apperror.ErrIncorrectData
	}
	return s.repo.Add(ctx, userID, *norm, media.MaxAvatarHistory)
}

func (s *avatarHistoryService) Select(ctx context.Context, userID, photoID uuid.UUID) error {
	return s.repo.Select(ctx, userID, photoID)
}

func (s *avatarHistoryService) Delete(ctx context.Context, userID uuid.UUID, photoID *uuid.UUID) (string, error) {
	return s.repo.Delete(ctx, userID, photoID)
}
