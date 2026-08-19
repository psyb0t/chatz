-- Recreates the table exactly as 0000016 built it. The rows are not restored.
CREATE TABLE projects (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_projects_user_name
    ON projects (user_id, name)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_projects_user_id ON projects (user_id);
CREATE INDEX idx_projects_deleted_at ON projects (deleted_at);
