CREATE TABLE mcp_tool_executions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    message_id  UUID NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    server      TEXT NOT NULL,
    tool        TEXT NOT NULL,
    params      JSONB,
    result      TEXT NOT NULL DEFAULT '',
    is_error    BOOLEAN NOT NULL DEFAULT FALSE,
    duration_ms BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX idx_mcp_tool_executions_message_id ON mcp_tool_executions (message_id);
CREATE INDEX idx_mcp_tool_executions_deleted_at ON mcp_tool_executions (deleted_at);
