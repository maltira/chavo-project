package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

type BlockService interface {
	GetAllBlocks(ctx context.Context, userID uuid.UUID) ([]models.Block, error)
	IsBlock(ctx context.Context, userID, targetID uuid.UUID) (bool, error)
	BlockUser(ctx context.Context, userID, blockedUserID uuid.UUID) (*models.Block, error)
	UnblockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error
}

type blockService struct {
	repo     repository.BlockRepository
	profRepo repository.ProfileRepository
}

func NewBlockService(repo repository.BlockRepository, profRepo repository.ProfileRepository) BlockService {
	return &blockService{repo: repo, profRepo: profRepo}
}

func (sc *blockService) GetAllBlocks(ctx context.Context, userID uuid.UUID) ([]models.Block, error) {
	return sc.repo.GetAllBlocks(ctx, userID)
}

func (sc *blockService) IsBlock(ctx context.Context, userID, targetID uuid.UUID) (bool, error) {
	return sc.repo.CheckBlock(ctx, userID, targetID)
}

func (sc *blockService) BlockUser(ctx context.Context, userID, blockedUserID uuid.UUID) (*models.Block, error) {
	if userID == blockedUserID {
		return nil, apperror.ErrSelfBlock
	}

	profile, err := sc.profRepo.FindByID(ctx, blockedUserID)
	if err != nil {
		return nil, err
	}

	block := models.Block{
		ProfileID:        userID,
		BlockedProfileID: blockedUserID,
	}

	if err = sc.repo.BlockUser(ctx, &block); err != nil {
		return nil, err
	}
	block.BlockedProfile = profile

	return &block, nil
}

func (sc *blockService) UnblockUser(ctx context.Context, userID, blockedUserID uuid.UUID) error {
	if userID == blockedUserID {
		return apperror.ErrSelfUnblock
	}
	return sc.repo.UnblockUser(ctx, userID, blockedUserID)
}
