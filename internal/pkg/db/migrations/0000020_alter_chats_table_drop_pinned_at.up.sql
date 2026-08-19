-- Pinning is removed from the product. The activity index is rebuilt without
-- the pinned_at key rather than just dropped, so the sidebar's ordering query
-- (newest activity first) keeps an index.
DROP INDEX IF EXISTS idx_chats_user_active_pinned_updated;

ALTER TABLE chats
    DROP COLUMN pinned_at;

CREATE INDEX idx_chats_user_active_updated
    ON chats (user_id, updated_at DESC)
    WHERE deleted_at IS NULL;
