-- обязательная 2FA (email или TOTP + резервные коды) и освобождение email при удалении аккаунта
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_email_key,
    ADD COLUMN IF NOT EXISTS two_factor_method VARCHAR(16) NOT NULL DEFAULT 'email'
        CHECK (two_factor_method IN ('email', 'totp')),
    ADD COLUMN IF NOT EXISTS totp_secret_enc BYTEA,
    ADD COLUMN IF NOT EXISTS totp_verified_at TIMESTAMPTZ;

DROP INDEX IF EXISTS idx_users_email_active;
CREATE UNIQUE INDEX idx_users_email_active ON users(email) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS backup_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_backup_codes_user ON backup_codes(user_id) WHERE used_at IS NULL;
