DROP INDEX IF EXISTS idx_chats_project_id;
DROP INDEX IF EXISTS idx_chats_user_active_pinned_updated;

ALTER TABLE chats
    DROP COLUMN pinned_at,
    DROP COLUMN archived_at,
    DROP COLUMN project_id;
