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
    
    invite_token_hash TEXT UNIQUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_conversations_name_trgm ON conversations USING GIN (name gin_trgm_ops);
CREATE UNIQUE INDEX idx_conversations_invite_token ON conversations(invite_token_hash) WHERE invite_token_hash IS NOT NULL;

CREATE TABLE conversation_members (
    conversation_id UUID NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL,
    
    member_role VARCHAR(20) NOT NULL DEFAULT 'member'
        CHECK (member_role IN ('member', 'admin')),

    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (conversation_id, user_id)
);
CREATE INDEX idx_conversation_members_user
    ON conversation_members(user_id);
    
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

CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    conversation_id UUID NOT NULL
        REFERENCES conversations(id)
        ON DELETE CASCADE,

    sender_id UUID NOT NULL,

    content TEXT NOT NULL,
    
    reply_to_message_id UUID
        REFERENCES messages(id)
        ON DELETE SET NULL,

    is_edited BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ
);
CREATE INDEX idx_messages_conversation_created ON messages(conversation_id, created_at DESC);
CREATE INDEX idx_messages_reply_to ON messages(reply_to_message_id);

CREATE TABLE message_receipts (
    message_id UUID NOT NULL
        REFERENCES messages(id)
        ON DELETE CASCADE,

    user_id UUID NOT NULL,
    read_at TIMESTAMPTZ,

    PRIMARY KEY (message_id, user_id)
);
