package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

// systemPoster пишет системные сообщения (состав группы, закреп) в той же транзакции, что и само изменение
type systemPoster struct {
	msgs   repository.MessageRepository
	convs  repository.ConversationRepository
	outbox repository.OutboxRepository
}

// post вызывается после изменения состава: событие получают те, кто в группе сейчас
// (исключённый или вышедший его уже не увидит); чат поднимается в списке, как от обычного сообщения
func (p systemPoster) post(ctx context.Context, tx repository.DBTX, convID, actorID, targetID uuid.UUID, event string) error {
	return p.create(ctx, tx, &repository.MessageRecord{
		ConversationID: convID,
		SenderID:       actorID,
		SystemEvent:    &event,
		TargetID:       &targetID,
	})
}

// postPinned - «закрепил сообщение»: только для общего закрепа, его видят все участники
func (p systemPoster) postPinned(ctx context.Context, tx repository.DBTX, convID, actorID, messageID uuid.UUID) error {
	event := models.SystemPinned
	return p.create(ctx, tx, &repository.MessageRecord{
		ConversationID:   convID,
		SenderID:         actorID,
		SystemEvent:      &event,
		ReplyToMessageID: &messageID,
	})
}

func (p systemPoster) create(ctx context.Context, tx repository.DBTX, rec *repository.MessageRecord) error {
	convID, actorID := rec.ConversationID, rec.SenderID
	if err := p.msgs.CreateSystem(ctx, tx, rec); err != nil {
		return err
	}
	if err := p.convs.TouchLastMessage(ctx, tx, convID, rec.CreatedAt); err != nil {
		return err
	}
	memberIDs, err := p.convs.MemberIDs(ctx, tx, convID)
	if err != nil {
		return err
	}
	return p.outbox.Insert(ctx, tx, events.TopicMessageEvents, convID.String(), events.NewEvent(events.TypeMessageCreated, events.MessageCreatedPayload{
		ConversationID: convID,
		MessageID:      rec.ID,
		SenderID:       actorID,
		CreatedAt:      rec.CreatedAt,
		MemberIDs:      memberIDs,
		Kind:           models.MessageSystem,
		SystemEvent:    rec.SystemEvent,
		TargetID:       rec.TargetID,
		// у pinned - закреплённое сообщение
		ReplyToMessageID: rec.ReplyToMessageID,
	}))
}
