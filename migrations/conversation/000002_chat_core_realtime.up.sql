ALTER TABLE conversation_members
    ADD COLUMN IF NOT EXISTS hidden_before TIMESTAMPTZ;

-- существующие «удалённые только у меня» личные чаты остаются скрытыми из списка до нового сообщения
UPDATE conversation_members
SET hidden_before = cleared_at
WHERE cleared_at IS NOT NULL AND hidden_before IS NULL;

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS is_silent BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS message_reactions (
    message_id UUID NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    emoji VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp (),
    PRIMARY KEY (message_id, user_id, emoji)
);

CREATE INDEX IF NOT EXISTS idx_message_reactions_message
    ON message_reactions (message_id, created_at ASC);
