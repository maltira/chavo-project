package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

// MessageRecord — строка messages; содержимое зашифровано.
type MessageRecord struct {
	ID               uuid.UUID
	ConversationID   uuid.UUID
	SenderID         uuid.UUID
	ContentEnc       []byte
	ReplyToMessageID *uuid.UUID
	IsEdited         bool
	// IsForwarded - пересланная копия; ForwardedFromUserID - первый автор (nil - скрыл имя)
	IsForwarded         bool
	ForwardedFromUserID *uuid.UUID
	// Entities - форматирование текста (nil - обычный текст)
	Entities []models.Entity
	// IsSilent - отправлено без звука
	IsSilent bool
	// LinkPreview - превью ссылки (OpenGraph)
	LinkPreview *models.LinkPreview
	// Kind - user или system; у системного нет текста, есть SystemEvent и TargetID
	Kind        string
	SystemEvent *string
	TargetID    *uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   *time.Time
	DeletedAt   *time.Time
}

type MessageRepository interface {
	Create(ctx context.Context, q DBTX, rec *MessageRecord) error
	CreateSystem(ctx context.Context, q DBTX, rec *MessageRecord) error
	FindByID(ctx context.Context, q DBTX, id uuid.UUID) (*MessageRecord, error)
	// LockByID блокирует строку сообщения (FOR UPDATE) на время транзакции
	LockByID(ctx context.Context, q DBTX, id uuid.UUID) (*MessageRecord, error)
	// List отдаёт только сообщения, видимые участнику userID (от новых к старым).
	List(ctx context.Context, q DBTX, convID, userID uuid.UUID, before *uuid.UUID, limit int) ([]MessageRecord, error)
	// ListAfter отдаёт сообщения строго новее курсора after (от старых к новым - ближайшие к курсору первыми)
	ListAfter(ctx context.Context, q DBTX, convID, userID, after uuid.UUID, limit int) ([]MessageRecord, error)
	// FindVisible — сообщение, которое участник вправе видеть (ErrNotFound иначе).
	FindVisible(ctx context.Context, q DBTX, id, userID uuid.UUID) (*MessageRecord, error)
	// FindForViewer — как FindVisible, но также отдаёт сообщение, на которое отвечает видимое участнику сообщение.
	FindForViewer(ctx context.Context, q DBTX, id, userID uuid.UUID) (*MessageRecord, error)
	Update(ctx context.Context, q DBTX, id uuid.UUID, contentEnc []byte, entities []models.Entity, linkPreview *models.LinkPreview) (*time.Time, error)
	SoftDelete(ctx context.Context, q DBTX, id uuid.UUID) (bool, error)
	SetReadCursor(ctx context.Context, q DBTX, convID, userID, messageID uuid.UUID) (moved bool, prev *uuid.UUID, err error)
	// AuthorsBetween возвращает авторов неудалённых сообщений в (after, upTo], кроме exclude; after == nil — с начала чата.
	AuthorsBetween(ctx context.Context, q DBTX, convID uuid.UUID, after *uuid.UUID, upTo, exclude uuid.UUID) ([]uuid.UUID, error)
	ListReaders(ctx context.Context, q DBTX, convID, messageID uuid.UUID) ([]uuid.UUID, error)
	// ReplaceMentions переписывает список упомянутых в сообщении пользователей
	ReplaceMentions(ctx context.Context, q DBTX, msgID, convID uuid.UUID, createdAt time.Time, userIDs []uuid.UUID) error
	// DeleteMentionsByMessage удаляет индекс упоминаний при удалении сообщения
	DeleteMentionsByMessage(ctx context.Context, q DBTX, msgID uuid.UUID) error
	// ListUnreadMentions возвращает id непрочитанных сообщений, где упомянут userID (от старых к новым)
	ListUnreadMentions(ctx context.Context, q DBTX, convID, userID uuid.UUID, limit int) ([]uuid.UUID, error)
}

type messageRepository struct{}

func NewMessageRepository() MessageRepository {
	return &messageRepository{}
}

// Участник видит сообщения с момента вступления, admin - всю историю; в обоих случаях - только после
// своей очистки личного чата (cleared_at): в direct оба участника admin, поэтому условие через AND.
const visibleToMember = `((cm.member_role = 'admin' OR m.created_at >= cm.joined_at)
	AND (cm.cleared_at IS NULL OR m.created_at > cm.cleared_at))`

const messageColumnsM = `m.id, m.conversation_id, m.sender_id, m.content_enc, m.reply_to_message_id, m.is_edited,
	m.kind, m.system_event, m.target_id, m.created_at, m.updated_at, m.deleted_at,
	m.is_forwarded, m.forwarded_from_user_id, m.entities, m.is_silent, m.link_preview`

const messageColumns = `id, conversation_id, sender_id, content_enc, reply_to_message_id, is_edited,
	kind, system_event, target_id, created_at, updated_at, deleted_at,
	is_forwarded, forwarded_from_user_id, entities, is_silent, link_preview`

// messageDest - поля для Scan в порядке messageColumns
func messageDest(m *MessageRecord) []any {
	return []any{&m.ID, &m.ConversationID, &m.SenderID, &m.ContentEnc, &m.ReplyToMessageID,
		&m.IsEdited, &m.Kind, &m.SystemEvent, &m.TargetID, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt,
		&m.IsForwarded, &m.ForwardedFromUserID, &m.Entities, &m.IsSilent, &m.LinkPreview}
}

func scanMessage(row pgx.Row) (*MessageRecord, error) {
	var m MessageRecord
	if err := row.Scan(messageDest(&m)...); err != nil {
		return nil, err
	}
	return &m, nil
}

// entitiesArg: без форматирования в колонку пишется NULL, а не пустой массив
func entitiesArg(entities []models.Entity) any {
	if len(entities) == 0 {
		return nil
	}
	return entities
}

func linkPreviewArg(lp *models.LinkPreview) any {
	if lp == nil {
		return nil
	}
	return lp
}

func (r *messageRepository) Create(ctx context.Context, q DBTX, rec *MessageRecord) error {
	// clock_timestamp(), а не NOW(): порядок сообщений должен отражать момент вставки, а не начало транзакции.
	err := q.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, content_enc, reply_to_message_id,
		                       is_forwarded, forwarded_from_user_id, entities, is_silent, link_preview, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, clock_timestamp())
		 RETURNING id, created_at`,
		rec.ConversationID, rec.SenderID, rec.ContentEnc, rec.ReplyToMessageID,
		rec.IsForwarded, rec.ForwardedFromUserID, entitiesArg(rec.Entities), rec.IsSilent, linkPreviewArg(rec.LinkPreview),
	).Scan(&rec.ID, &rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	return nil
}

// CreateSystem вставляет системное сообщение (rec.Kind и текст не нужны); у pinned задан ReplyToMessageID
func (r *messageRepository) CreateSystem(ctx context.Context, q DBTX, rec *MessageRecord) error {
	rec.Kind = models.MessageSystem
	err := q.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, kind, system_event, target_id, reply_to_message_id, created_at)
		 VALUES ($1, $2, 'system', $3, $4, $5, clock_timestamp())
		 RETURNING id, created_at`,
		rec.ConversationID, rec.SenderID, rec.SystemEvent, rec.TargetID, rec.ReplyToMessageID,
	).Scan(&rec.ID, &rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("create system message: %w", err)
	}
	return nil
}

func (r *messageRepository) FindByID(ctx context.Context, q DBTX, id uuid.UUID) (*MessageRecord, error) {
	m, err := scanMessage(q.QueryRow(ctx, `SELECT `+messageColumns+` FROM messages WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find message: %w", err)
	}
	return m, nil
}

func (r *messageRepository) LockByID(ctx context.Context, q DBTX, id uuid.UUID) (*MessageRecord, error) {
	m, err := scanMessage(q.QueryRow(ctx, `SELECT `+messageColumns+` FROM messages WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("lock message: %w", err)
	}
	return m, nil
}

// List возвращает сообщения от новых к старым; before — id сообщения-курсора (строго старше него).
func (r *messageRepository) List(ctx context.Context, q DBTX, convID, userID uuid.UUID, before *uuid.UUID, limit int) ([]MessageRecord, error) {
	rows, err := q.Query(ctx,
		`WITH cur AS (SELECT created_at, id FROM messages WHERE id = $2 AND conversation_id = $1)
		 SELECT `+messageColumnsM+`
		 FROM messages m
		 JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = $4
		 WHERE m.conversation_id = $1
		   AND `+visibleToMember+`
		   AND ($2::uuid IS NULL OR EXISTS (
		        SELECT 1 FROM cur WHERE (m.created_at, m.id) < (cur.created_at, cur.id)))
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT $3`,
		convID, before, limit, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	res := make([]MessageRecord, 0, limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		res = append(res, *m)
	}
	return res, rows.Err()
}

// ListAfter возвращает сообщения строго новее курсора after (от старых к новым)
func (r *messageRepository) ListAfter(ctx context.Context, q DBTX, convID, userID, after uuid.UUID, limit int) ([]MessageRecord, error) {
	rows, err := q.Query(ctx,
		`WITH cur AS (SELECT created_at, id FROM messages WHERE id = $2 AND conversation_id = $1)
		 SELECT `+messageColumnsM+`
		 FROM messages m
		 JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = $4
		 WHERE m.conversation_id = $1
		   AND `+visibleToMember+`
		   AND EXISTS (
		        SELECT 1 FROM cur WHERE (m.created_at, m.id) > (cur.created_at, cur.id))
		 ORDER BY m.created_at ASC, m.id ASC
		 LIMIT $3`,
		convID, after, limit, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages after: %w", err)
	}
	defer rows.Close()

	res := make([]MessageRecord, 0, limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		res = append(res, *m)
	}
	return res, rows.Err()
}

func (r *messageRepository) FindVisible(ctx context.Context, q DBTX, id, userID uuid.UUID) (*MessageRecord, error) {
	m, err := scanMessage(q.QueryRow(ctx,
		`SELECT `+messageColumnsM+`
		 FROM messages m
		 JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = $2
		 WHERE m.id = $1 AND `+visibleToMember, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find visible message: %w", err)
	}
	return m, nil
}

func (r *messageRepository) FindForViewer(ctx context.Context, q DBTX, id, userID uuid.UUID) (*MessageRecord, error) {
	m, err := scanMessage(q.QueryRow(ctx,
		`SELECT `+messageColumnsM+`
		 FROM messages m
		 JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = $2
		 WHERE m.id = $1
		   AND (`+visibleToMember+` OR EXISTS (
		        SELECT 1 FROM messages r
		        WHERE r.reply_to_message_id = m.id AND r.conversation_id = m.conversation_id
		          AND (cm.member_role = 'admin' OR r.created_at >= cm.joined_at)
		          AND (cm.cleared_at IS NULL OR r.created_at > cm.cleared_at)))`, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrNotFound
		}
		return nil, fmt.Errorf("find message for viewer: %w", err)
	}
	return m, nil
}

// Update меняет содержимое неудалённого сообщения; ErrMessageDeleted, если оно удалено.
func (r *messageRepository) Update(ctx context.Context, q DBTX, id uuid.UUID, contentEnc []byte, entities []models.Entity, linkPreview *models.LinkPreview) (*time.Time, error) {
	var updatedAt time.Time
	err := q.QueryRow(ctx,
		`UPDATE messages SET content_enc = $2, entities = $3, link_preview = $4, is_edited = TRUE, updated_at = clock_timestamp()
		 WHERE id = $1 AND deleted_at IS NULL AND kind = 'user'
		 RETURNING updated_at`,
		id, contentEnc, entitiesArg(entities), linkPreviewArg(linkPreview),
	).Scan(&updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.ErrMessageDeleted
		}
		return nil, fmt.Errorf("update message: %w", err)
	}
	return &updatedAt, nil
}

// SoftDelete возвращает true, если сообщение было удалено именно этим вызовом.
func (r *messageRepository) SoftDelete(ctx context.Context, q DBTX, id uuid.UUID) (bool, error) {
	ct, err := q.Exec(ctx,
		`UPDATE messages SET content_enc = NULL, entities = NULL, link_preview = NULL, deleted_at = clock_timestamp()
		 WHERE id = $1 AND deleted_at IS NULL AND kind = 'user'`, id,
	)
	if err != nil {
		return false, fmt.Errorf("delete message: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

// SetReadCursor двигает курсор прочтения только вперёд; moved=false — курсор не изменился.
// prev — прежнее значение курсора (до обновления).
func (r *messageRepository) SetReadCursor(ctx context.Context, q DBTX, convID, userID, messageID uuid.UUID) (bool, *uuid.UUID, error) {
	var prev *uuid.UUID
	err := q.QueryRow(ctx,
		`UPDATE conversation_members cm SET last_read_message_id = $3
		 FROM messages nm, conversation_members old
		 WHERE nm.id = $3 AND nm.conversation_id = $1
		   AND cm.conversation_id = $1 AND cm.user_id = $2
		   AND old.conversation_id = cm.conversation_id AND old.user_id = cm.user_id
		   AND (cm.last_read_message_id IS NULL OR NOT EXISTS (
		        SELECT 1 FROM messages om
		        WHERE om.id = cm.last_read_message_id
		          AND (om.created_at, om.id) >= (nm.created_at, nm.id)))
		 RETURNING old.last_read_message_id`,
		convID, userID, messageID,
	).Scan(&prev)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, fmt.Errorf("set read cursor: %w", err)
	}
	return true, prev, nil
}

func (r *messageRepository) AuthorsBetween(ctx context.Context, q DBTX, convID uuid.UUID, after *uuid.UUID, upTo, exclude uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx,
		`SELECT DISTINCT m.sender_id
		 FROM messages m
		 JOIN messages hi ON hi.id = $3
		 LEFT JOIN messages lo ON lo.id = $2
		 WHERE m.conversation_id = $1
		   AND m.deleted_at IS NULL
		   AND m.kind = 'user'
		   AND m.sender_id <> $4
		   AND (m.created_at, m.id) <= (hi.created_at, hi.id)
		   AND (lo.id IS NULL OR (m.created_at, m.id) > (lo.created_at, lo.id))
		 ORDER BY m.sender_id`,
		convID, after, upTo, exclude,
	)
	if err != nil {
		return nil, fmt.Errorf("authors between: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan author: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListReaders возвращает участников (кроме автора), прочитавших сообщение включительно.
func (r *messageRepository) ListReaders(ctx context.Context, q DBTX, convID, messageID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx,
		`SELECT cm.user_id
		 FROM conversation_members cm
		 JOIN messages tm ON tm.id = $2 AND tm.conversation_id = $1
		 JOIN messages rm ON rm.id = cm.last_read_message_id
		 WHERE cm.conversation_id = $1
		   AND cm.user_id <> tm.sender_id
		   AND (rm.created_at, rm.id) >= (tm.created_at, tm.id)
		 ORDER BY cm.user_id`,
		convID, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("list readers: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan reader: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *messageRepository) ReplaceMentions(ctx context.Context, q DBTX, msgID, convID uuid.UUID, createdAt time.Time, userIDs []uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM message_mentions WHERE message_id = $1`, msgID); err != nil {
		return fmt.Errorf("clear message mentions: %w", err)
	}
	if len(userIDs) == 0 {
		return nil
	}
	for _, uid := range userIDs {
		if _, err := q.Exec(ctx,
			`INSERT INTO message_mentions (message_id, conversation_id, user_id, created_at)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (message_id, user_id) DO NOTHING`,
			msgID, convID, uid, createdAt,
		); err != nil {
			return fmt.Errorf("insert message mention: %w", err)
		}
	}
	return nil
}

func (r *messageRepository) DeleteMentionsByMessage(ctx context.Context, q DBTX, msgID uuid.UUID) error {
	if _, err := q.Exec(ctx, `DELETE FROM message_mentions WHERE message_id = $1`, msgID); err != nil {
		return fmt.Errorf("delete message mentions: %w", err)
	}
	return nil
}

func (r *messageRepository) ListUnreadMentions(ctx context.Context, q DBTX, convID, userID uuid.UUID, limit int) ([]uuid.UUID, error) {
	rows, err := q.Query(ctx,
		`SELECT mm.message_id
		 FROM message_mentions mm
		 JOIN messages m ON m.id = mm.message_id
		 JOIN conversation_members cm ON cm.conversation_id = mm.conversation_id AND cm.user_id = $2
		 LEFT JOIN messages rm ON rm.id = cm.last_read_message_id
		 WHERE mm.conversation_id = $1
		   AND mm.user_id = $2
		   AND m.deleted_at IS NULL
		   AND m.sender_id <> $2
		   AND m.created_at >= cm.joined_at
		   AND (cm.cleared_at IS NULL OR m.created_at > cm.cleared_at)
		   AND (rm.id IS NULL OR (m.created_at, m.id) > (rm.created_at, rm.id))
		 ORDER BY mm.created_at ASC, mm.message_id ASC
		 LIMIT $3`,
		convID, userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list unread mentions: %w", err)
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan unread mention: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
