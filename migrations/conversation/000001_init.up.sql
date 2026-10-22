CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    conversation_type VARCHAR(20) NOT NULL CHECK (conversation_type IN ('direct', 'group')),
    visibility VARCHAR(20) NOT NULL DEFAULT 'private' CHECK (visibility IN ('private', 'public')),
    name VARCHAR(100),
    description VARCHAR(255),
    avatar_url TEXT,
    -- "<меньший_uuid>:<больший_uuid>" для direct, NULL для group.
    -- UNIQUE гарантирует максимум один direct между парой пользователей.
    direct_key TEXT UNIQUE,
    -- ссылка на чат: у открытой группы - код, заданный админом ([a-zA-Z0-9_], уникален без учёта регистра);
    -- у закрытой - сгенерированный код: SHA-256 для поиска и AES-256-GCM, чтобы админы могли его увидеть
    public_slug TEXT,
    invite_token_hash TEXT UNIQUE,
    invite_token_enc BYTEA,
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
    CONSTRAINT conversations_direct_key_check CHECK (
        (
            conversation_type = 'direct'
            AND direct_key IS NOT NULL
        )
        OR (
            conversation_type = 'group'
            AND direct_key IS NULL
        )
    ),
    CONSTRAINT conversations_invite_check CHECK (
        (
            conversation_type = 'direct'
            AND public_slug IS NULL
            AND invite_token_hash IS NULL
            AND invite_token_enc IS NULL
        )
        OR (
            conversation_type = 'group'
            AND visibility = 'public'
            AND public_slug IS NOT NULL
            AND invite_token_hash IS NULL
            AND invite_token_enc IS NULL
        )
        OR (
            conversation_type = 'group'
            AND visibility = 'private'
            AND public_slug IS NULL
            AND invite_token_hash IS NOT NULL
            AND invite_token_enc IS NOT NULL
        )
    )
);

CREATE UNIQUE INDEX idx_conversations_public_slug ON conversations (LOWER(public_slug));

CREATE INDEX idx_conversations_name_trgm ON conversations USING GIN (name gin_trgm_ops);

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    sender_id UUID NOT NULL,
    -- AES-256-GCM: [nonce 12B][ciphertext][tag 16B]. NULL после удаления сообщения и у системных.
    content_enc BYTEA,
    -- system - событие чата («вступил», «покинул», «исключил», «закрепил»): текста нет, его собирает клиент
    -- по system_event, sender_id (кто совершил) и target_id (кого касается); у pinned вместо target_id
    -- reply_to_message_id - закреплённое сообщение (NULL, если его строку удалили)
    kind VARCHAR(10) NOT NULL DEFAULT 'user' CHECK (kind IN ('user', 'system')),
    system_event VARCHAR(20),
    target_id UUID,
    reply_to_message_id UUID REFERENCES messages (id) ON DELETE SET NULL,
    is_edited BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT messages_content_check CHECK (
        deleted_at IS NOT NULL
        OR content_enc IS NOT NULL
        OR kind = 'system'
    ),
    CONSTRAINT messages_system_check CHECK (
        (
            kind = 'user'
            AND system_event IS NULL
            AND target_id IS NULL
        )
        OR (
            kind = 'system'
            AND system_event IN ('joined', 'added', 'left', 'removed')
            AND target_id IS NOT NULL
            AND content_enc IS NULL
            AND reply_to_message_id IS NULL
        )
        OR (
            kind = 'system'
            AND system_event = 'pinned'
            AND target_id IS NULL
            AND content_enc IS NULL
        )
    )
);

CREATE INDEX idx_messages_conversation_cursor ON messages (conversation_id, created_at DESC, id DESC);

CREATE INDEX idx_messages_reply_to ON messages (reply_to_message_id);

CREATE TABLE conversation_members (
    conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    member_role VARCHAR(20) NOT NULL DEFAULT 'member' CHECK (member_role IN ('member', 'admin')),
    -- Курсор прочтения: все сообщения до этого включительно прочитаны.
    last_read_message_id UUID REFERENCES messages (id) ON DELETE SET NULL,
    -- История видна участнику с этого момента (admin видит всё); clock_timestamp() согласован с messages.created_at.
    joined_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp (),
    -- Личные настройки чата участника: без звука до (навсегда = 9999-12-31), в архиве с, закреплён с.
    muted_until TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    pinned_at TIMESTAMPTZ,
    -- Личный чат удалён «только у меня»: участник не видит сообщений до этого момента (включительно).
    cleared_at TIMESTAMPTZ,
    -- Закреплённый чат не бывает в архиве.
    CONSTRAINT conversation_members_pin_archive_check CHECK (pinned_at IS NULL OR archived_at IS NULL),
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX idx_conversation_members_user ON conversation_members (user_id);

-- Закреплённые сообщения: owner_id NULL - закреп для всех участников, иначе виден только владельцу
-- (так можно только в личном чате). Удаление сообщения или чата убирает закреп каскадом.
CREATE TABLE pinned_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    message_id UUID NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    owner_id UUID,
    pinned_by UUID NOT NULL,
    pinned_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp ()
);

CREATE UNIQUE INDEX idx_pinned_messages_shared ON pinned_messages (message_id) WHERE owner_id IS NULL;
CREATE UNIQUE INDEX idx_pinned_messages_own ON pinned_messages (message_id, owner_id) WHERE owner_id IS NOT NULL;
CREATE INDEX idx_pinned_messages_conversation ON pinned_messages (conversation_id);

-- Чёрный список группы: забаненный не может вступить ни в открытую группу, ни подать заявку в закрытую.
CREATE TABLE conversation_bans (
    conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    banned_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp (),
    PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE conversation_join_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    request_status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (
        request_status IN ('pending', 'approved', 'rejected')
    ),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
);

CREATE UNIQUE INDEX idx_join_requests_pending ON conversation_join_requests (conversation_id, user_id)
WHERE
    request_status = 'pending';

CREATE INDEX idx_join_requests_conversation_status ON conversation_join_requests (conversation_id, request_status);

-- Transactional outbox: события пишутся в одной транзакции с изменением данных,
-- затем публикуются в Kafka отдельным воркером.
CREATE TABLE outbox_events (
    id BIGSERIAL PRIMARY KEY,
    topic VARCHAR(100) NOT NULL,
    event_key TEXT NOT NULL,
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW (),
    published_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_unpublished ON outbox_events (id)
WHERE
    published_at IS NULL
    AND failed_at IS NULL;

CREATE INDEX idx_outbox_message_id ON outbox_events ((payload->'payload'->>'message_id'))
WHERE
    topic = 'message-events';

CREATE TABLE conversation_statuses (
    conversation_id UUID PRIMARY KEY REFERENCES conversations (id) ON DELETE CASCADE,
    icon VARCHAR(32) NOT NULL,
    text VARCHAR(70) NOT NULL,
    background CHAR(7) NOT NULL,
    foreground CHAR(7) NOT NULL,
    expires_at TIMESTAMPTZ,
    set_by UUID NOT NULL,
    set_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
);

CREATE TABLE conversation_avatar_shapes (
    conversation_id UUID PRIMARY KEY REFERENCES conversations (id) ON DELETE CASCADE,
    shape VARCHAR(32) NOT NULL,
    color CHAR(7) NOT NULL,
    set_by UUID NOT NULL,
    set_at TIMESTAMPTZ NOT NULL DEFAULT NOW ()
);
