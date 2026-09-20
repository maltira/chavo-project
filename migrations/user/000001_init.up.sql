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
    user_id         UUID PRIMARY KEY,
    system_language VARCHAR(10) NOT NULL DEFAULT 'en',
    theme           VARCHAR(20) NOT NULL DEFAULT 'system',
    
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),    
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_user_settings_user_id ON user_settings(user_id);

CREATE TABLE IF NOT EXISTS user_blocks (
    user_id UUID NOT NULL,    
    blocked_user_id UUID NOT NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, blocked_user_id)
);
CREATE INDEX idx_user_blocks_blocked ON user_blocks(blocked_user_id);
