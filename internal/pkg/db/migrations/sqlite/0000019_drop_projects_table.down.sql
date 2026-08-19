-- Recreates the table exactly as the SQLite 0000016 built it. The rows are not
-- restored.
CREATE TABLE projects (
    id         TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_projects_user_name
    ON projects (user_id, name)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_projects_user_id ON projects (user_id);
CREATE INDEX idx_projects_deleted_at ON projects (deleted_at);
