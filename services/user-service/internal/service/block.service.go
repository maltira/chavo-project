package service

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/user-service/pkg/kafka"
)

type BlockService interface {
	GetBlockedUsers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.BlockedEntry, error)
	GetBlockStatus(ctx context.Context, myID, targetID uuid.UUID) (blockedByMe, blockedByThem bool, err error)
	BlockUser(ctx context.Context, myID, targetID uuid.UUID) error
	UnblockUser(ctx context.Context, myID, targetID uuid.UUID) error
}

type blockService struct {
	repo     repository.BlockRepository
	producer *kafka.Producer
	log      *zap.Logger
}

func NewBlockService(repo repository.BlockRepository, producer *kafka.Producer, log *zap.Logger) BlockService {
	return &blockService{repo: repo, producer: producer, log: log}
}

func (s *blockService) GetBlockedUsers(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.BlockedEntry, error) {
	return s.repo.GetBlockedUsers(ctx, userID, limit, offset)
}

func (s *blockService) GetBlockStatus(ctx context.Context, myID, targetID uuid.UUID) (blockedByMe, blockedByThem bool, err error) {
	return s.repo.CheckBlockBidirectional(ctx, myID, targetID)
}

func (s *blockService) BlockUser(ctx context.Context, myID, targetID uuid.UUID) error {
	if myID == targetID {
		return apperror.ErrSelfBlock
	}

	if err := s.repo.BlockUser(ctx, myID, targetID); err != nil {
		return err
	}

	evt := events.NewEvent(events.TypeUserBlocked, events.BlockPayload{
		BlockerID: myID,
		BlockedID: targetID,
	})
	if err := s.producer.Publish(ctx, events.TopicUserEvents, myID.String(), evt); err != nil {
		s.log.Error("Failed to publish user.blocked event",
			zap.String("blocker_id", myID.String()),
			zap.String("blocked_id", targetID.String()),
			zap.Error(err),
		)
	}
	return nil
}

func (s *blockService) UnblockUser(ctx context.Context, myID, targetID uuid.UUID) error {
	if myID == targetID {
		return apperror.ErrSelfUnblock
	}

	if err := s.repo.UnblockUser(ctx, myID, targetID); err != nil {
		return err
	}

	evt := events.NewEvent(events.TypeUserUnblocked, events.BlockPayload{
		BlockerID: myID,
		BlockedID: targetID,
	})
	if err := s.producer.Publish(ctx, events.TopicUserEvents, myID.String(), evt); err != nil {
		s.log.Error("Failed to publish user.unblocked event",
			zap.String("blocker_id", myID.String()),
			zap.String("blocked_id", targetID.String()),
			zap.Error(err),
		)
	}
	return nil
}
