package ws

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
)

// ErrInvalidConversationID возвращается, если conversation_id не является валидным UUID
var ErrInvalidConversationID = errors.New("invalid conversation id")

// TypingSink - отправка кадра пользователям (реализует *hub.Hub)
type TypingSink interface {
	SendToUsers(userIDs []string, msg []byte)
}

type typingKey struct {
	userID string
	convID string
}

type memberCacheEntry struct {
	ids       []string
	expiresAt time.Time
}

type typingEvent struct {
	Type string     `json:"type"`
	Data typingData `json:"data"`
}

type typingData struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
}

const (
	defaultTypingInterval = 2 * time.Second
	defaultMembersTTL     = 15 * time.Second
	typingRPCTimeout      = 2 * time.Second
	maxThrottleEntries    = 10_000
	maxMemberCacheEntries = 5_000
)

// TypingService рассылает индикатор набора текста другим участникам чата без записи в БД и Kafka
type TypingService struct {
	sink        TypingSink
	convs       conversationv1.ConversationInternalServiceClient
	log         *zap.Logger
	minInterval time.Duration
	cacheTTL    time.Duration
	now         func() time.Time

	mu       sync.Mutex
	lastSent map[typingKey]time.Time
	members  map[string]memberCacheEntry
}

func NewTypingService(sink TypingSink, convs conversationv1.ConversationInternalServiceClient, log *zap.Logger) *TypingService {
	return &TypingService{
		sink:        sink,
		convs:       convs,
		log:         log,
		minInterval: defaultTypingInterval,
		cacheTTL:    defaultMembersTTL,
		now:         time.Now,
		lastSent:    make(map[typingKey]time.Time),
		members:     make(map[string]memberCacheEntry),
	}
}

// Invalidate сбрасывает кеш состава чата после добавления/удаления участников или удаления чата
func (s *TypingService) Invalidate(conversationID string) {
	s.mu.Lock()
	delete(s.members, conversationID)
	s.mu.Unlock()
}

func (s *TypingService) Notify(ctx context.Context, userID, conversationID string) error {
	if _, err := uuid.Parse(conversationID); err != nil {
		return ErrInvalidConversationID
	}

	now := s.now()
	key := typingKey{userID: userID, convID: conversationID}

	s.mu.Lock()
	if last, ok := s.lastSent[key]; ok && now.Sub(last) < s.minInterval {
		s.mu.Unlock()
		return nil
	}
	if len(s.lastSent) >= maxThrottleEntries {
		for k, t := range s.lastSent {
			if now.Sub(t) >= s.minInterval {
				delete(s.lastSent, k)
			}
		}
	}
	s.lastSent[key] = now

	var cached []string
	if entry, ok := s.members[conversationID]; ok && now.Before(entry.expiresAt) && slices.Contains(entry.ids, userID) {
		cached = entry.ids
	}
	s.mu.Unlock()

	ids := cached
	if ids == nil {
		callCtx, cancel := context.WithTimeout(ctx, typingRPCTimeout)
		resp, err := s.convs.ListMemberIDs(callCtx, &conversationv1.ListMemberIDsRequest{
			ConversationId: conversationID,
			UserId:         userID,
		})
		cancel()
		if err != nil {
			code := status.Code(err)
			if code != codes.PermissionDenied && code != codes.NotFound {
				s.log.Debug("typing member lookup failed", zap.String("conversation_id", conversationID), zap.Error(err))
			}
			return nil
		}
		ids = resp.GetUserIds()
		s.mu.Lock()
		if len(s.members) >= maxMemberCacheEntries {
			for k, e := range s.members {
				if !now.Before(e.expiresAt) {
					delete(s.members, k)
				}
			}
		}
		s.members[conversationID] = memberCacheEntry{ids: ids, expiresAt: now.Add(s.cacheTTL)}
		s.mu.Unlock()
	}

	others := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && id != userID {
			others = append(others, id)
		}
	}
	if len(others) == 0 {
		return nil
	}
	s.sink.SendToUsers(others, encode(typingEvent{
		Type: "conversation.typing",
		Data: typingData{ConversationID: conversationID, UserID: userID},
	}))
	return nil
}
