package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

// Access — положение пользователя в conversation.
type Access struct {
	Type   string
	Role   string
	PeerID *uuid.UUID // собеседник в direct
}

// SummaryRecord — строка списка чатов; LastMessage зашифрован.
type SummaryRecord struct {
	Conversation models.Conversation
	MyRole       string
	PeerID       *uuid.UUID
	LastMessage  *MessageRecord
	UnreadCount  int
}

type ConversationRepository interface {
	CreateDirectIfAbsent(ctx context.Context, q DBTX, directKey string) (id uuid.UUID, created bool, err error)
	AddMember(ctx context.Context, q DBTX, convID, userID uuid.UUID, role string) error
	Access(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*Access, error)
	MemberIDs(ctx context.Context, q DBTX, convID uuid.UUID) ([]uuid.UUID, error)
	TouchLastMessage(ctx context.Context, q DBTX, convID uuid.UUID, at time.Time) error
	ListSummaries(ctx context.Context, q DBTX, userID uuid.UUID, convID *uuid.UUID, limit, offset int) ([]SummaryRecord, error)

	// Lock блокирует строку conversation (FOR UPDATE) и возвращает её: сериализует изменения состава и настроек.
	Lock(ctx context.Context, q DBTX, convID uuid.UUID) (*models.Conversation, error)
	CreateGroup(ctx context.Context, q DBTX, g NewGroup) (uuid.UUID, error)
	UpdateGroup(ctx context.Context, q DBTX, convID uuid.UUID, updates map[string]any) error
	Delete(ctx context.Context, q DBTX, convID uuid.UUID) error
	RemoveMember(ctx context.Context, q DBTX, convID, userID uuid.UUID) (bool, error)
	SetMemberRole(ctx context.Context, q DBTX, convID, userID uuid.UUID, role string) (bool, error)
	MemberRole(ctx context.Context, q DBTX, convID, userID uuid.UUID) (string, error)
	CountAdmins(ctx context.Context, q DBTX, convID uuid.UUID) (int, error)
	ListMembers(ctx context.Context, q DBTX, convID uuid.UUID, limit, offset int) ([]models.Member, error)
	CountMembers(ctx context.Context, q DBTX, convID uuid.UUID) (int, error)
	AdminIDs(ctx context.Context, q DBTX, convID uuid.UUID) ([]uuid.UUID, error)
	FindByInviteHash(ctx context.Context, q DBTX, hash string) (*models.Conversation, error)
	SearchPublicGroups(ctx context.Context, q DBTX, userID uuid.UUID, query string, limit, offset int) ([]models.PublicGroup, error)
}

type NewGroup struct {
	Name            string
	Description     *string
	AvatarURL       *string
	Visibility      string
	InviteTokenHash *string
}

type conversationRepository struct{}

func NewConversationRepository() ConversationRepository {
	return &conversationRepository{}
}

// CreateDirectIfAbsent атомарно создаёт direct по ключу пары. При гонке второй INSERT ждёт
// коммита первого и ничего не вставляет, после чего возвращается id существующего direct.
func (r *conversationRepository) CreateDirectIfAbsent(ctx context.Context, q DBTX, directKey string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx,
		`INSERT INTO conversations (conversation_type, direct_key)
		 VALUES ('direct', $1)
		 ON CONFLICT (direct_key) DO NOTHING
		 RETURNING id`, directKey,
	).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, fmt.Errorf("create direct: %w", err)
	}

	if err = q.QueryRow(ctx, `SELECT id FROM conversations WHERE direct_key = $1`, directKey).Scan(&id); err != nil {
		return uuid.Nil, false, fmt.Errorf("find direct: %w", err)
	}
	return id, false, nil
}

func (r *conversationRepository) AddMember(ctx context.Context, q DBTX, convID, userID uuid.UUID, role string) error {
	if _, err := q.Exec(ctx,
		`INSERT INTO conversation_members (conversation_id, user_id, member_role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (conversation_id, user_id) DO NOTHING`,
		convID, userID, role,
	); err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	return nil
}

// Access возвращает ErrNotFound, если conversation нет, и ErrNotMember, если пользователь не участник.
func (r *conversationRepository) Access(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*Access, error) {
	var (
		a    Access
		role *string
	)
	err := q.QueryRow(ctx,
		`SELECT c.conversation_type, cm.member_role,
		        CASE WHEN c.conversation_type = 'direct' THEN
		            (SELECT p.user_id FROM conversation_members p
		             WHERE p.conversation_id = c.id AND p.user_id <> $2 LIMIT 1)
		        END
		 FROM conversations c
		 LEFT JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = $2
		 WHERE c.id = $1`,
		convID, userID,
	).Scan(&a.Type, &role, &a.PeerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("conversation access: %w", err)
	}
	if role == nil {
		return nil, apperror.ErrNotMember
	}
	a.Role = *role
	return &a, nil
}

func (r *conversationRepository) MemberIDs(ctx context.Context, q DBTX, convID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx,
		`SELECT user_id FROM conversation_members WHERE conversation_id = $1 ORDER BY user_id`, convID)
	if err != nil {
		return nil, fmt.Errorf("list member ids: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan member id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *conversationRepository) TouchLastMessage(ctx context.Context, q DBTX, convID uuid.UUID, at time.Time) error {
	if _, err := q.Exec(ctx,
		`UPDATE conversations SET last_message_at = $2 WHERE id = $1 AND last_message_at < $2`,
		convID, at,
	); err != nil {
		return fmt.Errorf("touch last message: %w", err)
	}
	return nil
}

// ListSummaries возвращает чаты пользователя (или один, если задан convID), свежие сверху.
func (r *conversationRepository) ListSummaries(ctx context.Context, q DBTX, userID uuid.UUID, convID *uuid.UUID, limit, offset int) ([]SummaryRecord, error) {
	rows, err := q.Query(ctx,
		`SELECT c.id, c.conversation_type, c.visibility, c.name, c.description, c.avatar_url,
		        c.last_message_at, c.created_at, c.updated_at,
		        cm.member_role, peer.user_id,
		        lm.id, lm.sender_id, lm.content_enc, lm.reply_to_message_id, lm.is_edited,
		        lm.created_at, lm.updated_at, lm.deleted_at,
		        (SELECT COUNT(*) FROM messages m
		          WHERE m.conversation_id = c.id AND m.deleted_at IS NULL AND m.sender_id <> $1
		            AND (rm.id IS NULL OR (m.created_at, m.id) > (rm.created_at, rm.id)))
		 FROM conversation_members cm
		 JOIN conversations c ON c.id = cm.conversation_id
		 LEFT JOIN messages rm ON rm.id = cm.last_read_message_id
		 LEFT JOIN conversation_members peer
		        ON c.conversation_type = 'direct' AND peer.conversation_id = c.id AND peer.user_id <> $1
		 LEFT JOIN LATERAL (
		        SELECT * FROM messages x WHERE x.conversation_id = c.id
		        ORDER BY x.created_at DESC, x.id DESC LIMIT 1) lm ON TRUE
		 WHERE cm.user_id = $1 AND ($2::uuid IS NULL OR c.id = $2)
		 ORDER BY c.last_message_at DESC, c.id DESC
		 LIMIT $3 OFFSET $4`,
		userID, convID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	res := make([]SummaryRecord, 0)
	for rows.Next() {
		var (
			s                               SummaryRecord
			lmID, lmSender, lmReply         *uuid.UUID
			lmEnc                           []byte
			lmEdited                        *bool
			lmCreated, lmUpdated, lmDeleted *time.Time
		)
		if err := rows.Scan(
			&s.Conversation.ID, &s.Conversation.ConversationType, &s.Conversation.Visibility,
			&s.Conversation.Name, &s.Conversation.Description, &s.Conversation.AvatarURL,
			&s.Conversation.LastMessageAt, &s.Conversation.CreatedAt, &s.Conversation.UpdatedAt,
			&s.MyRole, &s.PeerID,
			&lmID, &lmSender, &lmEnc, &lmReply, &lmEdited, &lmCreated, &lmUpdated, &lmDeleted,
			&s.UnreadCount,
		); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		if lmID != nil {
			s.LastMessage = &MessageRecord{
				ID: *lmID, ConversationID: s.Conversation.ID, SenderID: *lmSender,
				ContentEnc: lmEnc, ReplyToMessageID: lmReply, IsEdited: *lmEdited,
				CreatedAt: *lmCreated, UpdatedAt: lmUpdated, DeletedAt: lmDeleted,
			}
		}
		res = append(res, s)
	}
	return res, rows.Err()
}

func (r *conversationRepository) Lock(ctx context.Context, q DBTX, convID uuid.UUID) (*models.Conversation, error) {
	var c models.Conversation
	err := q.QueryRow(ctx,
		`SELECT id, conversation_type, visibility, name, description, avatar_url, last_message_at, created_at, updated_at
		 FROM conversations WHERE id = $1 FOR UPDATE`, convID,
	).Scan(&c.ID, &c.ConversationType, &c.Visibility, &c.Name, &c.Description, &c.AvatarURL,
		&c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("lock conversation: %w", err)
	}
	return &c, nil
}

func (r *conversationRepository) CreateGroup(ctx context.Context, q DBTX, g NewGroup) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx,
		`INSERT INTO conversations (conversation_type, visibility, name, description, avatar_url, invite_token_hash)
		 VALUES ('group', $1, $2, $3, $4, $5) RETURNING id`,
		g.Visibility, g.Name, g.Description, g.AvatarURL, g.InviteTokenHash,
	).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create group: %w", err)
	}
	return id, nil
}

var allowedConversationColumns = map[string]bool{
	"name": true, "description": true, "avatar_url": true, "visibility": true, "invite_token_hash": true,
}

// UpdateGroup обновляет перечисленные колонки; значение nil записывается как NULL.
func (r *conversationRepository) UpdateGroup(ctx context.Context, q DBTX, convID uuid.UUID, updates map[string]any) error {
	setClauses := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+2)
	for col, val := range updates {
		if !allowedConversationColumns[col] {
			return fmt.Errorf("update group: unknown column %q", col)
		}
		args = append(args, val)
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	setClauses = append(setClauses, "updated_at = clock_timestamp()")
	args = append(args, convID)

	ct, err := q.Exec(ctx,
		fmt.Sprintf("UPDATE conversations SET %s WHERE id = $%d", strings.Join(setClauses, ", "), len(args)), args...)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotFound
	}
	return nil
}

func (r *conversationRepository) Delete(ctx context.Context, q DBTX, convID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, convID); err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

func (r *conversationRepository) RemoveMember(ctx context.Context, q DBTX, convID, userID uuid.UUID) (bool, error) {
	ct, err := q.Exec(ctx,
		`DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2`, convID, userID)
	if err != nil {
		return false, fmt.Errorf("remove member: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

func (r *conversationRepository) SetMemberRole(ctx context.Context, q DBTX, convID, userID uuid.UUID, role string) (bool, error) {
	ct, err := q.Exec(ctx,
		`UPDATE conversation_members SET member_role = $3 WHERE conversation_id = $1 AND user_id = $2`,
		convID, userID, role)
	if err != nil {
		return false, fmt.Errorf("set member role: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

// MemberRole возвращает ErrNotFound, если пользователь не участник.
func (r *conversationRepository) MemberRole(ctx context.Context, q DBTX, convID, userID uuid.UUID) (string, error) {
	var role string
	err := q.QueryRow(ctx,
		`SELECT member_role FROM conversation_members WHERE conversation_id = $1 AND user_id = $2`,
		convID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperror.ErrNotFound
		}
		return "", fmt.Errorf("member role: %w", err)
	}
	return role, nil
}

func (r *conversationRepository) CountAdmins(ctx context.Context, q DBTX, convID uuid.UUID) (int, error) {
	var n int
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM conversation_members WHERE conversation_id = $1 AND member_role = 'admin'`,
		convID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return n, nil
}

func (r *conversationRepository) ListMembers(ctx context.Context, q DBTX, convID uuid.UUID, limit, offset int) ([]models.Member, error) {
	rows, err := q.Query(ctx,
		`SELECT conversation_id, user_id, member_role, last_read_message_id, joined_at
		 FROM conversation_members WHERE conversation_id = $1
		 ORDER BY joined_at, user_id LIMIT $2 OFFSET $3`, convID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	res := make([]models.Member, 0)
	for rows.Next() {
		var m models.Member
		if err := rows.Scan(&m.ConversationID, &m.UserID, &m.Role, &m.LastReadMessageID, &m.JoinedAt); err != nil {
			return nil, fmt.Errorf("scan member: %w", err)
		}
		res = append(res, m)
	}
	return res, rows.Err()
}

func (r *conversationRepository) CountMembers(ctx context.Context, q DBTX, convID uuid.UUID) (int, error) {
	var n int
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM conversation_members WHERE conversation_id = $1`, convID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count members: %w", err)
	}
	return n, nil
}

func (r *conversationRepository) AdminIDs(ctx context.Context, q DBTX, convID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx,
		`SELECT user_id FROM conversation_members WHERE conversation_id = $1 AND member_role = 'admin' ORDER BY user_id`, convID)
	if err != nil {
		return nil, fmt.Errorf("list admin ids: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan admin id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// FindByInviteHash ищет приватную группу по хэшу invite-токена; ErrNotFound, если такой нет.
func (r *conversationRepository) FindByInviteHash(ctx context.Context, q DBTX, hash string) (*models.Conversation, error) {
	var c models.Conversation
	err := q.QueryRow(ctx,
		`SELECT id, conversation_type, visibility, name, description, avatar_url, last_message_at, created_at, updated_at
		 FROM conversations
		 WHERE invite_token_hash = $1 AND conversation_type = 'group' AND visibility = 'private'`, hash,
	).Scan(&c.ID, &c.ConversationType, &c.Visibility, &c.Name, &c.Description, &c.AvatarURL,
		&c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find by invite hash: %w", err)
	}
	return &c, nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchPublicGroups ищет публичные группы по подстроке в названии (приватные не попадают в выдачу).
func (r *conversationRepository) SearchPublicGroups(ctx context.Context, q DBTX, userID uuid.UUID, query string, limit, offset int) ([]models.PublicGroup, error) {
	rows, err := q.Query(ctx,
		`SELECT c.id, c.name, c.description, c.avatar_url,
		        (SELECT COUNT(*) FROM conversation_members m WHERE m.conversation_id = c.id),
		        EXISTS (SELECT 1 FROM conversation_members m WHERE m.conversation_id = c.id AND m.user_id = $1)
		 FROM conversations c
		 WHERE c.conversation_type = 'group' AND c.visibility = 'public' AND c.name ILIKE $2
		 ORDER BY c.name, c.id
		 LIMIT $3 OFFSET $4`,
		userID, "%"+likeEscaper.Replace(query)+"%", limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("search groups: %w", err)
	}
	defer rows.Close()

	res := make([]models.PublicGroup, 0)
	for rows.Next() {
		var g models.PublicGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.AvatarURL, &g.MembersCount, &g.IsMember); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		res = append(res, g)
	}
	return res, rows.Err()
}
