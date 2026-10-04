CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    conversation_type VARCHAR(20) NOT NULL
        CHECK (conversation_type IN ('direct', 'group')),

    visibility VARCHAR(20) NOT NULL DEFAULT 'private'
        CHECK (visibility IN ('private', 'public')),

    name VARCHAR(100),
    description VARCHAR(255),
    avatar_url TEXT,

    -- "<меньший_uuid>:<больший_uuid>" для direct, NULL для group.
    -- UNIQUE гарантирует максимум один direct между парой пользователей.
    direct_key TEXT UNIQUE,

    invite_token_hash TEXT UNIQUE,

    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT conversations_direct_key_check CHECK (
        (conversation_type = 'direct' AND direct_key IS NOT NULL)
        OR (conversation_type = 'group' AND direct_key IS NULL)
    )
);
CREATE INDEX idx_conversations_name_trgm ON conversations USING GIN (name gin_trgm_ops);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    conversation_id UUID NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    sender_id UUID NOT NULL,

    -- AES-256-GCM: [nonce 12B][ciphertext][tag 16B]. NULL после удаления сообщения.
    content_enc BYTEA,

    reply_to_message_id UUID
        REFERENCES messages(id)
        ON DELETE SET NULL,

    is_edited BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT messages_content_check CHECK (deleted_at IS NOT NULL OR content_enc IS NOT NULL)
);
CREATE INDEX idx_messages_conversation_cursor ON messages(conversation_id, created_at DESC, id DESC);
CREATE INDEX idx_messages_reply_to ON messages(reply_to_message_id);

CREATE TABLE conversation_members (
    conversation_id UUID NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL,

    member_role VARCHAR(20) NOT NULL DEFAULT 'member'
        CHECK (member_role IN ('member', 'admin')),

    -- Курсор прочтения: все сообщения до этого включительно прочитаны.
    last_read_message_id UUID
        REFERENCES messages(id)
        ON DELETE SET NULL,

    -- История видна участнику с этого момента (admin видит всё); clock_timestamp() согласован с messages.created_at.
    joined_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX idx_conversation_members_user ON conversation_members(user_id);

-- Чёрный список группы: забаненный не может вступить ни в открытую группу, ни подать заявку в закрытую.
CREATE TABLE conversation_bans (
    conversation_id UUID NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL,
    banned_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),

    PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE conversation_join_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    conversation_id UUID NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL,

    request_status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (request_status IN ('pending', 'approved', 'rejected')),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_join_requests_pending ON conversation_join_requests(conversation_id, user_id) WHERE request_status = 'pending';
CREATE INDEX idx_join_requests_conversation_status ON conversation_join_requests(conversation_id, request_status);

-- Transactional outbox: события пишутся в одной транзакции с изменением данных,
-- затем публикуются в Kafka отдельным воркером.
CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    topic VARCHAR(100) NOT NULL,
    event_key TEXT NOT NULL,
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);
CREATE INDEX idx_outbox_unpublished ON outbox_events(id) WHERE published_at IS NULL;
CREATE INDEX idx_outbox_message_id ON outbox_events ((payload->'payload'->>'message_id')) WHERE topic = 'message-events';
