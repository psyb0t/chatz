DROP INDEX IF EXISTS idx_chats_project_id;
DROP INDEX IF EXISTS idx_chats_user_active_pinned_updated;

ALTER TABLE chats DROP COLUMN pinned_at;
ALTER TABLE chats DROP COLUMN archived_at;
ALTER TABLE chats DROP COLUMN project_id;
