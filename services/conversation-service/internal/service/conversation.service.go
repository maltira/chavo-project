package service

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
	"github.com/maltira/chavo-project-backend/services/conversation-service/pkg/crypto"
)

type ConversationService interface {
	List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.ConversationSummary, error)
	Get(ctx context.Context, userID, convID uuid.UUID) (*models.ConversationSummary, error)
	SearchPublicGroups(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.PublicGroup, error)
	DirectPeers(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

type conversationService struct {
	db     *repository.DB
	convs  repository.ConversationRepository
	cipher *crypto.Cipher
}

func NewConversationService(db *repository.DB, convs repository.ConversationRepository, cipher *crypto.Cipher) ConversationService {
	return &conversationService{db: db, convs: convs, cipher: cipher}
}

func (s *conversationService) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.ConversationSummary, error) {
	recs, err := s.convs.ListSummaries(ctx, s.db.Q(), userID, nil, limit, offset)
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
	recs, err := s.convs.ListSummaries(ctx, s.db.Q(), userID, &convID, 1, 0)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, apperror.ErrNotFound
	}
	return s.toSummary(&recs[0])
}

func (s *conversationService) toSummary(rec *repository.SummaryRecord) (*models.ConversationSummary, error) {
	item := &models.ConversationSummary{
		Conversation: rec.Conversation,
		MyRole:       rec.MyRole,
		PeerID:       rec.PeerID,
		UnreadCount:  rec.UnreadCount,
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
