DROP INDEX IF EXISTS idx_message_mentions_unread;
DROP TABLE IF EXISTS message_mentions;

ALTER TABLE messages
    DROP COLUMN IF EXISTS link_preview;
