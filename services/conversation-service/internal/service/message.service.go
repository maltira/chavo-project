package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/client/userclient"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

const maxMessageLength = 4096

type SendMessageInput struct {
	ConversationID   *uuid.UUID
	RecipientID      *uuid.UUID
	Content          string
	ReplyToMessageID *uuid.UUID
	// Entities - форматирование текста; смещения в единицах UTF-16
	Entities []models.Entity
	// IsSilent - отправить без звука (без уведомления и звука у получателей)
	IsSilent bool
}

// ListMessagesQuery - параметры постраничного чтения истории; задаётся не более одного курсора
type ListMessagesQuery struct {
	Before *uuid.UUID
	After  *uuid.UUID
	Around *uuid.UUID
	Limit  int
}

type MessageService interface {
	Send(ctx context.Context, senderID uuid.UUID, in SendMessageInput) (*models.Message, error)
	List(ctx context.Context, userID, convID uuid.UUID, before *uuid.UUID, limit int) ([]models.Message, error)
	ListPage(ctx context.Context, userID, convID uuid.UUID, q ListMessagesQuery) (*models.MessagePage, error)
	// Edit заменяет текст и форматирование целиком
	Edit(ctx context.Context, userID, messageID uuid.UUID, content string, entities []models.Entity) (*models.Message, error)
	Get(ctx context.Context, userID, messageID uuid.UUID) (*models.Message, error)
	Delete(ctx context.Context, userID, messageID uuid.UUID) error
	DeleteBatch(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) error
	MarkRead(ctx context.Context, userID, convID, messageID uuid.UUID) error
	Forward(ctx context.Context, userID uuid.UUID, in ForwardInput) ([]ForwardResult, error)
	Readers(ctx context.Context, userID, convID, messageID uuid.UUID) ([]uuid.UUID, error)

	// закреплённые сообщения: scope all - для всех (в группе только админы), me - только для себя (личный чат);
	// закреп удалённого сообщения снимается вместе с ним. В личном чате UnpinMessage all снимает и мой личный
	// закреп, me при общем закрепе оставляет сообщение закреплённым у собеседника
	PinMessage(ctx context.Context, userID, convID, messageID uuid.UUID, scope string) (*models.PinnedMessage, error)
	UnpinMessage(ctx context.Context, userID, convID, messageID uuid.UUID, scope string) error
	// ListPins - новые сверху; только сообщения, которые участник видит
	ListPins(ctx context.Context, userID, convID uuid.UUID) ([]models.PinnedMessage, error)

	// реакции на сообщения: добавить или снять свою реакцию
	AddReaction(ctx context.Context, userID, messageID uuid.UUID, emoji string) ([]models.ReactionSummary, error)
	RemoveReaction(ctx context.Context, userID, messageID uuid.UUID, emoji string) ([]models.ReactionSummary, error)
}

type messageService struct {
	db        *repository.DB
	convs     repository.ConversationRepository
	msgs      repository.MessageRepository
	pins      repository.PinRepository
	reactions repository.ReactionRepository
	outbox    repository.OutboxRepository
	users     userclient.Client
	cipher    *crypto.Cipher
	log       *zap.Logger
}

func NewMessageService(
	db *repository.DB,
	convs repository.ConversationRepository,
	msgs repository.MessageRepository,
	pins repository.PinRepository,
	reactions repository.ReactionRepository,
	outbox repository.OutboxRepository,
	users userclient.Client,
	cipher *crypto.Cipher,
	log *zap.Logger,
) MessageService {
	return &messageService{db: db, convs: convs, msgs: msgs, pins: pins, reactions: reactions, outbox: outbox, users: users, cipher: cipher, log: log}
}

func validateContent(content string) error {
	if strings.TrimSpace(content) == "" || utf8.RuneCountInString(content) > maxMessageLength {
		return apperror.ErrIncorrectData
	}
	return nil
}

func directKey(a, b uuid.UUID) string {
	sa, sb := a.String(), b.String()
	if sa > sb {
		sa, sb = sb, sa
	}
	return sa + ":" + sb
}

func (s *messageService) publish(ctx context.Context, q repository.DBTX, topic string, convID uuid.UUID, eventType string, payload any) error {
	return s.outbox.Insert(ctx, q, topic, convID.String(), events.NewEvent(eventType, payload))
}

// toModel расшифровывает запись; у удалённых сообщений content = nil.
func (s *messageService) toModel(rec *repository.MessageRecord) (*models.Message, error) {
	return recordToModel(s.cipher, rec)
}

func recordToModel(c *crypto.Cipher, rec *repository.MessageRecord) (*models.Message, error) {
	m := &models.Message{
		ID:               rec.ID,
		ConversationID:   rec.ConversationID,
		SenderID:         rec.SenderID,
		ReplyToMessageID: rec.ReplyToMessageID,
		IsEdited:         rec.IsEdited,
		IsDeleted:        rec.DeletedAt != nil,
		Kind:             rec.Kind,
		SystemEvent:      rec.SystemEvent,
		TargetID:         rec.TargetID,
		CreatedAt:        rec.CreatedAt,
		UpdatedAt:        rec.UpdatedAt,

		IsForwarded:         rec.IsForwarded,
		ForwardedFromUserID: rec.ForwardedFromUserID,
		Entities:            rec.Entities,
		IsSilent:            rec.IsSilent,
	}
	if m.Kind == "" {
		m.Kind = models.MessageUser
	}
	// у системного сообщения текста нет: его собирает клиент
	if rec.DeletedAt == nil && rec.Kind != models.MessageSystem {
		plain, err := c.Decrypt(rec.ContentEnc)
		if err != nil {
			return nil, err
		}
		text := string(plain)
		m.Content = &text
	}
	return m, nil
}

func (s *messageService) Send(ctx context.Context, senderID uuid.UUID, in SendMessageInput) (*models.Message, error) {
	if (in.ConversationID == nil) == (in.RecipientID == nil) {
		return nil, apperror.ErrIncorrectData
	}
	if err := validateContent(in.Content); err != nil {
		return nil, err
	}
	entities, err := normalizeEntities(in.Content, in.Entities)
	if err != nil {
		return nil, err
	}

	// Сетевые проверки user-service выполняются до транзакции. Себе («Избранное») писать можно всегда:
	// проверять существование и блокировки не с кем.
	var peerID uuid.UUID
	if in.RecipientID != nil && *in.RecipientID != senderID {
		peerID = *in.RecipientID
		exists, err := s.users.UserExists(ctx, peerID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, apperror.ErrUserNotFound
		}
	} else if in.ConversationID != nil {
		access, err := s.convs.Access(ctx, s.db.Q(), *in.ConversationID, senderID)
		if err != nil {
			return nil, err
		}
		if access.Type == models.ConversationDirect && access.PeerID != nil {
			peerID = *access.PeerID
		}
	}
	if peerID != uuid.Nil {
		if err := s.users.CheckMessagingAllowed(ctx, senderID, peerID); err != nil {
			return nil, err
		}
	}

	enc, err := s.cipher.Encrypt([]byte(in.Content))
	if err != nil {
		return nil, err
	}

	var result *models.Message
	err = s.db.WithTx(ctx, func(tx repository.DBTX) error {
		convID, err := s.resolveConversation(ctx, tx, senderID, in)
		if err != nil {
			return err
		}

		if _, err = s.convs.Access(ctx, tx, convID, senderID); err != nil {
			return err
		}

		if in.ReplyToMessageID != nil {
			reply, err := s.msgs.FindByID(ctx, tx, *in.ReplyToMessageID)
			// на системное сообщение не отвечают
			if err != nil || reply.ConversationID != convID || reply.Kind == models.MessageSystem {
				return apperror.ErrInvalidReply
			}
		}

		rec := &repository.MessageRecord{
			ConversationID:   convID,
			SenderID:         senderID,
			ContentEnc:       enc,
			ReplyToMessageID: in.ReplyToMessageID,
			Entities:         entities,
			IsSilent:         in.IsSilent,
		}
		if err = s.msgs.Create(ctx, tx, rec); err != nil {
			return err
		}
		if err = s.convs.TouchLastMessage(ctx, tx, convID, rec.CreatedAt); err != nil {
			return err
		}

		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		if err = s.publish(ctx, tx, events.TopicMessageEvents, convID, events.TypeMessageCreated, events.MessageCreatedPayload{
			ConversationID:   convID,
			MessageID:        rec.ID,
			SenderID:         senderID,
			ReplyToMessageID: in.ReplyToMessageID,
			CreatedAt:        rec.CreatedAt,
			MemberIDs:        memberIDs,
			ContentEnc:       enc,
			Kind:             models.MessageUser,
			Entities:         entities,
			IsSilent:         in.IsSilent,
		}); err != nil {
			return err
		}

		content := in.Content
		result = &models.Message{
			ID: rec.ID, ConversationID: convID, SenderID: senderID, Content: &content, Kind: models.MessageUser,
			ReplyToMessageID: in.ReplyToMessageID, CreatedAt: rec.CreatedAt, Entities: entities, IsSilent: in.IsSilent,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// resolveConversation возвращает id существующей conversation или лениво создаёт direct;
// direct с самим собой («Избранное») - ключ <id>:<id> и одна строка участника
func (s *messageService) resolveConversation(ctx context.Context, tx repository.DBTX, senderID uuid.UUID, in SendMessageInput) (uuid.UUID, error) {
	if in.ConversationID != nil {
		return *in.ConversationID, nil
	}

	convID, created, err := s.convs.CreateDirectIfAbsent(ctx, tx, directKey(senderID, *in.RecipientID))
	if err != nil {
		return uuid.Nil, err
	}
	if !created {
		return convID, nil
	}

	for _, uid := range []uuid.UUID{senderID, *in.RecipientID} {
		if err = s.convs.AddMember(ctx, tx, convID, uid, models.RoleAdmin); err != nil {
			return uuid.Nil, err
		}
	}
	memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
	if err != nil {
		return uuid.Nil, err
	}
	err = s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeConversationCreated, events.ConversationCreatedPayload{
		ConversationID:   convID,
		ConversationType: models.ConversationDirect,
		CreatedBy:        senderID,
		MemberIDs:        memberIDs,
	})
	return convID, err
}

func (s *messageService) List(ctx context.Context, userID, convID uuid.UUID, before *uuid.UUID, limit int) ([]models.Message, error) {
	page, err := s.ListPage(ctx, userID, convID, ListMessagesQuery{Before: before, Limit: limit})
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (s *messageService) ListPage(ctx context.Context, userID, convID uuid.UUID, query ListMessagesQuery) (*models.MessagePage, error) {
	cursors := 0
	if query.Before != nil {
		cursors++
	}
	if query.After != nil {
		cursors++
	}
	if query.Around != nil {
		cursors++
	}
	if cursors > 1 {
		return nil, apperror.ErrIncorrectData
	}
	limit := query.Limit
	if limit < 1 {
		limit = 50
	}

	q := s.db.Q()
	if _, err := s.convs.Access(ctx, q, convID, userID); err != nil {
		return nil, err
	}

	var (
		recs     []repository.MessageRecord
		hasOlder bool
		hasNewer bool
	)

	switch {
	case query.Around != nil:
		cur, err := s.msgs.FindVisible(ctx, q, *query.Around, userID)
		if err != nil || cur.ConversationID != convID {
			return nil, apperror.ErrNotFound
		}
		newerBudget := (limit - 1) / 2
		newer, err := s.msgs.ListAfter(ctx, q, convID, userID, *query.Around, newerBudget+1)
		if err != nil {
			return nil, err
		}
		if len(newer) > newerBudget {
			hasNewer = true
			newer = newer[:newerBudget]
		}
		// если новее целевого меньше половины окна, отдаём остаток бюджета под более старые сообщения
		olderBudget := limit - 1 - len(newer)
		older, err := s.msgs.List(ctx, q, convID, userID, query.Around, olderBudget+1)
		if err != nil {
			return nil, err
		}
		if len(older) > olderBudget {
			hasOlder = true
			older = older[:olderBudget]
		}
		recs = make([]repository.MessageRecord, 0, len(newer)+1+len(older))
		for i := len(newer) - 1; i >= 0; i-- {
			recs = append(recs, newer[i])
		}
		recs = append(recs, *cur)
		recs = append(recs, older...)

	case query.After != nil:
		cur, err := s.msgs.FindVisible(ctx, q, *query.After, userID)
		if err != nil || cur.ConversationID != convID {
			return nil, apperror.ErrIncorrectData
		}
		raw, err := s.msgs.ListAfter(ctx, q, convID, userID, *query.After, limit+1)
		if err != nil {
			return nil, err
		}
		if len(raw) > limit {
			hasNewer = true
			raw = raw[:limit]
		}
		// приводим к порядку от новых к старым, как в остальных режимах
		recs = make([]repository.MessageRecord, len(raw))
		for i := range raw {
			recs[len(raw)-1-i] = raw[i]
		}
		hasOlder = len(recs) > 0

	default:
		if query.Before != nil {
			cur, err := s.msgs.FindVisible(ctx, q, *query.Before, userID)
			if err != nil || cur.ConversationID != convID {
				return nil, apperror.ErrIncorrectData
			}
		}
		raw, err := s.msgs.List(ctx, q, convID, userID, query.Before, limit+1)
		if err != nil {
			return nil, err
		}
		if len(raw) > limit {
			hasOlder = true
			raw = raw[:limit]
		}
		recs = raw
		hasNewer = query.Before != nil && len(recs) > 0
	}

	items := make([]models.Message, 0, len(recs))
	for i := range recs {
		m, err := s.toModel(&recs[i])
		if err != nil {
			return nil, err
		}
		items = append(items, *m)
	}
	if err := s.attachReactions(ctx, q, items, userID); err != nil {
		return nil, err
	}
	return &models.MessagePage{Items: items, HasOlder: hasOlder, HasNewer: hasNewer}, nil
}

func (s *messageService) Edit(ctx context.Context, userID, messageID uuid.UUID, content string, entities []models.Entity) (*models.Message, error) {
	if err := validateContent(content); err != nil {
		return nil, err
	}
	entities, err := normalizeEntities(content, entities)
	if err != nil {
		return nil, err
	}
	enc, err := s.cipher.Encrypt([]byte(content))
	if err != nil {
		return nil, err
	}

	var result *models.Message
	err = s.db.WithTx(ctx, func(tx repository.DBTX) error {
		rec, err := s.msgs.FindByID(ctx, tx, messageID)
		if err != nil {
			return err
		}
		if _, err = s.convs.Access(ctx, tx, rec.ConversationID, userID); err != nil {
			return err
		}
		if _, err = s.msgs.FindVisible(ctx, tx, messageID, userID); err != nil {
			return err
		}
		// системное сообщение не правится: у него нет текста; пересланное - копия чужого текста
		if rec.SenderID != userID || rec.Kind == models.MessageSystem || rec.IsForwarded {
			return apperror.ErrForbidden
		}
		if rec.DeletedAt != nil {
			return apperror.ErrMessageDeleted
		}

		updatedAt, err := s.msgs.Update(ctx, tx, messageID, enc, entities)
		if err != nil {
			return err
		}
		memberIDs, err := s.convs.MemberIDs(ctx, tx, rec.ConversationID)
		if err != nil {
			return err
		}
		if err = s.publish(ctx, tx, events.TopicMessageEvents, rec.ConversationID, events.TypeMessageUpdated, events.MessageUpdatedPayload{
			ConversationID: rec.ConversationID,
			MessageID:      messageID,
			SenderID:       userID,
			UpdatedAt:      *updatedAt,
			MemberIDs:      memberIDs,
			ContentEnc:     enc,
			Entities:       entities,
		}); err != nil {
			return err
		}

		byMsg, err := s.reactions.ListByMessages(ctx, tx, []uuid.UUID{messageID}, userID)
		if err != nil {
			return err
		}
		result = &models.Message{
			ID: rec.ID, ConversationID: rec.ConversationID, SenderID: rec.SenderID, Content: &content,
			ReplyToMessageID: rec.ReplyToMessageID, IsEdited: true, CreatedAt: rec.CreatedAt, UpdatedAt: updatedAt,
			Entities: entities, IsSilent: rec.IsSilent, Reactions: byMsg[messageID],
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Get отдаёт одно сообщение: видимое участнику или цель reply из видимого ему сообщения.
func (s *messageService) Get(ctx context.Context, userID, messageID uuid.UUID) (*models.Message, error) {
	q := s.db.Q()
	rec, err := s.msgs.FindForViewer(ctx, q, messageID, userID)
	if err != nil {
		return nil, err
	}
	m, err := s.toModel(rec)
	if err != nil {
		return nil, err
	}
	if !m.IsDeleted {
		byMsg, err := s.reactions.ListByMessages(ctx, q, []uuid.UUID{m.ID}, userID)
		if err != nil {
			return nil, err
		}
		m.Reactions = byMsg[m.ID]
	}
	return m, nil
}

func (s *messageService) Delete(ctx context.Context, userID, messageID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		rec, err := s.msgs.FindByID(ctx, tx, messageID)
		if err != nil {
			return err
		}
		access, err := s.convs.Access(ctx, tx, rec.ConversationID, userID)
		if err != nil {
			return err
		}
		if _, err = s.msgs.FindVisible(ctx, tx, messageID, userID); err != nil {
			return err
		}

		// Чужие сообщения удалять может только admin группы; в direct — только автор.
		isAuthor := rec.SenderID == userID
		isGroupAdmin := access.Type == models.ConversationGroup && access.Role == models.RoleAdmin
		if (!isAuthor && !isGroupAdmin) || rec.Kind == models.MessageSystem {
			return apperror.ErrForbidden
		}

		deleted, err := s.msgs.SoftDelete(ctx, tx, messageID)
		if err != nil || !deleted {
			return err // повторное удаление идемпотентно и не порождает событие
		}
		if err = s.outbox.ScrubMessageContent(ctx, tx, messageID); err != nil {
			return err
		}
		if err = s.reactions.DeleteByMessage(ctx, tx, messageID); err != nil {
			return err
		}
		// удалённое сообщение не остаётся закреплённым ни у кого
		owners, err := s.pins.DeleteForMessage(ctx, tx, messageID)
		if err != nil {
			return err
		}
		if len(owners) > 0 {
			if err = s.publishPinsChanged(ctx, tx, rec.ConversationID, owners); err != nil {
				return err
			}
		}
		memberIDs, err := s.convs.MemberIDs(ctx, tx, rec.ConversationID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicMessageEvents, rec.ConversationID, events.TypeMessageDeleted, events.MessageDeletedPayload{
			ConversationID: rec.ConversationID,
			MessageID:      messageID,
			DeletedBy:      userID,
			MemberIDs:      memberIDs,
		})
	})
}

// DeleteBatch мягко удаляет до maxForwardMessages сообщений одного чата в одной транзакции:
// если хотя бы одно недоступно или запрещено - откатывается всё целиком
func (s *messageService) DeleteBatch(ctx context.Context, userID uuid.UUID, messageIDs []uuid.UUID) error {
	ids := uniqueIDs(messageIDs)
	if len(ids) == 0 || len(ids) > maxForwardMessages {
		return apperror.ErrIncorrectData
	}

	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		var convID uuid.UUID
		var access *repository.Access
		for _, id := range ids {
			rec, err := s.msgs.FindByID(ctx, tx, id)
			if err != nil {
				return err
			}
			if convID == uuid.Nil {
				convID = rec.ConversationID
				access, err = s.convs.Access(ctx, tx, convID, userID)
				if err != nil {
					return err
				}
			} else if rec.ConversationID != convID {
				return apperror.ErrIncorrectData
			}
			if _, err = s.msgs.FindVisible(ctx, tx, id, userID); err != nil {
				return err
			}
			isAuthor := rec.SenderID == userID
			isGroupAdmin := access.Type == models.ConversationGroup && access.Role == models.RoleAdmin
			if (!isAuthor && !isGroupAdmin) || rec.Kind == models.MessageSystem {
				return apperror.ErrForbidden
			}
		}

		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		var pinOwners []*uuid.UUID
		for _, id := range ids {
			deleted, err := s.msgs.SoftDelete(ctx, tx, id)
			if err != nil {
				return err
			}
			if !deleted {
				continue
			}
			if err = s.outbox.ScrubMessageContent(ctx, tx, id); err != nil {
				return err
			}
			if err = s.reactions.DeleteByMessage(ctx, tx, id); err != nil {
				return err
			}
			owners, err := s.pins.DeleteForMessage(ctx, tx, id)
			if err != nil {
				return err
			}
			pinOwners = append(pinOwners, owners...)
			if err = s.publish(ctx, tx, events.TopicMessageEvents, convID, events.TypeMessageDeleted, events.MessageDeletedPayload{
				ConversationID: convID,
				MessageID:      id,
				DeletedBy:      userID,
				MemberIDs:      memberIDs,
			}); err != nil {
				return err
			}
		}
		if len(pinOwners) > 0 {
			if err = s.publishPinsChanged(ctx, tx, convID, pinOwners); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *messageService) MarkRead(ctx context.Context, userID, convID, messageID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Access(ctx, tx, convID, userID); err != nil {
			return err
		}
		rec, err := s.msgs.FindVisible(ctx, tx, messageID, userID)
		if err != nil || rec.ConversationID != convID {
			return apperror.ErrNotFound
		}

		moved, prev, err := s.msgs.SetReadCursor(ctx, tx, convID, userID, messageID)
		if err != nil || !moved {
			return err // курсор уже дальше — ничего не меняем
		}
		authors, err := s.msgs.AuthorsBetween(ctx, tx, convID, prev, messageID, userID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicMessageEvents, convID, events.TypeMessageRead, events.MessageReadPayload{
			ConversationID: convID,
			ReaderID:       userID,
			MessageID:      messageID,
			AuthorIDs:      authors,
		})
	})
}

func (s *messageService) Readers(ctx context.Context, userID, convID, messageID uuid.UUID) ([]uuid.UUID, error) {
	q := s.db.Q()
	if _, err := s.convs.Access(ctx, q, convID, userID); err != nil {
		return nil, err
	}
	rec, err := s.msgs.FindVisible(ctx, q, messageID, userID)
	if err != nil || rec.ConversationID != convID {
		return nil, apperror.ErrNotFound
	}
	return s.msgs.ListReaders(ctx, q, convID, messageID)
}
