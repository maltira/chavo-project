package service_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type settingsRepo struct {
	repository.SettingsRepository
	updates map[string]any
}

func (r *settingsRepo) UpdateSettings(_ context.Context, _ uuid.UUID, updates map[string]any) error {
	r.updates = updates
	return nil
}

func (r *settingsRepo) GetSettings(context.Context, uuid.UUID) (*models.Settings, error) {
	return &models.Settings{}, nil
}

func TestFavoriteEmojisValidation(t *testing.T) {
	ctx := context.Background()
	repo := &settingsRepo{}
	svc := service.NewSettingsService(repo)
	update := func(list []string) error {
		repo.updates = nil
		return svc.UpdateSettings(ctx, uuid.New(), map[string]any{"favorite_emojis": list})
	}

	// порядок сохраняется, повторы убираются
	if err := update([]string{"🔥", "😂", "🔥", "👨‍👩‍👧‍👦", "🇷🇺"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.updates["favorite_emojis"].([]string); !slices.Equal(got, []string{"🔥", "😂", "👨‍👩‍👧‍👦", "🇷🇺"}) {
		t.Fatalf("saved: %v", got)
	}
	// пустой список очищает избранное (и записывается, а не пропускается)
	if err := update([]string{}); err != nil {
		t.Fatal(err)
	}
	if got, ok := repo.updates["favorite_emojis"].([]string); !ok || len(got) != 0 {
		t.Fatalf("cleared: %#v", repo.updates)
	}

	bad := map[string][]string{
		"empty entry": {"🔥", ""},
		"space":       {"a b"},
		"newline":     {"🔥\n"},
		"control":     {"\x00"},
		"too long":    {strings.Repeat("😀", 9)},
		"too many":    nil,
	}
	many := make([]string, service.MaxFavoriteEmojis+1)
	for i := range many {
		many[i] = strings.Repeat("a", 1) + string(rune('а'+i))
	}
	bad["too many"] = many
	for name, list := range bad {
		if err := update(list); !errors.Is(err, apperror.ErrIncorrectData) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// ровно предел проходит
	if err := update(many[:service.MaxFavoriteEmojis]); err != nil {
		t.Fatalf("exact limit: %v", err)
	}
}
