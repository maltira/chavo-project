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
	// LastMessageRead - кто-то кроме автора последнего сообщения дочитал до него (курсор прочтения)
	LastMessageRead bool
	MembersCount    int
	Settings        models.ChatSettings
	// IsSaved - «Избранное»: direct с самим собой
	IsSaved bool
}

// conversationColumns и customJoins - чтение разговора со статусом и фигурой за аватаром; истёкший статус не читается
const (
	customColumns = `cs.icon, cs.text, cs.background, cs.foreground, cs.expires_at, sh.shape, sh.color`
	baseColumns   = `c.id, c.conversation_type, c.visibility, c.name, c.description, c.avatar_url,
		c.public_slug, c.invite_token_enc, c.last_message_at, c.created_at, c.updated_at`
	conversationColumns = baseColumns + `, ` + customColumns
	customJoins         = `LEFT JOIN conversation_statuses cs ON cs.conversation_id = c.id AND (cs.expires_at IS NULL OR cs.expires_at > NOW())
		 LEFT JOIN conversation_avatar_shapes sh ON sh.conversation_id = c.id`
)

// customCols - nullable-поля статуса и фигуры из customJoins
type customCols struct {
	icon, text, bg, fg *string
	expires            *time.Time
	shape, color       *string
}

// dest - поля для Scan в порядке customColumns
func (s *customCols) dest() []any {
	return []any{&s.icon, &s.text, &s.bg, &s.fg, &s.expires, &s.shape, &s.color}
}

func (s customCols) status() *models.Status {
	if s.icon == nil || s.text == nil || s.bg == nil || s.fg == nil {
		return nil
	}
	return &models.Status{Icon: *s.icon, Text: *s.text, Background: *s.bg, Foreground: *s.fg, ExpiresAt: s.expires}
}

func (s customCols) avatarShape() *models.AvatarShape {
	if s.shape == nil || s.color == nil {
		return nil
	}
	return &models.AvatarShape{Shape: *s.shape, Color: *s.color}
}

// baseDest - поля для Scan в порядке baseColumns
func baseDest(c *models.Conversation) []any {
	return []any{&c.ID, &c.ConversationType, &c.Visibility, &c.Name, &c.Description, &c.AvatarURL,
		&c.Slug, &c.InviteCodeEnc, &c.LastMessageAt, &c.CreatedAt, &c.UpdatedAt}
}

func scanConversation(row pgx.Row) (*models.Conversation, error) {
	var (
		c  models.Conversation
		cu customCols
	)
	if err := row.Scan(append(baseDest(&c), cu.dest()...)...); err != nil {
		return nil, err
	}
	c.Status = cu.status()
	c.AvatarShape = cu.avatarShape()
	return &c, nil
}

type ConversationRepository interface {
	// SetStatus ставит статус группы целиком (прежний заменяется); ClearStatus — false, если статуса не было.
	SetStatus(ctx context.Context, q DBTX, convID, setBy uuid.UUID, st models.Status) error
	ClearStatus(ctx context.Context, q DBTX, convID uuid.UUID) (bool, error)
	// SetAvatarShape ставит фигуру за аватаром группы целиком (прежняя заменяется)
	SetAvatarShape(ctx context.Context, q DBTX, convID, setBy uuid.UUID, sh models.AvatarShape) error
	ClearAvatarShape(ctx context.Context, q DBTX, convID uuid.UUID) error
	CreateDirectIfAbsent(ctx context.Context, q DBTX, directKey string) (id uuid.UUID, created bool, err error)
	AddMember(ctx context.Context, q DBTX, convID, userID uuid.UUID, role string) error
	Access(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*Access, error)
	MemberIDs(ctx context.Context, q DBTX, convID uuid.UUID) ([]uuid.UUID, error)
	// DirectPeers — собеседники пользователя по всем его direct-чатам.
	DirectPeers(ctx context.Context, q DBTX, userID uuid.UUID) ([]uuid.UUID, error)
	TouchLastMessage(ctx context.Context, q DBTX, convID uuid.UUID, at time.Time) error
	// ListSummaries: archived nil - без фильтра по архиву (нужно для одного чата по convID)
	ListSummaries(ctx context.Context, q DBTX, userID uuid.UUID, convID *uuid.UUID, archived *bool, limit, offset int) ([]SummaryRecord, error)
	// UpdateChatSettings меняет личные настройки участника (nil - NULL); ErrNotMember, если он не участник
	UpdateChatSettings(ctx context.Context, q DBTX, convID, userID uuid.UUID, updates map[string]any) (*models.ChatSettings, error)
	// ClearHistory - личный чат «удалён у меня»: история до этого момента скрыта, личные настройки сброшены
	ClearHistory(ctx context.Context, q DBTX, convID, userID uuid.UUID) error
	// LockChatSettings читает настройки участника с блокировкой строки; ErrNotMember, если он не участник
	LockChatSettings(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*models.ChatSettings, error)
	// CountPinnedChats блокирует строки участия пользователя до конца транзакции: лимит закрепов не обходится параллельными запросами
	CountPinnedChats(ctx context.Context, q DBTX, userID uuid.UUID) (int, error)

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
	// FindBySlug ищет открытую группу по коду ссылки без учёта регистра; ErrNotFound, если такой нет
	FindBySlug(ctx context.Context, q DBTX, slug string) (*models.Conversation, error)
	SearchPublicGroups(ctx context.Context, q DBTX, userID uuid.UUID, query string, limit, offset int) ([]models.PublicGroup, error)
}

type NewGroup struct {
	Name            string
	Description     *string
	AvatarURL       *string
	Visibility      string
	Slug            *string
	InviteTokenHash *string
	InviteTokenEnc  []byte
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

// ListSummaries возвращает чаты пользователя (или один, если задан convID): закреплённые сверху (новый закреп выше), дальше свежие.
func (r *conversationRepository) ListSummaries(ctx context.Context, q DBTX, userID uuid.UUID, convID *uuid.UUID, archived *bool, limit, offset int) ([]SummaryRecord, error) {
	rows, err := q.Query(ctx,
		`SELECT `+baseColumns+`,
		        `+customColumns+`,
		        cm.member_role, peer.user_id,
		        lm.id, lm.sender_id, lm.content_enc, lm.reply_to_message_id, lm.is_edited,
		        lm.kind, lm.system_event, lm.target_id, lm.created_at, lm.updated_at, lm.deleted_at,
		        lm.is_forwarded, lm.forwarded_from_user_id,
		        (SELECT COUNT(*) FROM messages m
		          WHERE m.conversation_id = c.id AND m.deleted_at IS NULL AND m.sender_id <> $1
		            AND m.kind = 'user'
		            AND m.created_at >= cm.joined_at
		            AND (cm.cleared_at IS NULL OR m.created_at > cm.cleared_at)
		            AND (rm.id IS NULL OR (m.created_at, m.id) > (rm.created_at, rm.id))),
		        COALESCE((SELECT TRUE FROM conversation_members om
		          JOIN messages orm ON orm.id = om.last_read_message_id
		          WHERE om.conversation_id = c.id AND om.user_id <> lm.sender_id
		            AND (orm.created_at, orm.id) >= (lm.created_at, lm.id)
		          LIMIT 1), FALSE),
		        (SELECT COUNT(*) FROM conversation_members mc WHERE mc.conversation_id = c.id),
		        cm.muted_until, cm.archived_at, cm.pinned_at,
		        (c.conversation_type = 'direct' AND peer.user_id IS NULL)
		 FROM conversation_members cm
		 JOIN conversations c ON c.id = cm.conversation_id
		 `+customJoins+`
		 LEFT JOIN messages rm ON rm.id = cm.last_read_message_id
		 LEFT JOIN conversation_members peer
		        ON c.conversation_type = 'direct' AND peer.conversation_id = c.id AND peer.user_id <> $1
		 LEFT JOIN LATERAL (
		        SELECT * FROM messages x WHERE x.conversation_id = c.id
		           AND (cm.member_role = 'admin' OR x.created_at >= cm.joined_at)
		           AND (cm.cleared_at IS NULL OR x.created_at > cm.cleared_at)
		        ORDER BY x.created_at DESC, x.id DESC LIMIT 1) lm ON TRUE
		 WHERE cm.user_id = $1 AND ($2::uuid IS NULL OR c.id = $2)
		   AND ($5::boolean IS NULL OR (cm.archived_at IS NOT NULL) = $5)
		   -- очищенный «только у меня» чат пропадает из списка до следующего сообщения; карточка (convID) доступна
		   AND ($2::uuid IS NOT NULL OR cm.cleared_at IS NULL OR c.last_message_at > cm.cleared_at)
		 ORDER BY cm.pinned_at DESC NULLS LAST, c.last_message_at DESC, c.id DESC
		 LIMIT $3 OFFSET $4`,
		userID, convID, limit, offset, archived,
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
			lmKind, lmEvent                 *string
			lmTarget                        *uuid.UUID
			lmCreated, lmUpdated, lmDeleted *time.Time
			lmForwarded                     *bool
			lmFrom                          *uuid.UUID
			cu                              customCols
		)
		dest := append(baseDest(&s.Conversation), cu.dest()...)
		dest = append(dest, &s.MyRole, &s.PeerID,
			&lmID, &lmSender, &lmEnc, &lmReply, &lmEdited, &lmKind, &lmEvent, &lmTarget,
			&lmCreated, &lmUpdated, &lmDeleted, &lmForwarded, &lmFrom,
			&s.UnreadCount, &s.LastMessageRead, &s.MembersCount,
			&s.Settings.MutedUntil, &s.Settings.ArchivedAt, &s.Settings.PinnedAt, &s.IsSaved)
		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		s.Conversation.Status = cu.status()
		s.Conversation.AvatarShape = cu.avatarShape()
		if lmID != nil {
			s.LastMessage = &MessageRecord{
				ID: *lmID, ConversationID: s.Conversation.ID, SenderID: *lmSender,
				ContentEnc: lmEnc, ReplyToMessageID: lmReply, IsEdited: *lmEdited,
				Kind: *lmKind, SystemEvent: lmEvent, TargetID: lmTarget,
				CreatedAt: *lmCreated, UpdatedAt: lmUpdated, DeletedAt: lmDeleted,
				IsForwarded: *lmForwarded, ForwardedFromUserID: lmFrom,
			}
		}
		res = append(res, s)
	}
	return res, rows.Err()
}

var allowedChatSettingsColumns = map[string]bool{"muted_until": true, "archived_at": true, "pinned_at": true}

func (r *conversationRepository) UpdateChatSettings(ctx context.Context, q DBTX, convID, userID uuid.UUID, updates map[string]any) (*models.ChatSettings, error) {
	setClauses := make([]string, 0, len(updates))
	args := []any{convID, userID}
	for col, val := range updates {
		if !allowedChatSettingsColumns[col] {
			return nil, fmt.Errorf("update chat settings: unknown column %q", col)
		}
		args = append(args, val)
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, len(args)))
	}
	if len(setClauses) == 0 {
		return nil, fmt.Errorf("update chat settings: nothing to update")
	}

	var cs models.ChatSettings
	err := q.QueryRow(ctx,
		fmt.Sprintf(`UPDATE conversation_members SET %s WHERE conversation_id = $1 AND user_id = $2
		 RETURNING muted_until, archived_at, pinned_at`, strings.Join(setClauses, ", ")), args...,
	).Scan(&cs.MutedUntil, &cs.ArchivedAt, &cs.PinnedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotMember
		}
		return nil, fmt.Errorf("update chat settings: %w", err)
	}
	return &cs, nil
}

func (r *conversationRepository) ClearHistory(ctx context.Context, q DBTX, convID, userID uuid.UUID) error {
	// clock_timestamp(), как у messages.created_at: граница точно отделяет уже отправленные сообщения
	ct, err := q.Exec(ctx,
		`UPDATE conversation_members
		 SET cleared_at = clock_timestamp(), muted_until = NULL, archived_at = NULL, pinned_at = NULL
		 WHERE conversation_id = $1 AND user_id = $2`, convID, userID)
	if err != nil {
		return fmt.Errorf("clear history: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return apperror.ErrNotMember
	}
	return nil
}

func (r *conversationRepository) LockChatSettings(ctx context.Context, q DBTX, convID, userID uuid.UUID) (*models.ChatSettings, error) {
	var cs models.ChatSettings
	err := q.QueryRow(ctx,
		`SELECT muted_until, archived_at, pinned_at FROM conversation_members
		 WHERE conversation_id = $1 AND user_id = $2 FOR UPDATE`, convID, userID,
	).Scan(&cs.MutedUntil, &cs.ArchivedAt, &cs.PinnedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotMember
		}
		return nil, fmt.Errorf("lock chat settings: %w", err)
	}
	return &cs, nil
}

func (r *conversationRepository) CountPinnedChats(ctx context.Context, q DBTX, userID uuid.UUID) (int, error) {
	// блокировка и подсчёт - разные запросы: подсчёт берёт новый снимок и видит закрепы, закоммиченные,
	// пока ждали блокировку (в одном запросе условие по pinned_at проверилось бы по старому снимку)
	if _, err := q.Exec(ctx, `SELECT 1 FROM conversation_members WHERE user_id = $1 FOR UPDATE`, userID); err != nil {
		return 0, fmt.Errorf("lock memberships: %w", err)
	}
	var n int
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM conversation_members WHERE user_id = $1 AND pinned_at IS NOT NULL`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pinned chats: %w", err)
	}
	return n, nil
}

func (r *conversationRepository) Lock(ctx context.Context, q DBTX, convID uuid.UUID) (*models.Conversation, error) {
	// OF c: строки статуса и фигуры - nullable-сторона LEFT JOIN, блокируется только разговор
	c, err := scanConversation(q.QueryRow(ctx,
		`SELECT `+conversationColumns+`
		 FROM conversations c `+customJoins+`
		 WHERE c.id = $1 FOR UPDATE OF c`, convID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("lock conversation: %w", err)
	}
	return c, nil
}

func (r *conversationRepository) CreateGroup(ctx context.Context, q DBTX, g NewGroup) (uuid.UUID, error) {
	var id uuid.UUID
	err := q.QueryRow(ctx,
		`INSERT INTO conversations (conversation_type, visibility, name, description, avatar_url,
		                            public_slug, invite_token_hash, invite_token_enc)
		 VALUES ('group', $1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		g.Visibility, g.Name, g.Description, g.AvatarURL, g.Slug, g.InviteTokenHash, g.InviteTokenEnc,
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err, "idx_conversations_public_slug") {
			return uuid.Nil, apperror.ErrSlugTaken
		}
		return uuid.Nil, fmt.Errorf("create group: %w", err)
	}
	return id, nil
}

var allowedConversationColumns = map[string]bool{
	"name": true, "description": true, "avatar_url": true, "visibility": true,
	"public_slug": true, "invite_token_hash": true, "invite_token_enc": true,
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
		if isUniqueViolation(err, "idx_conversations_public_slug") {
			return apperror.ErrSlugTaken
		}
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

// ListMembers: сначала админы, внутри - по времени вступления
func (r *conversationRepository) ListMembers(ctx context.Context, q DBTX, convID uuid.UUID, limit, offset int) ([]models.Member, error) {
	rows, err := q.Query(ctx,
		`SELECT conversation_id, user_id, member_role, last_read_message_id, joined_at
		 FROM conversation_members WHERE conversation_id = $1
		 ORDER BY member_role = 'admin' DESC, joined_at, user_id LIMIT $2 OFFSET $3`, convID, limit, offset)
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
	c, err := scanConversation(q.QueryRow(ctx,
		`SELECT `+conversationColumns+`
		 FROM conversations c `+customJoins+`
		 WHERE c.invite_token_hash = $1 AND c.conversation_type = 'group' AND c.visibility = 'private'`, hash))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find by invite hash: %w", err)
	}
	return c, nil
}

func (r *conversationRepository) FindBySlug(ctx context.Context, q DBTX, slug string) (*models.Conversation, error) {
	c, err := scanConversation(q.QueryRow(ctx,
		`SELECT `+conversationColumns+`
		 FROM conversations c `+customJoins+`
		 WHERE LOWER(c.public_slug) = LOWER($1) AND c.conversation_type = 'group' AND c.visibility = 'public'`, slug))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find by slug: %w", err)
	}
	return c, nil
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchPublicGroups ищет публичные группы по подстроке в названии (приватные не попадают в выдачу).
func (r *conversationRepository) SearchPublicGroups(ctx context.Context, q DBTX, userID uuid.UUID, query string, limit, offset int) ([]models.PublicGroup, error) {
	rows, err := q.Query(ctx,
		`SELECT c.id, c.name, c.description, c.avatar_url, c.public_slug,
		        (SELECT COUNT(*) FROM conversation_members m WHERE m.conversation_id = c.id),
		        EXISTS (SELECT 1 FROM conversation_members m WHERE m.conversation_id = c.id AND m.user_id = $1),
		        `+customColumns+`
		 FROM conversations c `+customJoins+`
		 WHERE c.conversation_type = 'group' AND c.visibility = 'public' AND (c.name ILIKE $2 OR c.public_slug ILIKE $2)
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
		var (
			g  models.PublicGroup
			cu customCols
		)
		if err := rows.Scan(append([]any{&g.ID, &g.Name, &g.Description, &g.AvatarURL, &g.Slug, &g.MembersCount, &g.IsMember},
			cu.dest()...)...); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		g.Status = cu.status()
		g.AvatarShape = cu.avatarShape()
		res = append(res, g)
	}
	return res, rows.Err()
}

func (r *conversationRepository) DirectPeers(ctx context.Context, q DBTX, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx,
		`SELECT peer.user_id
		 FROM conversation_members me
		 JOIN conversations c ON c.id = me.conversation_id AND c.conversation_type = 'direct'
		 JOIN conversation_members peer ON peer.conversation_id = c.id AND peer.user_id <> me.user_id
		 WHERE me.user_id = $1
		 ORDER BY peer.user_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("direct peers: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan direct peer: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *conversationRepository) SetStatus(ctx context.Context, q DBTX, convID, setBy uuid.UUID, st models.Status) error {
	if _, err := q.Exec(ctx,
		`INSERT INTO conversation_statuses (conversation_id, icon, text, background, foreground, expires_at, set_by, set_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		 ON CONFLICT (conversation_id) DO UPDATE
		   SET icon = EXCLUDED.icon, text = EXCLUDED.text, background = EXCLUDED.background,
		       foreground = EXCLUDED.foreground, expires_at = EXCLUDED.expires_at,
		       set_by = EXCLUDED.set_by, set_at = NOW()`,
		convID, st.Icon, st.Text, st.Background, st.Foreground, st.ExpiresAt, setBy,
	); err != nil {
		return fmt.Errorf("set group status: %w", err)
	}
	return nil
}

func (r *conversationRepository) ClearStatus(ctx context.Context, q DBTX, convID uuid.UUID) (bool, error) {
	ct, err := q.Exec(ctx, `DELETE FROM conversation_statuses WHERE conversation_id = $1`, convID)
	if err != nil {
		return false, fmt.Errorf("clear group status: %w", err)
	}
	return ct.RowsAffected() > 0, nil
}

func (r *conversationRepository) SetAvatarShape(ctx context.Context, q DBTX, convID, setBy uuid.UUID, sh models.AvatarShape) error {
	if _, err := q.Exec(ctx,
		`INSERT INTO conversation_avatar_shapes (conversation_id, shape, color, set_by, set_at)
		 VALUES ($1, $2, $3, $4, NOW())
		 ON CONFLICT (conversation_id) DO UPDATE
		   SET shape = EXCLUDED.shape, color = EXCLUDED.color, set_by = EXCLUDED.set_by, set_at = NOW()`,
		convID, sh.Shape, sh.Color, setBy,
	); err != nil {
		return fmt.Errorf("set group avatar shape: %w", err)
	}
	return nil
}

func (r *conversationRepository) ClearAvatarShape(ctx context.Context, q DBTX, convID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM conversation_avatar_shapes WHERE conversation_id = $1`, convID); err != nil {
		return fmt.Errorf("clear group avatar shape: %w", err)
	}
	return nil
}
