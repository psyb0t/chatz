-- Projects and archiving are removed from the product. Pinning stays, so the
-- activity index is rebuilt without the archived_at predicate rather than just
-- dropped: without the rebuild the sidebar's ordering query loses its index.
DROP INDEX IF EXISTS idx_chats_project_id;
DROP INDEX IF EXISTS idx_chats_user_active_pinned_updated;

ALTER TABLE chats
    DROP COLUMN project_id,
    DROP COLUMN archived_at;

CREATE INDEX idx_chats_user_active_pinned_updated
    ON chats (user_id, pinned_at DESC, updated_at DESC)
    WHERE deleted_at IS NULL;
