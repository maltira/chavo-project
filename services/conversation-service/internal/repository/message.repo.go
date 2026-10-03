package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
)

// MessageRecord — строка messages; содержимое зашифровано.
type MessageRecord struct {
	ID               uuid.UUID
	ConversationID   uuid.UUID
	SenderID         uuid.UUID
	ContentEnc       []byte
	ReplyToMessageID *uuid.UUID
	IsEdited         bool
	CreatedAt        time.Time
	UpdatedAt        *time.Time
	DeletedAt        *time.Time
}

type MessageRepository interface {
	Create(ctx context.Context, q DBTX, rec *MessageRecord) error
	FindByID(ctx context.Context, q DBTX, id uuid.UUID) (*MessageRecord, error)
	List(ctx context.Context, q DBTX, convID uuid.UUID, before *uuid.UUID, limit int) ([]MessageRecord, error)
	Update(ctx context.Context, q DBTX, id uuid.UUID, contentEnc []byte) (*time.Time, error)
	SoftDelete(ctx context.Context, q DBTX, id uuid.UUID) (bool, error)
	SetReadCursor(ctx context.Context, q DBTX, convID, userID, messageID uuid.UUID) (bool, error)
	ListReaders(ctx context.Context, q DBTX, convID, messageID uuid.UUID) ([]uuid.UUID, error)
}

type messageRepository struct{}

func NewMessageRepository() MessageRepository {
	return &messageRepository{}
}

const messageColumns = `id, conversation_id, sender_id, content_enc, reply_to_message_id, is_edited, created_at, updated_at, deleted_at`

func scanMessage(row pgx.Row) (*MessageRecord, error) {
	var m MessageRecord
	if err := row.Scan(
		&m.ID, &m.ConversationID, &m.SenderID, &m.ContentEnc, &m.ReplyToMessageID,
		&m.IsEdited, &m.CreatedAt, &m.UpdatedAt, &m.DeletedAt,
	); err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *messageRepository) Create(ctx context.Context, q DBTX, rec *MessageRecord) error {
	// clock_timestamp(), а не NOW(): порядок сообщений должен отражать момент вставки, а не начало транзакции.
	err := q.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, content_enc, reply_to_message_id, created_at)
		 VALUES ($1, $2, $3, $4, clock_timestamp())
		 RETURNING id, created_at`,
		rec.ConversationID, rec.SenderID, rec.ContentEnc, rec.ReplyToMessageID,
	).Scan(&rec.ID, &rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
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

// List возвращает сообщения от новых к старым; before — id сообщения-курсора (строго старше него).
func (r *messageRepository) List(ctx context.Context, q DBTX, convID uuid.UUID, before *uuid.UUID, limit int) ([]MessageRecord, error) {
	rows, err := q.Query(ctx,
		`WITH cur AS (SELECT created_at, id FROM messages WHERE id = $2 AND conversation_id = $1)
		 SELECT `+messageColumns+`
		 FROM messages m
		 WHERE m.conversation_id = $1
		   AND ($2::uuid IS NULL OR EXISTS (
		        SELECT 1 FROM cur WHERE (m.created_at, m.id) < (cur.created_at, cur.id)))
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT $3`,
		convID, before, limit,
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

// Update меняет содержимое неудалённого сообщения; ErrMessageDeleted, если оно удалено.
func (r *messageRepository) Update(ctx context.Context, q DBTX, id uuid.UUID, contentEnc []byte) (*time.Time, error) {
	var updatedAt time.Time
	err := q.QueryRow(ctx,
		`UPDATE messages SET content_enc = $2, is_edited = TRUE, updated_at = clock_timestamp()
		 WHERE id = $1 AND deleted_at IS NULL
		 RETURNING updated_at`,
		id, contentEnc,
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
		`UPDATE messages SET content_enc = NULL, deleted_at = clock_timestamp()
		 WHERE id = $1 AND deleted_at IS NULL`, id,
	)
	if err != nil {
		return false, fmt.Errorf("delete message: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

// SetReadCursor двигает курсор прочтения только вперёд; false — курсор не изменился.
func (r *messageRepository) SetReadCursor(ctx context.Context, q DBTX, convID, userID, messageID uuid.UUID) (bool, error) {
	ct, err := q.Exec(ctx,
		`UPDATE conversation_members cm SET last_read_message_id = $3
		 FROM messages nm
		 WHERE nm.id = $3 AND nm.conversation_id = $1
		   AND cm.conversation_id = $1 AND cm.user_id = $2
		   AND (cm.last_read_message_id IS NULL OR NOT EXISTS (
		        SELECT 1 FROM messages om
		        WHERE om.id = cm.last_read_message_id
		          AND (om.created_at, om.id) >= (nm.created_at, nm.id)))`,
		convID, userID, messageID,
	)
	if err != nil {
		return false, fmt.Errorf("set read cursor: %w", err)
	}
	return ct.RowsAffected() == 1, nil
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
