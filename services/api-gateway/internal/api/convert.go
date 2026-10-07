package api

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/maltira/chavo-project-backend/proto/gen/go/auth/v1"
	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
)

// JSON внешнего API повторяет прежние REST-ответы сервисов; proto наружу не отдаётся.

func tsPtr(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}

type tokenJSON struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type sessionJSON struct {
	ID         string    `json:"id"`
	DeviceName *string   `json:"device_name,omitempty"`
	UserAgent  *string   `json:"user_agent,omitempty"`
	IPAddress  *string   `json:"ip_address,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func toSession(s *authv1.Session) sessionJSON {
	return sessionJSON{
		ID:         s.GetId(),
		DeviceName: s.DeviceName,
		UserAgent:  s.UserAgent,
		IPAddress:  s.IpAddress,
		CreatedAt:  s.GetCreatedAt().AsTime(),
		ExpiresAt:  s.GetExpiresAt().AsTime(),
	}
}

type profileJSON struct {
	UserID      string  `json:"user_id"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Bio         *string `json:"bio,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	// LastSeenAt = null, если пользователь скрыл онлайн-статус.
	LastSeenAt *time.Time `json:"last_seen_at"`
	Online     bool       `json:"online"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

func toProfile(p *userv1.Profile) profileJSON {
	return profileJSON{
		UserID:      p.GetUserId(),
		Username:    p.GetUsername(),
		DisplayName: p.GetDisplayName(),
		Bio:         p.Bio,
		AvatarURL:   p.AvatarUrl,
		LastSeenAt:  tsPtr(p.GetLastSeenAt()),
		Online:      p.GetOnline(),
		CreatedAt:   p.GetCreatedAt().AsTime(),
		UpdatedAt:   p.GetUpdatedAt().AsTime(),
	}
}

type settingsJSON struct {
	UserID            string    `json:"user_id"`
	AllowGroupInvites bool      `json:"allow_group_invites"`
	ShowOnlineStatus  bool      `json:"show_online_status"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toSettings(s *userv1.Settings) settingsJSON {
	return settingsJSON{
		UserID:            s.GetUserId(),
		AllowGroupInvites: s.GetAllowGroupInvites(),
		ShowOnlineStatus:  s.GetShowOnlineStatus(),
		CreatedAt:         s.GetCreatedAt().AsTime(),
		UpdatedAt:         s.GetUpdatedAt().AsTime(),
	}
}

type blockedJSON struct {
	BlockedUserID string    `json:"blocked_user_id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	AvatarURL     *string   `json:"avatar_url,omitempty"`
	BlockedAt     time.Time `json:"blocked_at"`
}

func toBlocked(b *userv1.BlockedEntry) blockedJSON {
	return blockedJSON{
		BlockedUserID: b.GetBlockedUserId(),
		Username:      b.GetUsername(),
		DisplayName:   b.GetDisplayName(),
		AvatarURL:     b.AvatarUrl,
		BlockedAt:     b.GetBlockedAt().AsTime(),
	}
}

type conversationJSON struct {
	ID               string    `json:"id"`
	ConversationType string    `json:"conversation_type"`
	Visibility       string    `json:"visibility"`
	Name             *string   `json:"name"`
	Description      *string   `json:"description"`
	AvatarURL        *string   `json:"avatar_url"`
	LastMessageAt    time.Time `json:"last_message_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func toConversation(c *conversationv1.Conversation) conversationJSON {
	return conversationJSON{
		ID:               c.GetId(),
		ConversationType: c.GetConversationType(),
		Visibility:       c.GetVisibility(),
		Name:             c.Name,
		Description:      c.Description,
		AvatarURL:        c.AvatarUrl,
		LastMessageAt:    c.GetLastMessageAt().AsTime(),
		CreatedAt:        c.GetCreatedAt().AsTime(),
		UpdatedAt:        c.GetUpdatedAt().AsTime(),
	}
}

// groupJSON: invite_token есть только в момент создания токена.
type groupJSON struct {
	conversationJSON
	InviteToken *string `json:"invite_token,omitempty"`
}

type messageJSON struct {
	ID               string     `json:"id"`
	ConversationID   string     `json:"conversation_id"`
	SenderID         string     `json:"sender_id"`
	Content          *string    `json:"content"`
	ReplyToMessageID *string    `json:"reply_to_message_id"`
	IsEdited         bool       `json:"is_edited"`
	IsDeleted        bool       `json:"is_deleted"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        *time.Time `json:"updated_at"`
}

func toMessage(m *conversationv1.Message) *messageJSON {
	if m == nil {
		return nil
	}
	return &messageJSON{
		ID:               m.GetId(),
		ConversationID:   m.GetConversationId(),
		SenderID:         m.GetSenderId(),
		Content:          m.Content,
		ReplyToMessageID: m.ReplyToMessageId,
		IsEdited:         m.GetIsEdited(),
		IsDeleted:        m.GetIsDeleted(),
		CreatedAt:        m.GetCreatedAt().AsTime(),
		UpdatedAt:        tsPtr(m.GetUpdatedAt()),
	}
}

type summaryJSON struct {
	conversationJSON
	MyRole      string       `json:"my_role"`
	PeerID      *string      `json:"peer_id"`
	LastMessage *messageJSON `json:"last_message"`
	UnreadCount int32        `json:"unread_count"`
}

func toSummary(s *conversationv1.ConversationSummary) summaryJSON {
	return summaryJSON{
		conversationJSON: toConversation(s.GetConversation()),
		MyRole:           s.GetMyRole(),
		PeerID:           s.PeerId,
		LastMessage:      toMessage(s.GetLastMessage()),
		UnreadCount:      s.GetUnreadCount(),
	}
}

type memberJSON struct {
	ConversationID    string    `json:"conversation_id"`
	UserID            string    `json:"user_id"`
	Role              string    `json:"role"`
	LastReadMessageID *string   `json:"last_read_message_id"`
	JoinedAt          time.Time `json:"joined_at"`
}

func toMember(m *conversationv1.Member) memberJSON {
	return memberJSON{
		ConversationID:    m.GetConversationId(),
		UserID:            m.GetUserId(),
		Role:              m.GetRole(),
		LastReadMessageID: m.LastReadMessageId,
		JoinedAt:          m.GetJoinedAt().AsTime(),
	}
}

type banJSON struct {
	UserID    string    `json:"user_id"`
	BannedBy  string    `json:"banned_by"`
	CreatedAt time.Time `json:"created_at"`
}

func toBan(b *conversationv1.Ban) banJSON {
	return banJSON{UserID: b.GetUserId(), BannedBy: b.GetBannedBy(), CreatedAt: b.GetCreatedAt().AsTime()}
}

type joinRequestJSON struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	UserID         string    `json:"user_id"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toJoinRequest(r *conversationv1.JoinRequest) joinRequestJSON {
	return joinRequestJSON{
		ID:             r.GetId(),
		ConversationID: r.GetConversationId(),
		UserID:         r.GetUserId(),
		Status:         r.GetStatus(),
		CreatedAt:      r.GetCreatedAt().AsTime(),
		UpdatedAt:      r.GetUpdatedAt().AsTime(),
	}
}

type inviteInfoJSON struct {
	ConversationID string  `json:"conversation_id"`
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	AvatarURL      *string `json:"avatar_url"`
	MembersCount   int32   `json:"members_count"`
	IsMember       bool    `json:"is_member"`
	RequestStatus  *string `json:"request_status"`
}

func toInviteInfo(i *conversationv1.InviteInfo) inviteInfoJSON {
	return inviteInfoJSON{
		ConversationID: i.GetConversationId(),
		Name:           i.Name,
		Description:    i.Description,
		AvatarURL:      i.AvatarUrl,
		MembersCount:   i.GetMembersCount(),
		IsMember:       i.GetIsMember(),
		RequestStatus:  i.RequestStatus,
	}
}

type publicGroupJSON struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	AvatarURL    *string `json:"avatar_url"`
	MembersCount int32   `json:"members_count"`
	IsMember     bool    `json:"is_member"`
}

func toPublicGroup(g *conversationv1.PublicGroup) publicGroupJSON {
	return publicGroupJSON{
		ID:           g.GetId(),
		Name:         g.GetName(),
		Description:  g.Description,
		AvatarURL:    g.AvatarUrl,
		MembersCount: g.GetMembersCount(),
		IsMember:     g.GetIsMember(),
	}
}

// listJSON — страница с offset-пагинацией.
type listJSON[T any] struct {
	Items  []T   `json:"items"`
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

func mapSlice[S, T any](in []S, f func(S) T) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}
