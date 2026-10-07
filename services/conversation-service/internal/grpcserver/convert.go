package grpcserver

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

// currentUser — пользователь из metadata x-user-id, проставленной Gateway.
func currentUser(ctx context.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(grpcx.IncomingUserID(ctx))
	if err != nil {
		return uuid.Nil, apperror.ErrUnauthorized
	}
	return id, nil
}

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperror.ErrInvalidUUID
	}
	return id, nil
}

func parseOptionalID(s *string) (*uuid.UUID, error) {
	if s == nil {
		return nil, nil
	}
	id, err := parseID(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func parseIDs(raw []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(raw))
	for _, r := range raw {
		id, err := parseID(r)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// page ограничивает limit и offset так же, как это делали HTTP-хендлеры.
func page(limit, offset int32, def, max int) (int, int) {
	l := int(limit)
	if l < 1 || l > max {
		l = def
	}
	o := int(offset)
	if o < 0 {
		o = 0
	}
	return l, o
}

func ts(t time.Time) *timestamppb.Timestamp { return timestamppb.New(t) }

func optTS(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func optStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func strs(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func toConversation(c *models.Conversation) *conversationv1.Conversation {
	return &conversationv1.Conversation{
		Id:               c.ID.String(),
		ConversationType: c.ConversationType,
		Visibility:       c.Visibility,
		Name:             c.Name,
		Description:      c.Description,
		AvatarUrl:        c.AvatarURL,
		LastMessageAt:    ts(c.LastMessageAt),
		CreatedAt:        ts(c.CreatedAt),
		UpdatedAt:        ts(c.UpdatedAt),
	}
}

func toMessage(m *models.Message) *conversationv1.Message {
	if m == nil {
		return nil
	}
	return &conversationv1.Message{
		Id:               m.ID.String(),
		ConversationId:   m.ConversationID.String(),
		SenderId:         m.SenderID.String(),
		Content:          m.Content,
		ReplyToMessageId: optStr(m.ReplyToMessageID),
		IsEdited:         m.IsEdited,
		IsDeleted:        m.IsDeleted,
		CreatedAt:        ts(m.CreatedAt),
		UpdatedAt:        optTS(m.UpdatedAt),
	}
}

func toSummary(s *models.ConversationSummary) *conversationv1.ConversationSummary {
	return &conversationv1.ConversationSummary{
		Conversation: toConversation(&s.Conversation),
		MyRole:       s.MyRole,
		PeerId:       optStr(s.PeerID),
		LastMessage:  toMessage(s.LastMessage),
		UnreadCount:  int32(s.UnreadCount),
	}
}

func toMember(m *models.Member) *conversationv1.Member {
	return &conversationv1.Member{
		ConversationId:    m.ConversationID.String(),
		UserId:            m.UserID.String(),
		Role:              m.Role,
		LastReadMessageId: optStr(m.LastReadMessageID),
		JoinedAt:          ts(m.JoinedAt),
	}
}

func toJoinRequest(j *models.JoinRequest) *conversationv1.JoinRequest {
	return &conversationv1.JoinRequest{
		Id:             j.ID.String(),
		ConversationId: j.ConversationID.String(),
		UserId:         j.UserID.String(),
		Status:         j.Status,
		CreatedAt:      ts(j.CreatedAt),
		UpdatedAt:      ts(j.UpdatedAt),
	}
}
