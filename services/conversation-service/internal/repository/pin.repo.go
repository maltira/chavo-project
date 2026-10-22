package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// PinRecord - закреплённое сообщение; OwnerID nil - закреп для всех участников
type PinRecord struct {
	Message  MessageRecord
	OwnerID  *uuid.UUID
	PinnedBy uuid.UUID
	PinnedAt time.Time
}

type PinRepository interface {
	// Pin - false, если сообщение уже закреплено так же (для всех или этим владельцем)
	Pin(ctx context.Context, q DBTX, convID, messageID uuid.UUID, owner *uuid.UUID, by uuid.UUID) (bool, error)
	Unpin(ctx context.Context, q DBTX, messageID uuid.UUID, owner *uuid.UUID) (bool, error)
	// Count - закрепы для всех (owner nil) или личные закрепы владельца в чате
	Count(ctx context.Context, q DBTX, convID uuid.UUID, owner *uuid.UUID) (int, error)
	// List - закрепы, которые видит участник: общие и его личные, только видимые ему и не удалённые сообщения
	List(ctx context.Context, q DBTX, convID, viewer uuid.UUID) ([]PinRecord, error)
	// MakePersonal превращает общий закреп в личный закреп owner (если у него уже есть свой - общий просто
	// снимается); false - общего закрепа не было
	MakePersonal(ctx context.Context, q DBTX, messageID, owner uuid.UUID) (bool, error)
	// DeleteForMessage снимает все закрепы сообщения; возвращает их владельцев (nil - общий)
	DeleteForMessage(ctx context.Context, q DBTX, messageID uuid.UUID) ([]*uuid.UUID, error)
}

type pinRepository struct{}

func NewPinRepository() PinRepository {
	return &pinRepository{}
}

func (r *pinRepository) Pin(ctx context.Context, q DBTX, convID, messageID uuid.UUID, owner *uuid.UUID, by uuid.UUID) (bool, error) {
	ct, err := q.Exec(ctx,
		`INSERT INTO pinned_messages (conversation_id, message_id, owner_id, pinned_by)
		 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, convID, messageID, owner, by)
	if err != nil {
		return false, fmt.Errorf("pin message: %w", err)
	}
	return ct.RowsAffected() == 1, nil
}

func (r *pinRepository) Unpin(ctx context.Context, q DBTX, messageID uuid.UUID, owner *uuid.UUID) (bool, error) {
	ct, err := q.Exec(ctx,
		`DELETE FROM pinned_messages WHERE message_id = $1 AND owner_id IS NOT DISTINCT FROM $2`, messageID, owner)
	if err != nil {
		return false, fmt.Errorf("unpin message: %w", err)
	}
	return ct.RowsAffected() > 0, nil
}

func (r *pinRepository) Count(ctx context.Context, q DBTX, convID uuid.UUID, owner *uuid.UUID) (int, error) {
	var n int
	if err := q.QueryRow(ctx,
		`SELECT COUNT(*) FROM pinned_messages WHERE conversation_id = $1 AND owner_id IS NOT DISTINCT FROM $2`,
		convID, owner).Scan(&n); err != nil {
		return 0, fmt.Errorf("count pins: %w", err)
	}
	return n, nil
}

func (r *pinRepository) List(ctx context.Context, q DBTX, convID, viewer uuid.UUID) ([]PinRecord, error) {
	rows, err := q.Query(ctx,
		`SELECT `+messageColumnsM+`, p.owner_id, p.pinned_by, p.pinned_at
		 FROM pinned_messages p
		 JOIN messages m ON m.id = p.message_id
		 JOIN conversation_members cm ON cm.conversation_id = p.conversation_id AND cm.user_id = $2
		 WHERE p.conversation_id = $1 AND (p.owner_id IS NULL OR p.owner_id = $2)
		   AND m.deleted_at IS NULL AND `+visibleToMember+`
		 ORDER BY p.pinned_at DESC, p.id DESC`, convID, viewer)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}
	defer rows.Close()

	res := make([]PinRecord, 0)
	for rows.Next() {
		var p PinRecord
		if err := rows.Scan(append(messageDest(&p.Message), &p.OwnerID, &p.PinnedBy, &p.PinnedAt)...); err != nil {
			return nil, fmt.Errorf("scan pin: %w", err)
		}
		res = append(res, p)
	}
	return res, rows.Err()
}

func (r *pinRepository) MakePersonal(ctx context.Context, q DBTX, messageID, owner uuid.UUID) (bool, error) {
	var hasOwn bool
	if err := q.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pinned_messages WHERE message_id = $1 AND owner_id = $2)`,
		messageID, owner).Scan(&hasOwn); err != nil {
		return false, fmt.Errorf("find personal pin: %w", err)
	}
	var (
		ct  pgconn.CommandTag
		err error
	)
	if hasOwn {
		ct, err = q.Exec(ctx, `DELETE FROM pinned_messages WHERE message_id = $1 AND owner_id IS NULL`, messageID)
	} else {
		ct, err = q.Exec(ctx,
			`UPDATE pinned_messages SET owner_id = $2 WHERE message_id = $1 AND owner_id IS NULL`, messageID, owner)
	}
	if err != nil {
		return false, fmt.Errorf("make pin personal: %w", err)
	}
	return ct.RowsAffected() > 0, nil
}

func (r *pinRepository) DeleteForMessage(ctx context.Context, q DBTX, messageID uuid.UUID) ([]*uuid.UUID, error) {
	rows, err := q.Query(ctx, `DELETE FROM pinned_messages WHERE message_id = $1 RETURNING owner_id`, messageID)
	if err != nil {
		return nil, fmt.Errorf("delete message pins: %w", err)
	}
	defer rows.Close()

	owners := make([]*uuid.UUID, 0)
	for rows.Next() {
		var owner *uuid.UUID
		if err := rows.Scan(&owner); err != nil {
			return nil, fmt.Errorf("scan pin owner: %w", err)
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}
