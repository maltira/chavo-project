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
}

type MessageService interface {
	Send(ctx context.Context, senderID uuid.UUID, in SendMessageInput) (*models.Message, error)
	List(ctx context.Context, userID, convID uuid.UUID, before *uuid.UUID, limit int) ([]models.Message, error)
	Edit(ctx context.Context, userID, messageID uuid.UUID, content string) (*models.Message, error)
	Get(ctx context.Context, userID, messageID uuid.UUID) (*models.Message, error)
	Delete(ctx context.Context, userID, messageID uuid.UUID) error
	MarkRead(ctx context.Context, userID, convID, messageID uuid.UUID) error
	Readers(ctx context.Context, userID, convID, messageID uuid.UUID) ([]uuid.UUID, error)
}

type messageService struct {
	db     *repository.DB
	convs  repository.ConversationRepository
	msgs   repository.MessageRepository
	outbox repository.OutboxRepository
	users  userclient.Client
	cipher *crypto.Cipher
	log    *zap.Logger
}

func NewMessageService(
	db *repository.DB,
	convs repository.ConversationRepository,
	msgs repository.MessageRepository,
	outbox repository.OutboxRepository,
	users userclient.Client,
	cipher *crypto.Cipher,
	log *zap.Logger,
) MessageService {
	return &messageService{db: db, convs: convs, msgs: msgs, outbox: outbox, users: users, cipher: cipher, log: log}
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
		CreatedAt:        rec.CreatedAt,
		UpdatedAt:        rec.UpdatedAt,
	}
	if rec.DeletedAt == nil {
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

	// Сетевые проверки user-service выполняются до транзакции.
	var peerID uuid.UUID
	if in.RecipientID != nil {
		peerID = *in.RecipientID
		if peerID == senderID {
			return nil, apperror.ErrIncorrectData
		}
		exists, err := s.users.UserExists(ctx, peerID)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, apperror.ErrUserNotFound
		}
	} else {
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
			if err != nil || reply.ConversationID != convID {
				return apperror.ErrInvalidReply
			}
		}

		rec := &repository.MessageRecord{
			ConversationID:   convID,
			SenderID:         senderID,
			ContentEnc:       enc,
			ReplyToMessageID: in.ReplyToMessageID,
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
		}); err != nil {
			return err
		}

		content := in.Content
		result = &models.Message{
			ID: rec.ID, ConversationID: convID, SenderID: senderID, Content: &content,
			ReplyToMessageID: in.ReplyToMessageID, CreatedAt: rec.CreatedAt,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// resolveConversation возвращает id существующей conversation или лениво создаёт direct.
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
	q := s.db.Q()
	if _, err := s.convs.Access(ctx, q, convID, userID); err != nil {
		return nil, err
	}
	if before != nil {
		cur, err := s.msgs.FindVisible(ctx, q, *before, userID)
		if err != nil || cur.ConversationID != convID {
			return nil, apperror.ErrIncorrectData
		}
	}

	recs, err := s.msgs.List(ctx, q, convID, userID, before, limit)
	if err != nil {
		return nil, err
	}
	items := make([]models.Message, 0, len(recs))
	for i := range recs {
		m, err := s.toModel(&recs[i])
		if err != nil {
			return nil, err
		}
		items = append(items, *m)
	}
	return items, nil
}

func (s *messageService) Edit(ctx context.Context, userID, messageID uuid.UUID, content string) (*models.Message, error) {
	if err := validateContent(content); err != nil {
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
		if rec.SenderID != userID {
			return apperror.ErrForbidden
		}
		if rec.DeletedAt != nil {
			return apperror.ErrMessageDeleted
		}

		updatedAt, err := s.msgs.Update(ctx, tx, messageID, enc)
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
		}); err != nil {
			return err
		}

		result = &models.Message{
			ID: rec.ID, ConversationID: rec.ConversationID, SenderID: rec.SenderID, Content: &content,
			ReplyToMessageID: rec.ReplyToMessageID, IsEdited: true, CreatedAt: rec.CreatedAt, UpdatedAt: updatedAt,
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
	rec, err := s.msgs.FindForViewer(ctx, s.db.Q(), messageID, userID)
	if err != nil {
		return nil, err
	}
	return s.toModel(rec)
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
		if !isAuthor && !isGroupAdmin {
			return apperror.ErrForbidden
		}

		deleted, err := s.msgs.SoftDelete(ctx, tx, messageID)
		if err != nil || !deleted {
			return err // повторное удаление идемпотентно и не порождает событие
		}
		if err = s.outbox.ScrubMessageContent(ctx, tx, messageID); err != nil {
			return err
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
