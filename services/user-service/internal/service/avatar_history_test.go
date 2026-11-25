package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/media"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type avatarRepo struct {
	repository.AvatarHistoryRepository
	listed   []uuid.UUID
	addedURL string
	limit    int
}

func (r *avatarRepo) List(_ context.Context, owner uuid.UUID) ([]models.AvatarPhoto, error) {
	r.listed = append(r.listed, owner)
	return []models.AvatarPhoto{{ID: uuid.New(), URL: "x", IsCurrent: true}}, nil
}

func (r *avatarRepo) Add(_ context.Context, _ uuid.UUID, url string, limit int) ([]string, error) {
	r.addedURL, r.limit = url, limit
	return nil, nil
}

type blocksRepo struct {
	repository.BlockRepository
	blockedBy map[uuid.UUID]bool
}

func (b *blocksRepo) BlockedBy(_ context.Context, _ uuid.UUID, owners []uuid.UUID) (map[uuid.UUID]bool, error) {
	res := map[uuid.UUID]bool{}
	for _, o := range owners {
		if b.blockedBy[o] {
			res[o] = true
		}
	}
	return res, nil
}

func TestAvatarHistoryHiddenFromBlockedViewer(t *testing.T) {
	owner, viewer := uuid.New(), uuid.New()
	repo := &avatarRepo{}
	svc := service.NewAvatarHistoryService(repo, &blocksRepo{blockedBy: map[uuid.UUID]bool{owner: true}})
	ctx := context.Background()

	// заблокировавший владелец: история пуста и до базы дело не доходит
	items, err := svc.List(ctx, viewer, owner)
	if err != nil || len(items) != 0 || len(repo.listed) != 0 {
		t.Fatalf("blocked viewer: %v %v listed=%v", items, err, repo.listed)
	}
	// свою историю видно всегда, чужую - если не заблокирован
	if items, _ = svc.List(ctx, owner, owner); len(items) != 1 {
		t.Fatalf("own history: %v", items)
	}
	if items, _ = svc.List(ctx, owner, uuid.New()); len(items) != 1 {
		t.Fatalf("not blocked: %v", items)
	}
}

func TestAvatarHistoryAddChecksPathAndPassesTheLimit(t *testing.T) {
	repo := &avatarRepo{}
	svc := service.NewAvatarHistoryService(repo, &blocksRepo{})
	ctx := context.Background()
	user := uuid.New()

	for name, url := range map[string]string{"foreign": "https://example.com/a.png", "large file": media.AvatarURL(strings.Repeat("a", 32) + "_l.jpg")} {
		if _, err := svc.Add(ctx, user, url); !errors.Is(err, media.ErrInvalidAvatarURL) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// пустой адрес - не фото
	if _, err := svc.Add(ctx, user, ""); !errors.Is(err, apperror.ErrIncorrectData) {
		t.Errorf("empty: %v", err)
	}
	own := media.AvatarURL(strings.Repeat("b", 32) + ".jpg")
	if _, err := svc.Add(ctx, user, own); err != nil || repo.addedURL != own || repo.limit != media.MaxAvatarHistory {
		t.Fatalf("add: %v url=%q limit=%d", err, repo.addedURL, repo.limit)
	}
}
