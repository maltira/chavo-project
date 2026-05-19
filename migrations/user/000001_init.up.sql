CREATE TABLE IF NOT EXISTS profiles (
    id          UUID            PRIMARY KEY,
    username    VARCHAR(16)     NOT NULL    UNIQUE,
    full_name   VARCHAR(100)    NOT NULL,

    bio         VARCHAR(500),
    avatar_url  VARCHAR(254),
    birth_date  DATE,

    last_seen   TIMESTAMPTZ     NOT NULL    DEFAULT NOW(),

    created_at  TIMESTAMPTZ     NOT NULL    DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL    DEFAULT NOW(),

    CONSTRAINT username_format CHECK (username ~ '^[a-zA-Z0-9_]+$'),
    CONSTRAINT username_length CHECK (LENGTH(username) >= 3),
    CONSTRAINT full_name_not_empty CHECK (LENGTH(full_name) > 0),
    CONSTRAINT birth_date_future CHECK (birth_date <= CURRENT_DATE)
);
CREATE INDEX idx_profiles_username ON profiles(username);
CREATE INDEX idx_profiles_last_seen ON profiles(last_seen);
CREATE INDEX idx_profiles_username_last_seen ON profiles(username, last_seen);

CREATE TABLE IF NOT EXISTS settings (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id          UUID        NOT NULL    UNIQUE REFERENCES profiles(id) ON DELETE CASCADE,

    show_online_status  BOOLEAN     NOT NULL    DEFAULT TRUE,
    show_birth_date     BOOLEAN     NOT NULL    DEFAULT TRUE,

    created_at          TIMESTAMPTZ NOT NULL    DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL    DEFAULT NOW()
);
CREATE INDEX idx_settings_profile_id ON settings(profile_id);

CREATE TABLE IF NOT EXISTS blocks (
    id                  UUID            PRIMARY KEY     DEFAULT gen_random_uuid(),
    profile_id          UUID            NOT NULL        REFERENCES profiles(id) ON DELETE CASCADE,
    blocked_profile_id  UUID            NOT NULL        REFERENCES profiles(id) ON DELETE CASCADE,
    created_at          TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),

    CONSTRAINT block_pair_unique UNIQUE (profile_id, blocked_profile_id),
    CONSTRAINT no_self_block CHECK (profile_id != blocked_profile_id)
);
CREATE INDEX idx_blocks_profile_id ON blocks(profile_id);
CREATE INDEX idx_blocks_blocked_profile_id ON blocks(blocked_profile_id);

-- Триггер для updated_at
CREATE OR REPLACE FUNCTION user_update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_profiles_updated_at BEFORE UPDATE ON profiles
    FOR EACH ROW EXECUTE FUNCTION user_update_updated_at_column();

CREATE TRIGGER trigger_settings_updated_at BEFORE UPDATE ON settings
    FOR EACH ROW EXECUTE FUNCTION user_update_updated_at_column();