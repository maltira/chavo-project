package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/mute"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

const maxPinnedChats = 5

func (s *conversationService) SetMute(ctx context.Context, userID, convID uuid.UUID, duration *string) (*models.ChatSettings, error) {
	var until *time.Time
	if duration != nil {
		t, err := mute.Until(s.now(), *duration)
		if err != nil {
			return nil, err
		}
		until = &t
	}
	return s.changeSettings(ctx, userID, convID, false, func(_ *models.ChatSettings, _ int) (map[string]any, error) {
		return map[string]any{"muted_until": until}, nil
	})
}

func (s *conversationService) SetArchived(ctx context.Context, userID, convID uuid.UUID, archived bool) (*models.ChatSettings, error) {
	return s.changeSettings(ctx, userID, convID, false, func(cur *models.ChatSettings, _ int) (map[string]any, error) {
		if !archived {
			if cur.ArchivedAt == nil {
				return nil, nil
			}
			return map[string]any{"archived_at": nil}, nil
		}
		if cur.ArchivedAt != nil {
			return nil, nil
		}
		return map[string]any{"archived_at": s.now().UTC(), "pinned_at": nil}, nil
	})
}

func (s *conversationService) SetPinned(ctx context.Context, userID, convID uuid.UUID, pinned bool) (*models.ChatSettings, error) {
	return s.changeSettings(ctx, userID, convID, pinned, func(cur *models.ChatSettings, pinnedCount int) (map[string]any, error) {
		if !pinned {
			if cur.PinnedAt == nil {
				return nil, nil
			}
			return map[string]any{"pinned_at": nil}, nil
		}
		if cur.PinnedAt != nil {
			return nil, nil
		}
		if pinnedCount >= maxPinnedChats {
			return nil, apperror.ErrPinLimit
		}
		return map[string]any{"pinned_at": s.now().UTC(), "archived_at": nil}, nil
	})
}

// changeSettings: change получает текущие настройки под блокировкой строки и возвращает изменения;
// пустые изменения - ничего не пишется и событие не уходит. countPinned - заранее заблокировать все строки
// участия пользователя и посчитать его закрепы (для лимита)
func (s *conversationService) changeSettings(ctx context.Context, userID, convID uuid.UUID, countPinned bool,
	change func(cur *models.ChatSettings, pinnedCount int) (map[string]any, error)) (*models.ChatSettings, error) {
	var res *models.ChatSettings
	err := s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Access(ctx, tx, convID, userID); err != nil {
			return err
		}
		// все строки участия блокируются раньше своей: иначе два параллельных закрепа ждали бы друг друга
		pinnedCount := 0
		if countPinned {
			n, err := s.convs.CountPinnedChats(ctx, tx, userID)
			if err != nil {
				return err
			}
			pinnedCount = n
		}
		cur, err := s.convs.LockChatSettings(ctx, tx, convID, userID)
		if err != nil {
			return err
		}
		updates, err := change(cur, pinnedCount)
		if err != nil {
			return err
		}
		if len(updates) == 0 {
			res = cur
			return nil
		}
		if res, err = s.convs.UpdateChatSettings(ctx, tx, convID, userID, updates); err != nil {
			return err
		}
		return s.outbox.Insert(ctx, tx, events.TopicConversationEvents, convID.String(),
			events.NewEvent(events.TypeChatSettingsUpdated, events.ChatSettingsUpdatedPayload{
				ConversationID: convID, UserID: userID,
				MutedUntil: res.MutedUntil, ArchivedAt: res.ArchivedAt, PinnedAt: res.PinnedAt,
				MemberIDs: []uuid.UUID{userID},
			}))
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
