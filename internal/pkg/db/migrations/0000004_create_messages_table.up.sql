CREATE TABLE messages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ,
    chat_id       UUID NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    role          TEXT NOT NULL,
    content       TEXT NOT NULL DEFAULT '',
    tool_calls    JSONB,
    tool_call_id  TEXT NOT NULL DEFAULT '',
    ui_spec       JSONB,
    input_tokens  BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_messages_chat_id ON messages (chat_id);
CREATE INDEX idx_messages_deleted_at ON messages (deleted_at);
