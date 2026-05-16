CREATE TABLE IF NOT EXISTS users (
    id                  UUID            PRIMARY KEY     DEFAULT gen_random_uuid(),
    email               VARCHAR(254)    NOT NULL,
    password            VARCHAR(254)    NOT NULL,
    is_verified         BOOLEAN         NOT NULL        DEFAULT FALSE,

    deleted_at          TIMESTAMPTZ,
    deletion_reason     TEXT,
    deleted_by          VARCHAR(50)                     CHECK (deleted_by IN ('user', 'system')),

    created_at          TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),
    password_updated_at TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),
    email_updated_at    TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),

    CONSTRAINT valid_email CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
    CONSTRAINT password_not_empty CHECK (LENGTH(password) > 0),
    CONSTRAINT deletion_reason_check CHECK (
        (deleted_at IS NULL AND deletion_reason IS NULL AND deleted_by IS NULL) OR
        (deleted_at IS NOT NULL AND deletion_reason IS NOT NULL AND deleted_by IS NOT NULL)
    )
);
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_users_verified ON users(is_verified, deleted_at) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          UUID            PRIMARY KEY     DEFAULT gen_random_uuid(),
    user_id     UUID            NOT NULL        REFERENCES users(id) ON DELETE CASCADE,
    token       TEXT            NOT NULL,
    access_jti  VARCHAR(64)     NOT NULL,

    ip          VARCHAR(45)     NOT NULL,
    user_agent  VARCHAR(254)    NOT NULL,
    device      VARCHAR(100)    NOT NULL,

    created_at  TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),
    expires_at  TIMESTAMPTZ     NOT NULL,

    CONSTRAINT expires_at_future CHECK (expires_at > created_at),
);
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);

CREATE TABLE IF NOT EXISTS otp_codes (
    id          UUID            PRIMARY KEY     DEFAULT gen_random_uuid(),
    user_id     UUID            NOT NULL        REFERENCES users(id) ON DELETE CASCADE,
    code        VARCHAR(6)      NOT NULL        CHECK (code ~ '^[0-9]{6}$'),,
    code_type   VARCHAR(50)     NOT NULL        CHECK (code_type IN ('login', 'password_change', 'email_change', 'account_delete'))
    is_used     BOOLEAN         NOT NULL        DEFAULT FALSE,

    created_at  TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),
    expires_at  TIMESTAMPTZ     NOT NULL,

    CONSTRAINT expires_at_future CHECK (expires_at > created_at),
);
CREATE INDEX idx_otp_codes_user_id ON otp_codes(user_id);
CREATE INDEX idx_otp_codes_code ON otp_codes(code);