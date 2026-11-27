package service

import (
	"context"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

const (
	maxEmojiBytes              = 32
	maxDistinctReactionsPerMsg = 20
	maxUserReactionsPerMsg     = 5
)

func validateEmoji(raw string) (string, error) {
	e := strings.TrimSpace(raw)
	if e == "" || len(e) > maxEmojiBytes || !utf8.ValidString(e) {
		return "", apperror.ErrIncorrectData
	}
	hasNonASCII := false
	for _, r := range e {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", apperror.ErrIncorrectData
		}
		if r > unicode.MaxASCII {
			hasNonASCII = true
		}
	}
	if !hasNonASCII {
		return "", apperror.ErrIncorrectData
	}
	return e, nil
}

func (s *messageService) attachReactions(ctx context.Context, q repository.DBTX, items []models.Message, viewerID uuid.UUID) error {
	ids := make([]uuid.UUID, 0, len(items))
	for i := range items {
		if !items[i].IsDeleted && items[i].Kind != models.MessageSystem {
			ids = append(ids, items[i].ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	byMsg, err := s.reactions.ListByMessages(ctx, q, ids, viewerID)
	if err != nil {
		return err
	}
	for i := range items {
		items[i].Reactions = byMsg[items[i].ID]
	}
	return nil
}

func toEventReactions(in []models.ReactionSummary) []events.ReactionEventSummary {
	out := make([]events.ReactionEventSummary, len(in))
	for i, r := range in {
		out[i] = events.ReactionEventSummary{Emoji: r.Emoji, Count: r.Count, UserIDs: r.UserIDs}
	}
	return out
}

func (s *messageService) AddReaction(ctx context.Context, userID, messageID uuid.UUID, rawEmoji string) ([]models.ReactionSummary, error) {
	emoji, err := validateEmoji(rawEmoji)
	if err != nil {
		return nil, err
	}
	var summaries []models.ReactionSummary
	err = s.db.WithTx(ctx, func(tx repository.DBTX) error {
		// блокируем строку сообщения, чтобы параллельные запросы не обошли лимиты реакций
		rec, err := s.msgs.LockByID(ctx, tx, messageID)
		if err != nil {
			return err
		}
		if _, err := s.convs.Access(ctx, tx, rec.ConversationID, userID); err != nil {
			return err
		}
		if _, err := s.msgs.FindVisible(ctx, tx, messageID, userID); err != nil {
			return err
		}
		if rec.DeletedAt != nil {
			return apperror.ErrMessageDeleted
		}
		if rec.Kind == models.MessageSystem {
			return apperror.ErrForbidden
		}

		inserted, err := s.reactions.Add(ctx, tx, messageID, userID, emoji)
		if err != nil {
			return err
		}
		if inserted {
			distinct, err := s.reactions.CountDistinct(ctx, tx, messageID)
			if err != nil {
				return err
			}
			if distinct > maxDistinctReactionsPerMsg {
				return apperror.ErrReactionLimit
			}
			byUser, err := s.reactions.CountByUser(ctx, tx, messageID, userID)
			if err != nil {
				return err
			}
			if byUser > maxUserReactionsPerMsg {
				return apperror.ErrReactionLimit
			}
		}

		byMsg, err := s.reactions.ListByMessages(ctx, tx, []uuid.UUID{messageID}, userID)
		if err != nil {
			return err
		}
		summaries = byMsg[messageID]
		if !inserted {
			return nil
		}

		memberIDs, err := s.convs.MemberIDs(ctx, tx, rec.ConversationID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicMessageEvents, rec.ConversationID, events.TypeMessageReactionUpdated, events.MessageReactionUpdatedPayload{
			ConversationID: rec.ConversationID,
			MessageID:      messageID,
			UserID:         userID,
			Emoji:          emoji,
			Added:          true,
			Reactions:      toEventReactions(summaries),
			MemberIDs:      memberIDs,
		})
	})
	if err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *messageService) RemoveReaction(ctx context.Context, userID, messageID uuid.UUID, rawEmoji string) ([]models.ReactionSummary, error) {
	emoji, err := validateEmoji(rawEmoji)
	if err != nil {
		return nil, err
	}
	var summaries []models.ReactionSummary
	err = s.db.WithTx(ctx, func(tx repository.DBTX) error {
		rec, err := s.msgs.LockByID(ctx, tx, messageID)
		if err != nil {
			return err
		}
		if _, err := s.convs.Access(ctx, tx, rec.ConversationID, userID); err != nil {
			return err
		}
		if _, err := s.msgs.FindVisible(ctx, tx, messageID, userID); err != nil {
			return err
		}

		removed, err := s.reactions.Remove(ctx, tx, messageID, userID, emoji)
		if err != nil {
			return err
		}
		byMsg, err := s.reactions.ListByMessages(ctx, tx, []uuid.UUID{messageID}, userID)
		if err != nil {
			return err
		}
		summaries = byMsg[messageID]
		if !removed {
			return nil
		}

		memberIDs, err := s.convs.MemberIDs(ctx, tx, rec.ConversationID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicMessageEvents, rec.ConversationID, events.TypeMessageReactionUpdated, events.MessageReactionUpdatedPayload{
			ConversationID: rec.ConversationID,
			MessageID:      messageID,
			UserID:         userID,
			Emoji:          emoji,
			Added:          false,
			Reactions:      toEventReactions(summaries),
			MemberIDs:      memberIDs,
		})
	})
	if err != nil {
		return nil, err
	}
	return summaries, nil
}
