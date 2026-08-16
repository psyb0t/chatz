package models

// MessageRole is the author of a chat message. Finite domain enum; alias form
// (no methods) so it marshals to/from a plain string at the DB + JSON boundary.
type MessageRole = string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleTool      MessageRole = "tool"

	// MessageRoleSystem is only ever stored for a tool's message injection
	// that a run ended on. Ordinary system prompts are built per request and
	// never persisted, so a stored system row always has IsInjection set.
	MessageRoleSystem MessageRole = "system"
)

// MCPTransport is how an MCP server is reached.
type MCPTransport = string

const (
	MCPTransportStdio MCPTransport = "stdio"
	MCPTransportHTTP  MCPTransport = "http"
)

// MCPSource records whether a server came from the config file (read-only in
// the UI) or was added by an admin through the UI (fully editable).
type MCPSource = string

const (
	MCPSourceConfig MCPSource = "config"
	MCPSourceDB     MCPSource = "db"
)
