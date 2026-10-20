package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/media"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

// profileRepo запоминает, что сервис отдал на запись
type profileRepo struct {
	repository.ProfileRepository
	created *models.Profile
	updates map[string]any
}

func (r *profileRepo) Create(_ context.Context, p *models.Profile, _ *models.Settings) error {
	r.created = p
	return nil
}

func (r *profileRepo) Update(_ context.Context, _ uuid.UUID, updates map[string]any) error {
	r.updates = updates
	return nil
}

func (r *profileRepo) UsernameExists(context.Context, string) (bool, error) { return false, nil }

var ownAvatar = media.AvatarURL(strings.Repeat("a", 32) + ".jpg")

func TestAvatarAcceptsOnlyServerIssuedPath(t *testing.T) {
	repo := &profileRepo{}
	svc := service.NewProfileService(repo)
	ctx := context.Background()

	// чужой URL (картинка-трекер) не принимается ни при создании, ни при правке
	external := "https://tracker.example/pixel.gif"
	err := svc.Create(ctx, uuid.New(), service.CreateProfileInput{Username: "alice", DisplayName: "Alice", AvatarURL: &external})
	if !errors.Is(err, media.ErrInvalidAvatarURL) {
		t.Fatalf("create with external url: %v", err)
	}
	if err = svc.Update(ctx, uuid.New(), map[string]string{"avatar_url": external}); !errors.Is(err, media.ErrInvalidAvatarURL) {
		t.Fatalf("update with external url: %v", err)
	}

	if err = svc.Create(ctx, uuid.New(), service.CreateProfileInput{Username: "alice", DisplayName: "Alice", AvatarURL: &ownAvatar}); err != nil {
		t.Fatal(err)
	}
	if repo.created.AvatarURL == nil || *repo.created.AvatarURL != ownAvatar {
		t.Fatalf("created avatar: %v", repo.created.AvatarURL)
	}
	if err = svc.Update(ctx, uuid.New(), map[string]string{"avatar_url": ownAvatar}); err != nil {
		t.Fatal(err)
	}
	if got := repo.updates["avatar_url"].(*string); *got != ownAvatar {
		t.Fatalf("updated avatar: %v", *got)
	}
}

func TestAvatarEmptyStringClearsToNull(t *testing.T) {
	repo := &profileRepo{}
	svc := service.NewProfileService(repo)
	if err := svc.Update(context.Background(), uuid.New(), map[string]string{"avatar_url": ""}); err != nil {
		t.Fatal(err)
	}
	v, ok := repo.updates["avatar_url"]
	if !ok {
		t.Fatal("avatar_url must be written")
	}
	if p, isPtr := v.(*string); !isPtr || p != nil {
		t.Fatalf("cleared avatar must be NULL, got %#v", v)
	}
}
