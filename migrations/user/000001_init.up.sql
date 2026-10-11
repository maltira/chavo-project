CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS profiles (
    user_id         UUID PRIMARY KEY,
    username        VARCHAR(32) NOT NULL UNIQUE,    
    display_name    VARCHAR(100) NOT NULL,   
    bio             VARCHAR(255),
    avatar_url      TEXT,

    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    created_at  TIMESTAMPTZ     NOT NULL    DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL    DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT username_format CHECK (username ~ '^[a-zA-Z0-9_]+$'),
    CONSTRAINT username_length CHECK (LENGTH(username) >= 3),
    CONSTRAINT display_name_not_empty CHECK (LENGTH(display_name) > 0)
);
CREATE INDEX idx_profiles_username_trgm ON profiles USING GIN (username gin_trgm_ops);

CREATE TABLE IF NOT EXISTS user_settings (
    user_id             UUID PRIMARY KEY,
    allow_group_invites BOOLEAN NOT NULL DEFAULT TRUE,
    show_online_status  BOOLEAN NOT NULL DEFAULT TRUE,
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),    
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_blocks (
    user_id UUID NOT NULL,    
    blocked_user_id UUID NOT NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, blocked_user_id)
);
CREATE INDEX idx_user_blocks_blocked ON user_blocks(blocked_user_id);

CREATE TABLE IF NOT EXISTS user_statuses (
    user_id    UUID PRIMARY KEY REFERENCES profiles(user_id) ON DELETE CASCADE,
    icon       VARCHAR(32) NOT NULL,
    text       VARCHAR(70) NOT NULL,
    background CHAR(7)     NOT NULL,
    foreground CHAR(7)     NOT NULL,
    expires_at TIMESTAMPTZ,
    set_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
