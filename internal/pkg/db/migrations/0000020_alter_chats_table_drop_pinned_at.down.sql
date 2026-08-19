-- Restores the column and the pinned-order index, not the data: every pin
-- timestamp is gone for good.
DROP INDEX IF EXISTS idx_chats_user_active_updated;

ALTER TABLE chats
    ADD COLUMN pinned_at TIMESTAMPTZ;

CREATE INDEX idx_chats_user_active_pinned_updated
    ON chats (user_id, pinned_at DESC, updated_at DESC)
    WHERE deleted_at IS NULL;
