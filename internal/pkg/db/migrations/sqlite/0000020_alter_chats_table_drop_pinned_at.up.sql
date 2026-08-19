-- SQLite dialect of the Postgres 0000020. The index is dropped first because
-- SQLite refuses to drop a column a live index still references.
DROP INDEX IF EXISTS idx_chats_user_active_pinned_updated;

ALTER TABLE chats DROP COLUMN pinned_at;

CREATE INDEX idx_chats_user_active_updated
    ON chats (user_id, updated_at DESC)
    WHERE deleted_at IS NULL;
