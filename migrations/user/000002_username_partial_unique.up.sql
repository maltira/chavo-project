-- освобождение username сразу после мягкого удаления профиля
ALTER TABLE profiles DROP CONSTRAINT IF EXISTS profiles_username_key;
CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_username_active ON profiles(username) WHERE deleted_at IS NULL;
