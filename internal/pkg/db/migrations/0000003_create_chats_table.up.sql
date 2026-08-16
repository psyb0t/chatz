CREATE TABLE chats (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title      TEXT NOT NULL DEFAULT '',
    model_id   TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_chats_user_id ON chats (user_id);
CREATE INDEX idx_chats_deleted_at ON chats (deleted_at);
