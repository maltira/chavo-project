DROP TABLE IF EXISTS backup_codes;

DROP INDEX IF EXISTS idx_users_email_active;
CREATE INDEX idx_users_email_active ON users(email) WHERE deleted_at IS NULL;

-- удаляем мягко удалённые дубликаты email, оставшиеся после повторной регистрации,
-- чтобы восстановление полного UNIQUE-ограничения не падало
DELETE FROM users u
WHERE u.deleted_at IS NOT NULL
  AND EXISTS (
      SELECT 1 FROM users newer
      WHERE newer.email = u.email
        AND newer.id <> u.id
        AND (
            newer.deleted_at IS NULL
            OR newer.created_at > u.created_at
            OR (newer.created_at = u.created_at AND newer.id > u.id)
        )
  );

ALTER TABLE users
    DROP COLUMN IF EXISTS totp_verified_at,
    DROP COLUMN IF EXISTS totp_secret_enc,
    DROP COLUMN IF EXISTS two_factor_method,
    ADD CONSTRAINT users_email_key UNIQUE (email);
