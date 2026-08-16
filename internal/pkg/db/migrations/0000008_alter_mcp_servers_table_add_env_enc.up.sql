-- stdio MCP servers carry env vars that may hold secrets (API keys). Store them
-- AEAD-encrypted at rest, symmetric with headers_enc. Nullable = metadata-only,
-- no table rewrite.
ALTER TABLE mcp_servers ADD COLUMN env_enc BYTEA;
