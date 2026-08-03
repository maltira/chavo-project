package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

type ProfileService interface {
	Create(ctx context.Context, userID uuid.UUID) error
	Update(ctx context.Context, userID uuid.UUID, data map[string]any) error
	GetAllBySearch(ctx context.Context, search string, limit int) ([]models.Profile, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error)
	IsUsernameFree(ctx context.Context, username string) (bool, error)
}

type profileService struct {
	repo repository.ProfileRepository
}

func NewProfileService(repo repository.ProfileRepository) ProfileService {
	return &profileService{repo: repo}
}

func (sc *profileService) Create(ctx context.Context, userID uuid.UUID) error {
	name := "user_" + userID.String()[:8]
	profile := &models.Profile{
		ID:        userID,
		Username:  name,
		FullName:  name,
		AvatarURL: "https://i.ibb.co/2Y0R1nDf/avatar-white.png",
	}
	settings := &models.Settings{
		ProfileID: userID,
	}
	return sc.repo.Create(ctx, profile, settings)
}

func (sc *profileService) Update(ctx context.Context, userID uuid.UUID, data map[string]any) error {
	updates := make(map[string]any)

	if v, ok := data["username"]; ok {
		str, isStr := v.(string)
		if !isStr || len(str) < 4 || len(str) > 16 {
			return apperror.ErrInvalidUsername
		}
		exists, err := sc.repo.UsernameExists(ctx, str)
		if err != nil {
			return err
		}
		if exists {
			return apperror.ErrUsernameExists
		}
		updates["username"] = str
	}

	if v, ok := data["full_name"]; ok {
		str, isStr := v.(string)
		if !isStr || len(str) < 1 || len(str) > 100 {
			return apperror.ErrInvalidFullName
		}
		updates["full_name"] = str
	}

	if v, ok := data["bio"]; ok {
		str, isStr := v.(string)
		if !isStr || len(str) > 500 {
			return apperror.ErrInvalidBio
		}
		updates["bio"] = str
	}

	if v, ok := data["avatar_url"]; ok {
		updates["avatar_url"] = v
	}

	if v, ok := data["birth_date"]; ok {
		updates["birth_date"] = v
	}

	if len(updates) == 0 {
		return apperror.ErrNoColumnsToUpdate
	}

	return sc.repo.Update(ctx, userID, updates)
}

func (sc *profileService) GetAllBySearch(ctx context.Context, search string, limit int) ([]models.Profile, error) {
	return sc.repo.GetAllBySearch(ctx, search, limit)
}

func (sc *profileService) FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	return sc.repo.FindByID(ctx, userID)
}

func (sc *profileService) IsUsernameFree(ctx context.Context, username string) (bool, error) {
	exists, err := sc.repo.UsernameExists(ctx, username)
	if err != nil {
		return false, err
	}
	return !exists, nil
}
