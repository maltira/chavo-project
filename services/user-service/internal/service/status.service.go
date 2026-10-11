package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/status"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

// StatusService - статус пользователя: только установить новый (целиком, прежний заменяется) или удалить
// в реальном времени не рассылается: другие увидят изменение при следующей загрузке профиля
type StatusService interface {
	Set(ctx context.Context, userID uuid.UUID, in status.Input) (*models.Status, error)
	Clear(ctx context.Context, userID uuid.UUID) error
}

type statusService struct {
	repo repository.StatusRepository
	now  func() time.Time
}

func NewStatusService(repo repository.StatusRepository) StatusService {
	return &statusService{repo: repo, now: time.Now}
}

func (s *statusService) Set(ctx context.Context, userID uuid.UUID, in status.Input) (*models.Status, error) {
	v, err := status.Validate(in, s.now())
	if err != nil {
		return nil, err
	}
	st := models.Status{Icon: v.Icon, Text: v.Text, Background: v.Background, Foreground: v.Foreground, ExpiresAt: v.ExpiresAt}
	if err := s.repo.Set(ctx, userID, st); err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *statusService) Clear(ctx context.Context, userID uuid.UUID) error {
	_, err := s.repo.Clear(ctx, userID)
	return err
}
