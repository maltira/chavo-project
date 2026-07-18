CREATE TABLE IF NOT EXISTS chats (
    id          UUID            PRIMARY KEY     DEFAULT gen_random_uuid(),
    chat_type   VARCHAR(20)     NOT NULL        DEFAULT 'private' CHECK(chat_type IN ('private', 'group')),
    chat_name   VARCHAR(75)                     DEFAULT NULL,
    avatar_url  VARCHAR(254)                    DEFAULT NULL,
    can_join    BOOLEAN,
    created_by  UUID            NOT NULL,

    created_at  TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),
    updated_at  TIMESTAMPTZ     NOT NULL        DEFAULT NOW(),

    CONSTRAINT group_name_required CHECK (
        (chat_type = 'private' AND chat_name IS NULL) OR
        (chat_type = 'group' AND chat_name IS NOT NULL)
    ),
    CONSTRAINT created_by_not_empty CHECK (created_by != '00000000-0000-0000-0000-000000000000')
);
CREATE INDEX idx_chats_type ON chats(chat_type);
CREATE INDEX idx_chats_created_by ON chats(created_by);
CREATE INDEX idx_chats_created_at ON chats(created_at);

CREATE TABLE IF NOT EXISTS participants (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id     UUID        NOT NULL    REFERENCES chats(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL,
    role        VARCHAR(20) NOT NULL    DEFAULT 'member' CHECK (role IN ('member', 'admin', 'owner')),

    joined_at   TIMESTAMPTZ NOT NULL    DEFAULT NOW(),
    muted_until TIMESTAMPTZ,

    created_at  TIMESTAMPTZ NOT NULL    DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL    DEFAULT NOW(),

    CONSTRAINT unique_chat_user UNIQUE (chat_id, user_id),
    CONSTRAINT owner_role_check CHECK (
        (role = 'owner' AND muted_until IS NULL) OR
        (role != 'owner')
    )
);
CREATE INDEX idx_participants_chat_id ON participants(chat_id);
CREATE INDEX idx_participants_user_id ON participants(user_id);
CREATE INDEX idx_participants_muted ON participants(muted_until) WHERE muted_until IS NOT NULL;

CREATE TABLE IF NOT EXISTS messages (
    id                  UUID        PRIMARY KEY     DEFAULT gen_random_uuid(),
    chat_id             UUID        NOT NULL        REFERENCES chats(id) ON DELETE CASCADE,
    user_id             UUID,  -- NULL = системное сообщение
    content             TEXT        NOT NULL        CHECK (LENGTH(content) > 0),
    type                VARCHAR(20) NOT NULL        DEFAULT 'text' CHECK (type IN ('text', 'image', 'video', 'file', 'system')),
    reply_to_message    UUID,

    created_at          TIMESTAMPTZ NOT NULL        DEFAULT NOW(),
    updated_at          TIMESTAMPTZ,

    CONSTRAINT reply_to_self_check CHECK (reply_to_message != id),
    CONSTRAINT user_id_system CHECK (
        (type != 'system' AND user_id IS NOT NULL) OR
        (type = 'system' AND user_id IS NULL)
    )
);
CREATE INDEX idx_messages_chat_id ON messages(chat_id);
CREATE INDEX idx_messages_user_id ON messages(user_id);
CREATE INDEX idx_messages_created_at ON messages(created_at DESC);
CREATE INDEX idx_messages_chat_created ON messages(chat_id, created_at DESC);
CREATE INDEX idx_messages_reply_to ON messages(reply_to_message);
CREATE INDEX idx_messages_type ON messages(type);

CREATE TABLE IF NOT EXISTS message_reads (
    id          UUID        PRIMARY KEY     DEFAULT gen_random_uuid(),
    message_id  UUID        NOT NULL        REFERENCES messages(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL,
    read_at     TIMESTAMPTZ NOT NULL        DEFAULT NOW(),

    CONSTRAINT unique_message_read UNIQUE (message_id, user_id)
);
CREATE INDEX idx_message_reads_message_id ON message_reads(message_id);
CREATE INDEX idx_message_reads_user_id ON message_reads(user_id);

-- Триггер для updated_at
CREATE OR REPLACE FUNCTION chat_update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_chats_updated_at BEFORE UPDATE ON chats
    FOR EACH ROW EXECUTE FUNCTION chat_update_updated_at_column();
CREATE TRIGGER trigger_participants_updated_at BEFORE UPDATE ON participants
    FOR EACH ROW EXECUTE FUNCTION chat_update_updated_at_column();
CREATE TRIGGER trigger_messages_updated_at BEFORE UPDATE ON messages
    FOR EACH ROW EXECUTE FUNCTION chat_update_updated_at_column();