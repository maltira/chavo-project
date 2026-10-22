package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

// maxPinnedMessages - отдельно для общих закрепов чата и для личных закрепов каждого участника
const maxPinnedMessages = 10

// pinOwner: в группе закрепляют только админы и только для всех; в личном чате - любой из двоих,
// для обоих или только для себя. nil - закреп для всех
func pinOwner(access *repository.Access, userID uuid.UUID, scope string) (*uuid.UUID, error) {
	switch scope {
	case models.PinScopeAll:
		if access.Type == models.ConversationGroup && access.Role != models.RoleAdmin {
			return nil, apperror.ErrForbidden
		}
		return nil, nil
	case models.PinScopeMe:
		if access.Type != models.ConversationDirect {
			return nil, apperror.ErrForbidden
		}
		return &userID, nil
	default:
		return nil, apperror.ErrIncorrectData
	}
}

// pinRecipients: общий закреп видят все участники, личный - только владелец
func (s *messageService) pinRecipients(ctx context.Context, q repository.DBTX, convID uuid.UUID, owners []*uuid.UUID) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(owners))
	for _, o := range owners {
		if o == nil {
			return s.convs.MemberIDs(ctx, q, convID)
		}
		ids = append(ids, *o)
	}
	return uniqueOthers(ids, uuid.Nil), nil
}

func (s *messageService) publishPinsChanged(ctx context.Context, q repository.DBTX, convID uuid.UUID, owners []*uuid.UUID) error {
	recipients, err := s.pinRecipients(ctx, q, convID, owners)
	if err != nil {
		return err
	}
	return s.publish(ctx, q, events.TopicConversationEvents, convID, events.TypePinsChanged, events.PinsChangedPayload{
		ConversationID: convID, MemberIDs: recipients,
	})
}

func (s *messageService) PinMessage(ctx context.Context, userID, convID, messageID uuid.UUID, scope string) (*models.PinnedMessage, error) {
	var res *models.PinnedMessage
	err := s.db.WithTx(ctx, func(tx repository.DBTX) error {
		// блокировка строки чата: лимит закрепов не обходится параллельными запросами
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		access, err := s.convs.Access(ctx, tx, convID, userID)
		if err != nil {
			return err
		}
		owner, err := pinOwner(access, userID, scope)
		if err != nil {
			return err
		}
		rec, err := s.msgs.FindVisible(ctx, tx, messageID, userID)
		if err != nil {
			return err
		}
		if rec.ConversationID != convID {
			return apperror.ErrNotFound
		}
		if rec.DeletedAt != nil {
			return apperror.ErrMessageDeleted
		}
		if rec.Kind == models.MessageSystem {
			return apperror.ErrForbidden
		}

		inserted, err := s.pins.Pin(ctx, tx, convID, messageID, owner, userID)
		if err != nil {
			return err
		}
		if inserted {
			n, err := s.pins.Count(ctx, tx, convID, owner)
			if err != nil {
				return err
			}
			if n > maxPinnedMessages {
				return apperror.ErrMessagePinLimit
			}
			if err = s.publishPinsChanged(ctx, tx, convID, []*uuid.UUID{owner}); err != nil {
				return err
			}
			// общий закреп видят все - в истории строка «закрепил сообщение»; личный остаётся незаметным
			if owner == nil {
				poster := systemPoster{msgs: s.msgs, convs: s.convs, outbox: s.outbox}
				if err = poster.postPinned(ctx, tx, convID, userID, messageID); err != nil {
					return err
				}
			}
		}

		pins, err := s.listPins(ctx, tx, userID, convID)
		if err != nil {
			return err
		}
		for i := range pins {
			if pins[i].Message.ID == messageID && pins[i].Scope == scope {
				res = &pins[i]
				return nil
			}
		}
		return apperror.ErrNotFound
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *messageService) UnpinMessage(ctx context.Context, userID, convID, messageID uuid.UUID, scope string) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		access, err := s.convs.Access(ctx, tx, convID, userID)
		if err != nil {
			return err
		}
		owner, err := pinOwner(access, userID, scope)
		if err != nil {
			return err
		}
		rec, err := s.msgs.FindByID(ctx, tx, messageID)
		if err != nil {
			return err
		}
		if rec.ConversationID != convID {
			return apperror.ErrNotFound
		}
		var changed []*uuid.UUID
		removed, err := s.pins.Unpin(ctx, tx, messageID, owner)
		if err != nil {
			return err
		}
		if removed {
			changed = append(changed, owner)
		}
		if access.Type == models.ConversationDirect {
			switch {
			case owner == nil:
				// «открепить у обоих» снимает и мой личный закреп этого сообщения
				mine, err := s.pins.Unpin(ctx, tx, messageID, &userID)
				if err != nil {
					return err
				}
				if mine {
					changed = append(changed, &userID)
				}
			case access.PeerID != nil:
				// «только у меня» при общем закрепе: у собеседника сообщение остаётся закреплённым - его личным
				// закрепом (лимит личных закрепов тут не проверяется: закреп у него уже был)
				moved, err := s.pins.MakePersonal(ctx, tx, messageID, *access.PeerID)
				if err != nil {
					return err
				}
				if moved {
					changed = append(changed, nil)
				}
			}
		}
		if len(changed) == 0 {
			return nil // повторное открепление идемпотентно и без события
		}
		return s.publishPinsChanged(ctx, tx, convID, changed)
	})
}

func (s *messageService) ListPins(ctx context.Context, userID, convID uuid.UUID) ([]models.PinnedMessage, error) {
	q := s.db.Q()
	if _, err := s.convs.Access(ctx, q, convID, userID); err != nil {
		return nil, err
	}
	return s.listPins(ctx, q, userID, convID)
}

func (s *messageService) listPins(ctx context.Context, q repository.DBTX, userID, convID uuid.UUID) ([]models.PinnedMessage, error) {
	recs, err := s.pins.List(ctx, q, convID, userID)
	if err != nil {
		return nil, err
	}
	res := make([]models.PinnedMessage, 0, len(recs))
	for i := range recs {
		m, err := s.toModel(&recs[i].Message)
		if err != nil {
			return nil, err
		}
		scope := models.PinScopeAll
		if recs[i].OwnerID != nil {
			scope = models.PinScopeMe
		}
		res = append(res, models.PinnedMessage{
			Message: *m, Scope: scope, PinnedBy: recs[i].PinnedBy, PinnedAt: recs[i].PinnedAt,
		})
	}
	return res, nil
}
