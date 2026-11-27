DROP INDEX IF EXISTS idx_profiles_username_active;

-- удаляем мягко удалённые дубликаты username, оставшиеся после переиспользования имени
DELETE FROM profiles p
WHERE p.deleted_at IS NOT NULL
  AND EXISTS (
      SELECT 1 FROM profiles newer
      WHERE newer.username = p.username
        AND newer.user_id <> p.user_id
        AND (
            newer.deleted_at IS NULL
            OR newer.created_at > p.created_at
            OR (newer.created_at = p.created_at AND newer.user_id > p.user_id)
        )
  );

ALTER TABLE profiles ADD CONSTRAINT profiles_username_key UNIQUE (username);
