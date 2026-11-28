package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

type ConversationService interface {
	// List - не архивные чаты или (archived) только архивные
	List(ctx context.Context, userID uuid.UUID, archived bool, limit, offset int) ([]models.ConversationSummary, error)
	Get(ctx context.Context, userID, convID uuid.UUID) (*models.ConversationSummary, error)
	SearchPublicGroups(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.PublicGroup, error)
	DirectPeers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	// MemberIDs возвращает участников чата после проверки, что userID в нём состоит
	MemberIDs(ctx context.Context, userID, convID uuid.UUID) ([]uuid.UUID, error)
	// ClearHistory очищает историю чата только для userID; сам чат и его настройки остаются в списке
	ClearHistory(ctx context.Context, userID, convID uuid.UUID) error

	// личные настройки чата: изменение уходит событием только вкладкам самого участника
	// SetMute: duration nil - звук включён
	SetMute(ctx context.Context, userID, convID uuid.UUID, duration *string) (*models.ChatSettings, error)
	// SetArchived: архивация снимает закреп
	SetArchived(ctx context.Context, userID, convID uuid.UUID, archived bool) (*models.ChatSettings, error)
	// SetPinned: до maxPinnedChats, закреп достаёт чат из архива
	SetPinned(ctx context.Context, userID, convID uuid.UUID, pinned bool) (*models.ChatSettings, error)
}

type conversationService struct {
	db     *repository.DB
	convs  repository.ConversationRepository
	pins   repository.PinRepository
	outbox repository.OutboxRepository
	cipher *crypto.Cipher
	now    func() time.Time
}

func NewConversationService(db *repository.DB, convs repository.ConversationRepository, pins repository.PinRepository, outbox repository.OutboxRepository, cipher *crypto.Cipher) ConversationService {
	return &conversationService{db: db, convs: convs, pins: pins, outbox: outbox, cipher: cipher, now: time.Now}
}

func (s *conversationService) List(ctx context.Context, userID uuid.UUID, archived bool, limit, offset int) ([]models.ConversationSummary, error) {
	recs, err := s.convs.ListSummaries(ctx, s.db.Q(), userID, nil, &archived, limit, offset)
	if err != nil {
		return nil, err
	}
	items := make([]models.ConversationSummary, 0, len(recs))
	for i := range recs {
		item, err := s.toSummary(&recs[i])
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, nil
}

func (s *conversationService) Get(ctx context.Context, userID, convID uuid.UUID) (*models.ConversationSummary, error) {
	if _, err := s.convs.Access(ctx, s.db.Q(), convID, userID); err != nil {
		return nil, err
	}
	recs, err := s.convs.ListSummaries(ctx, s.db.Q(), userID, &convID, nil, 1, 0)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, apperror.ErrNotFound
	}
	item, err := s.toSummary(&recs[0])
	if err != nil {
		return nil, err
	}
	// код ссылки закрытой группы видят только её админы
	if item.MyRole == models.RoleAdmin {
		if item.InviteCode, err = decryptInviteCode(s.cipher, recs[0].Conversation.InviteCodeEnc); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s *conversationService) toSummary(rec *repository.SummaryRecord) (*models.ConversationSummary, error) {
	item := &models.ConversationSummary{
		Conversation:    rec.Conversation,
		MyRole:          rec.MyRole,
		PeerID:              rec.PeerID,
		UnreadCount:         rec.UnreadCount,
		UnreadMentionsCount: rec.UnreadMentionsCount,
		LastMessageRead: rec.LastMessageRead,
		MembersCount:    rec.MembersCount,
		ChatSettings:    rec.Settings,
		IsSaved:         rec.IsSaved,
	}
	if rec.LastMessage != nil {
		m, err := recordToModel(s.cipher, rec.LastMessage)
		if err != nil {
			return nil, err
		}
		item.LastMessage = m
	}
	return item, nil
}

// SearchPublicGroups ищет только публичные группы; запрос короче 3 символов даёт пустой результат.
func (s *conversationService) SearchPublicGroups(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.PublicGroup, error) {
	q := strings.TrimSpace(query)
	if utf8.RuneCountInString(q) < 3 {
		return []models.PublicGroup{}, nil
	}
	return s.convs.SearchPublicGroups(ctx, s.db.Q(), userID, q, limit, offset)
}

func (s *conversationService) DirectPeers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return s.convs.DirectPeers(ctx, s.db.Q(), userID)
}

func (s *conversationService) MemberIDs(ctx context.Context, userID, convID uuid.UUID) ([]uuid.UUID, error) {
	q := s.db.Q()
	if _, err := s.convs.Access(ctx, q, convID, userID); err != nil {
		return nil, err
	}
	return s.convs.MemberIDs(ctx, q, convID)
}

func (s *conversationService) ClearHistory(ctx context.Context, userID, convID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Access(ctx, tx, convID, userID); err != nil {
			return err
		}
		clearedAt, err := s.convs.ClearHistory(ctx, tx, convID, userID)
		if err != nil {
			return err
		}
		if _, err = s.pins.DeleteOwnForConversation(ctx, tx, convID, userID); err != nil {
			return err
		}
		return s.outbox.Insert(ctx, tx, events.TopicConversationEvents, convID.String(),
			events.NewEvent(events.TypeHistoryCleared, events.HistoryClearedPayload{
				ConversationID: convID,
				UserID:         userID,
				ClearedAt:      clearedAt,
				MemberIDs:      []uuid.UUID{userID},
			}))
	})
}
