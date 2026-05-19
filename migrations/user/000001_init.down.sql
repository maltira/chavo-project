DROP TRIGGER IF EXISTS trigger_settings_updated_at ON settings;
DROP TRIGGER IF EXISTS trigger_profiles_updated_at ON profiles;
DROP FUNCTION IF EXISTS user_update_updated_at_column();

DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS profiles;