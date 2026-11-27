DROP INDEX IF EXISTS idx_message_reactions_message;
DROP TABLE IF EXISTS message_reactions;

ALTER TABLE messages
    DROP COLUMN IF EXISTS is_silent;

ALTER TABLE conversation_members
    DROP COLUMN IF EXISTS hidden_before;
