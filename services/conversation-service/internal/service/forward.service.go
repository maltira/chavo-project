package service

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

const (
	maxForwardMessages = 20
	// чатов за одну пересылку, считая «Избранное»
	maxForwardTargets = 5
)

type ForwardInput struct {
	MessageIDs      []uuid.UUID
	ConversationIDs []uuid.UUID
	// ToSaved - также в «Избранное» (создаётся, если его ещё нет)
	ToSaved bool
	// HideAuthor - «без указания автора»: копия - обычное сообщение пересылающего
	HideAuthor bool
}

type ForwardResult struct {
	ConversationID uuid.UUID
	Messages       []models.Message
}

// forwardSource - пересылаемое сообщение: расшифрованный текст и кого показать автором
type forwardSource struct {
	rec    *repository.MessageRecord
	plain  []byte
	origin *uuid.UUID // первый автор; nil - его имя скрыто
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	return uniqueOthers(ids, uuid.Nil)
}

// Forward копирует сообщения одного чата в мои чаты: копия независима от оригинала (свой шифртекст,
// правка и удаление оригинала её не меняют), ответ-цитата не переносится. Автор берётся первый в цепочке
// пересылок; его настройка hide_forward_author проверяется сейчас, и если user-service недоступен,
// пересылка отклоняется - имя не раскрывается.
func (s *messageService) Forward(ctx context.Context, userID uuid.UUID, in ForwardInput) ([]ForwardResult, error) {
	msgIDs := uniqueIDs(in.MessageIDs)
	targets := uniqueIDs(in.ConversationIDs)
	targetCount := len(targets)
	if in.ToSaved {
		targetCount++
	}
	if len(msgIDs) == 0 || len(msgIDs) > maxForwardMessages || targetCount == 0 || targetCount > maxForwardTargets {
		return nil, apperror.ErrIncorrectData
	}

	q := s.db.Q()
	sources := make([]forwardSource, 0, len(msgIDs))
	var sourceConv uuid.UUID
	for _, id := range msgIDs {
		rec, err := s.msgs.FindVisible(ctx, q, id, userID)
		if err != nil {
			return nil, err
		}
		switch {
		case rec.DeletedAt != nil:
			return nil, apperror.ErrMessageDeleted
		case rec.Kind == models.MessageSystem:
			return nil, apperror.ErrForbidden
		case sourceConv != uuid.Nil && rec.ConversationID != sourceConv:
			return nil, apperror.ErrIncorrectData // все сообщения - из одного чата
		}
		sourceConv = rec.ConversationID
		plain, err := s.cipher.Decrypt(rec.ContentEnc)
		if err != nil {
			return nil, err
		}
		src := forwardSource{rec: rec, plain: plain}
		switch {
		case !rec.IsForwarded:
			src.origin = &rec.SenderID
		case rec.ForwardedFromUserID != nil:
			src.origin = rec.ForwardedFromUserID
		}
		sources = append(sources, src)
	}
	// в чате копии идут в том же порядке, что и оригиналы
	sort.Slice(sources, func(i, j int) bool {
		a, b := sources[i].rec, sources[j].rec
		if a.CreatedAt.Equal(b.CreatedAt) {
			return a.ID.String() < b.ID.String()
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})

	// сеть - до транзакции: блокировки в личных чатах и скрытые имена авторов
	for _, convID := range targets {
		access, err := s.convs.Access(ctx, q, convID, userID)
		if err != nil {
			return nil, err
		}
		if access.Type == models.ConversationDirect && access.PeerID != nil {
			if err = s.users.CheckMessagingAllowed(ctx, userID, *access.PeerID); err != nil {
				return nil, err
			}
		}
	}
	if !in.HideAuthor {
		authors := make([]uuid.UUID, 0, len(sources))
		for _, src := range sources {
			if src.origin != nil {
				authors = append(authors, *src.origin)
			}
		}
		hidden, err := s.users.ForwardAuthorsHidden(ctx, uniqueIDs(authors))
		if err != nil {
			return nil, err
		}
		for i := range sources {
			if o := sources[i].origin; o != nil && hidden[*o] {
				sources[i].origin = nil
			}
		}
	}

	var results []ForwardResult
	err := s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if in.ToSaved {
			savedID, err := s.resolveConversation(ctx, tx, userID, SendMessageInput{RecipientID: &userID})
			if err != nil {
				return err
			}
			targets = uniqueIDs(append(targets, savedID))
		}
		results = make([]ForwardResult, 0, len(targets))
		for _, convID := range targets {
			res, err := s.forwardInto(ctx, tx, userID, convID, sources, in.HideAuthor)
			if err != nil {
				return err
			}
			results = append(results, *res)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

func (s *messageService) forwardInto(ctx context.Context, tx repository.DBTX, userID, convID uuid.UUID,
	sources []forwardSource, hideAuthor bool) (*ForwardResult, error) {
	// участие проверяется ещё раз в транзакции: могли исключить, пока шла сеть
	if _, err := s.convs.Access(ctx, tx, convID, userID); err != nil {
		return nil, err
	}
	memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
	if err != nil {
		return nil, err
	}
	res := &ForwardResult{ConversationID: convID, Messages: make([]models.Message, 0, len(sources))}
	for _, src := range sources {
		enc, err := s.cipher.Encrypt(src.plain)
		if err != nil {
			return nil, err
		}
		rec := &repository.MessageRecord{
			ConversationID: convID, SenderID: userID, ContentEnc: enc,
			IsForwarded: !hideAuthor,
		}
		if !hideAuthor {
			rec.ForwardedFromUserID = src.origin
		}
		if err = s.msgs.Create(ctx, tx, rec); err != nil {
			return nil, err
		}
		if err = s.convs.TouchLastMessage(ctx, tx, convID, rec.CreatedAt); err != nil {
			return nil, err
		}
		if err = s.publish(ctx, tx, events.TopicMessageEvents, convID, events.TypeMessageCreated, events.MessageCreatedPayload{
			ConversationID:      convID,
			MessageID:           rec.ID,
			SenderID:            userID,
			CreatedAt:           rec.CreatedAt,
			MemberIDs:           memberIDs,
			ContentEnc:          enc,
			Kind:                models.MessageUser,
			IsForwarded:         rec.IsForwarded,
			ForwardedFromUserID: rec.ForwardedFromUserID,
		}); err != nil {
			return nil, err
		}
		content := string(src.plain)
		res.Messages = append(res.Messages, models.Message{
			ID: rec.ID, ConversationID: convID, SenderID: userID, Content: &content, Kind: models.MessageUser,
			CreatedAt: rec.CreatedAt, IsForwarded: rec.IsForwarded, ForwardedFromUserID: rec.ForwardedFromUserID,
		})
	}
	return res, nil
}
