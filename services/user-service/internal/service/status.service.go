package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/proto/status"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

type Publisher interface {
	Publish(ctx context.Context, topic, key string, event any) error
}

type StatusService interface {
	Set(ctx context.Context, userID uuid.UUID, in status.Input) (*models.Status, error)
	Clear(ctx context.Context, userID uuid.UUID) error
}

type statusService struct {
	repo repository.StatusRepository
	pub  Publisher
	log  *zap.Logger
	now  func() time.Time
}

func NewStatusService(repo repository.StatusRepository, pub Publisher, log *zap.Logger) StatusService {
	return &statusService{repo: repo, pub: pub, log: log, now: time.Now}
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
	s.publish(ctx, userID, &st)
	return &st, nil
}

func (s *statusService) Clear(ctx context.Context, userID uuid.UUID) error {
	removed, err := s.repo.Clear(ctx, userID)
	if err != nil {
		return err
	}
	if removed {
		s.publish(ctx, userID, nil)
	}
	return nil
}

func (s *statusService) publish(ctx context.Context, userID uuid.UUID, st *models.Status) {
	payload := events.StatusPayload{UserID: userID}
	if st != nil {
		payload.Status = &events.StatusFields{
			Icon: st.Icon, Text: st.Text, Background: st.Background, Foreground: st.Foreground, ExpiresAt: st.ExpiresAt,
		}
	}
	if err := s.pub.Publish(ctx, events.TopicUserEvents, userID.String(), events.NewEvent(events.TypeUserStatusUpdated, payload)); err != nil {
		s.log.Error("Failed to publish user.status.updated event", zap.String("user_id", userID.String()), zap.Error(err))
	}
}
