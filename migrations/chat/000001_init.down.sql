DROP TRIGGER IF EXISTS trigger_messages_updated_at ON messages;
DROP TRIGGER IF EXISTS trigger_participants_updated_at ON participants;
DROP TRIGGER IF EXISTS trigger_chats_updated_at ON chats;
DROP FUNCTION IF EXISTS chat_update_updated_at_column();

DROP TABLE IF EXISTS message_reads;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS participants;
DROP TABLE IF EXISTS chats;