package service

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

type CreateProfileInput struct {
	Username    string
	DisplayName string
	Bio         *string
	AvatarURL   *string
}

type ProfileService interface {
	Create(ctx context.Context, userID uuid.UUID, input CreateProfileInput) error
	Update(ctx context.Context, userID uuid.UUID, data map[string]string) error
	GetAllBySearch(ctx context.Context, query string, limit, offset int) ([]models.Profile, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error)
	UpdateLastSeen(ctx context.Context, userID uuid.UUID, at time.Time) error
}

type profileService struct {
	repo repository.ProfileRepository
}

func NewProfileService(repo repository.ProfileRepository) ProfileService {
	return &profileService{repo: repo}
}

func (s *profileService) Create(ctx context.Context, userID uuid.UUID, input CreateProfileInput) error {
	n := utf8.RuneCountInString(input.Username)
	if n < 3 || n > 32 || !usernameRe.MatchString(input.Username) {
		return apperror.ErrInvalidUsername
	}

	n = utf8.RuneCountInString(input.DisplayName)
	if n < 1 || n > 100 {
		return apperror.ErrInvalidDisplayName
	}

	if input.Bio != nil && utf8.RuneCountInString(*input.Bio) > 255 {
		return apperror.ErrInvalidBio
	}

	exists, err := s.repo.UsernameExists(ctx, input.Username)
	if err != nil {
		return err
	}
	if exists {
		return apperror.ErrUsernameExists
	}

	profile := &models.Profile{
		UserID:      userID,
		Username:    input.Username,
		DisplayName: input.DisplayName,
		Bio:         input.Bio,
		AvatarURL:   input.AvatarURL,
	}
	return s.repo.Create(ctx, profile, &models.Settings{UserID: userID})
}

func (s *profileService) Update(ctx context.Context, userID uuid.UUID, data map[string]string) error {
	updates := make(map[string]string)

	if v, ok := data["username"]; ok {
		n := utf8.RuneCountInString(v)
		if n < 3 || n > 32 || !usernameRe.MatchString(v) {
			return apperror.ErrInvalidUsername
		}
		exists, err := s.repo.UsernameExists(ctx, v)
		if err != nil {
			return err
		}
		if exists {
			return apperror.ErrUsernameExists
		}
		updates["username"] = v
	}

	if v, ok := data["display_name"]; ok {
		n := utf8.RuneCountInString(v)
		if n < 1 || n > 100 {
			return apperror.ErrInvalidDisplayName
		}
		updates["display_name"] = v
	}

	if v, ok := data["bio"]; ok {
		if utf8.RuneCountInString(v) > 255 {
			return apperror.ErrInvalidBio
		}
		updates["bio"] = v
	}

	if v, ok := data["avatar_url"]; ok {
		updates["avatar_url"] = v
	}

	if len(updates) == 0 {
		return apperror.ErrNoColumnsToUpdate
	}

	return s.repo.Update(ctx, userID, updates)
}

func (s *profileService) GetAllBySearch(ctx context.Context, query string, limit, offset int) ([]models.Profile, error) {
	cleanQuery := strings.TrimSpace(strings.TrimPrefix(query, "@"))
	if utf8.RuneCountInString(cleanQuery) < 3 {
		return []models.Profile{}, nil
	}
	return s.repo.GetAllBySearch(ctx, cleanQuery, limit, offset)
}

func (s *profileService) FindByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	return s.repo.FindByID(ctx, userID)
}

func (s *profileService) UpdateLastSeen(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return s.repo.UpdateLastSeenAt(ctx, userID, at)
}
