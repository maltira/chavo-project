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
	// Status = null, если статуса нет, он истёк или скрыт от заблокированного.
	Status      *statusJSON      `json:"status"`
	AvatarShape *avatarShapeJSON `json:"avatar_shape"`
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
		Status:      toStatus(p.GetStatus()),
		AvatarShape: toAvatarShape(p.GetAvatarShape()),
	}
}

type settingsJSON struct {
	UserID            string    `json:"user_id"`
	AllowGroupInvites bool      `json:"allow_group_invites"`
	ShowOnlineStatus  bool      `json:"show_online_status"`
	HideForwardAuthor bool      `json:"hide_forward_author"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func toSettings(s *userv1.Settings) settingsJSON {
	return settingsJSON{
		UserID:            s.GetUserId(),
		AllowGroupInvites: s.GetAllowGroupInvites(),
		ShowOnlineStatus:  s.GetShowOnlineStatus(),
		HideForwardAuthor: s.GetHideForwardAuthor(),
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
	// Status — только у групп; null, если статуса нет или он истёк.
	Status *statusJSON `json:"status"`
	// AvatarShape - только у групп; null, если фигуры нет
	AvatarShape *avatarShapeJSON `json:"avatar_shape"`
	// Slug - код ссылки открытой группы; null у закрытых и личных
	Slug *string `json:"slug"`
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
		Status:           toStatus(c.GetStatus()),
		AvatarShape:      toAvatarShape(c.GetAvatarShape()),
		Slug:             c.Slug,
	}
}

// groupJSON: invite_token - код ссылки закрытой группы в момент его выпуска
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
	// Kind - user или system; у системного вместо текста system_event (joined, added, left, removed) и target_id
	Kind        string  `json:"kind"`
	SystemEvent *string `json:"system_event"`
	TargetID    *string `json:"target_id"`
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
		Kind:             m.GetKind(),
		SystemEvent:      m.SystemEvent,
		TargetID:         m.TargetId,
	}
}

type summaryJSON struct {
	conversationJSON
	MyRole      string       `json:"my_role"`
	PeerID      *string      `json:"peer_id"`
	LastMessage *messageJSON `json:"last_message"`
	UnreadCount int32        `json:"unread_count"`
	// LastMessageRead - последнее сообщение прочитал кто-то кроме его автора
	LastMessageRead bool  `json:"last_message_read"`
	MembersCount    int32 `json:"members_count"`
	// InviteCode - код ссылки закрытой группы; только у её админов в GET /conversations/:id
	InviteCode *string `json:"invite_code"`
	// IsSaved - «Избранное»: личный чат с самим собой
	IsSaved bool `json:"is_saved"`
	chatSettingsJSON
}

// chatSettingsJSON - личные настройки чата вызывающего
type chatSettingsJSON struct {
	MutedUntil *time.Time `json:"muted_until"`
	ArchivedAt *time.Time `json:"archived_at"`
	PinnedAt   *time.Time `json:"pinned_at"`
}

type chatSettingsResponseJSON struct {
	ConversationID string `json:"conversation_id"`
	chatSettingsJSON
}

func toChatSettings(s *conversationv1.ChatSettings) chatSettingsResponseJSON {
	return chatSettingsResponseJSON{
		ConversationID: s.GetConversationId(),
		chatSettingsJSON: chatSettingsJSON{
			MutedUntil: tsPtr(s.MutedUntil), ArchivedAt: tsPtr(s.ArchivedAt), PinnedAt: tsPtr(s.PinnedAt),
		},
	}
}

func toSummary(s *conversationv1.ConversationSummary) summaryJSON {
	return summaryJSON{
		conversationJSON: toConversation(s.GetConversation()),
		MyRole:           s.GetMyRole(),
		PeerID:           s.PeerId,
		LastMessage:      toMessage(s.GetLastMessage()),
		UnreadCount:      s.GetUnreadCount(),
		LastMessageRead:  s.GetLastMessageRead(),
		MembersCount:     s.GetMembersCount(),
		InviteCode:       s.InviteCode,
		IsSaved:          s.GetIsSaved(),
		chatSettingsJSON: chatSettingsJSON{
			MutedUntil: tsPtr(s.MutedUntil), ArchivedAt: tsPtr(s.ArchivedAt), PinnedAt: tsPtr(s.PinnedAt),
		},
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
	ConversationID string           `json:"conversation_id"`
	Visibility     string           `json:"visibility"`
	Slug           *string          `json:"slug"`
	Name           *string          `json:"name"`
	Description    *string          `json:"description"`
	AvatarURL      *string          `json:"avatar_url"`
	MembersCount   int32            `json:"members_count"`
	IsMember       bool             `json:"is_member"`
	RequestStatus  *string          `json:"request_status"`
	Status         *statusJSON      `json:"status"`
	AvatarShape    *avatarShapeJSON `json:"avatar_shape"`
}

func toInviteInfo(i *conversationv1.InviteInfo) inviteInfoJSON {
	return inviteInfoJSON{
		ConversationID: i.GetConversationId(),
		Visibility:     i.GetVisibility(),
		Slug:           i.Slug,
		Name:           i.Name,
		Description:    i.Description,
		AvatarURL:      i.AvatarUrl,
		MembersCount:   i.GetMembersCount(),
		IsMember:       i.GetIsMember(),
		RequestStatus:  i.RequestStatus,
		Status:         toStatus(i.GetStatus()),
		AvatarShape:    toAvatarShape(i.GetAvatarShape()),
	}
}

type publicGroupJSON struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  *string          `json:"description"`
	AvatarURL    *string          `json:"avatar_url"`
	Slug         string           `json:"slug"`
	MembersCount int32            `json:"members_count"`
	IsMember     bool             `json:"is_member"`
	Status       *statusJSON      `json:"status"`
	AvatarShape  *avatarShapeJSON `json:"avatar_shape"`
}

func toPublicGroup(g *conversationv1.PublicGroup) publicGroupJSON {
	return publicGroupJSON{
		ID:           g.GetId(),
		Name:         g.GetName(),
		Description:  g.Description,
		AvatarURL:    g.AvatarUrl,
		Slug:         g.GetSlug(),
		MembersCount: g.GetMembersCount(),
		IsMember:     g.GetIsMember(),
		Status:       toStatus(g.GetStatus()),
		AvatarShape:  toAvatarShape(g.GetAvatarShape()),
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
