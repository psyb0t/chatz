CREATE TABLE mcp_servers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    name        TEXT NOT NULL,
    transport   TEXT NOT NULL,
    command     TEXT NOT NULL DEFAULT '',
    args        JSONB,
    url         TEXT NOT NULL DEFAULT '',
    headers_enc BYTEA,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  UUID
);

CREATE UNIQUE INDEX idx_mcp_servers_name ON mcp_servers (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_mcp_servers_deleted_at ON mcp_servers (deleted_at);
