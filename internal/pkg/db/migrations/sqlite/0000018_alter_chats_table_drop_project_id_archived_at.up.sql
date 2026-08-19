-- SQLite dialect of the Postgres 0000018. Same effect, one ALTER per
-- statement. The indexes are dropped first because SQLite refuses to drop a
-- column a live index still references.
DROP INDEX IF EXISTS idx_chats_project_id;
DROP INDEX IF EXISTS idx_chats_user_active_pinned_updated;

ALTER TABLE chats DROP COLUMN archived_at;
ALTER TABLE chats DROP COLUMN project_id;

CREATE INDEX idx_chats_user_active_pinned_updated
    ON chats (user_id, pinned_at DESC, updated_at DESC)
    WHERE deleted_at IS NULL;
