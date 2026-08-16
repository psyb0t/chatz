-- Per-chat MCP server enablement. Stores the set of MCP server IDs whose tools
-- are WITHHELD from this chat's turns; the empty default ('[]') means every
-- globally-enabled server is available to the chat. JSONB array of UUID strings.
-- Constant default = metadata-only add, no table rewrite; existing chats keep
-- every server enabled.
ALTER TABLE chats
    ADD COLUMN disabled_mcp_server_ids JSONB NOT NULL DEFAULT '[]'::jsonb;
