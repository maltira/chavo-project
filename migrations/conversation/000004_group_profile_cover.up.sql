CREATE TABLE IF NOT EXISTS conversation_profile_covers (
    conversation_id UUID PRIMARY KEY REFERENCES conversations (id) ON DELETE CASCADE,
    color           CHAR(7)     NOT NULL,
    pattern         VARCHAR(32) NOT NULL DEFAULT '',
    set_by          UUID        NOT NULL,
    set_at          TIMESTAMPTZ NOT NULL DEFAULT NOW ()
);
