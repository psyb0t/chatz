ALTER TABLE chats
    ADD COLUMN project_id UUID REFERENCES projects (id) ON DELETE SET NULL,
    ADD COLUMN archived_at TIMESTAMPTZ,
    ADD COLUMN pinned_at TIMESTAMPTZ;

CREATE INDEX idx_chats_user_active_pinned_updated
    ON chats (user_id, pinned_at DESC, updated_at DESC)
    WHERE deleted_at IS NULL AND archived_at IS NULL;

CREATE INDEX idx_chats_project_id ON chats (project_id)
    WHERE deleted_at IS NULL;
