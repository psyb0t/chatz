-- Restores the columns and their indexes, not the data: every project
-- assignment and archive timestamp is gone for good. Runs after 0000019's down
-- has recreated the projects table the FK points at.
ALTER TABLE chats
    ADD COLUMN project_id UUID REFERENCES projects (id) ON DELETE SET NULL,
    ADD COLUMN archived_at TIMESTAMPTZ;

DROP INDEX IF EXISTS idx_chats_user_active_pinned_updated;

CREATE INDEX idx_chats_user_active_pinned_updated
    ON chats (user_id, pinned_at DESC, updated_at DESC)
    WHERE deleted_at IS NULL AND archived_at IS NULL;

CREATE INDEX idx_chats_project_id ON chats (project_id)
    WHERE deleted_at IS NULL;
