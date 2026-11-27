package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BackupCodeRepository interface {
	ReplaceAllTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, codeHashes []string) error
	DeleteByUserIDTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error
	Consume(ctx context.Context, userID uuid.UUID, codeHash string) (bool, error)
	CountRemaining(ctx context.Context, userID uuid.UUID) (int, error)
}

type backupCodeRepository struct {
	pool *pgxpool.Pool
}

func NewBackupCodeRepository(pool *pgxpool.Pool) BackupCodeRepository {
	return &backupCodeRepository{pool: pool}
}

func (r *backupCodeRepository) ReplaceAllTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, codeHashes []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM backup_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for _, hash := range codeHashes {
		if _, err := tx.Exec(ctx, `INSERT INTO backup_codes (user_id, code_hash) VALUES ($1, $2)`, userID, hash); err != nil {
			return err
		}
	}
	return nil
}

func (r *backupCodeRepository) DeleteByUserIDTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM backup_codes WHERE user_id = $1`, userID)
	return err
}

func (r *backupCodeRepository) Consume(ctx context.Context, userID uuid.UUID, codeHash string) (bool, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx,
		`UPDATE backup_codes
		 SET used_at = NOW()
		 WHERE id = (
			SELECT id FROM backup_codes
			WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id`,
		userID, codeHash,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *backupCodeRepository) CountRemaining(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM backup_codes WHERE user_id = $1 AND used_at IS NULL`,
		userID,
	).Scan(&n)
	return n, err
}
