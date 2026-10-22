package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
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
	outbox repository.OutboxRepository
	cipher *crypto.Cipher
	now    func() time.Time
}

func NewConversationService(db *repository.DB, convs repository.ConversationRepository, outbox repository.OutboxRepository, cipher *crypto.Cipher) ConversationService {
	return &conversationService{db: db, convs: convs, outbox: outbox, cipher: cipher, now: time.Now}
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
		PeerID:          rec.PeerID,
		UnreadCount:     rec.UnreadCount,
		LastMessageRead: rec.LastMessageRead,
		MembersCount:    rec.MembersCount,
		ChatSettings:    rec.Settings,
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
