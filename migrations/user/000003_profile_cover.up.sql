CREATE TABLE IF NOT EXISTS user_profile_covers (
    user_id UUID PRIMARY KEY REFERENCES profiles(user_id) ON DELETE CASCADE,
    color   CHAR(7)     NOT NULL,
    pattern VARCHAR(32) NOT NULL DEFAULT '',
    set_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
