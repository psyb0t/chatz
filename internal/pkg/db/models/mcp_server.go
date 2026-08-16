package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// MCPServer is an admin-added MCP server (config-file servers are held in
// memory, not here). Transport is stdio | http. For stdio, Command + Args spawn
// the process (EnvEnc holds any env-var secrets, AEAD-encrypted); for http, URL
// + HeadersEnc reach the endpoint. Both *Enc columns are sealed at rest —
// secrets never land in plaintext.
type MCPServer struct {
	Base

	Name       string
	Transport  MCPTransport
	Command    string
	Args       datatypes.JSON
	URL        string
	HeadersEnc []byte
	EnvEnc     []byte
	Enabled    bool
	CreatedBy  *uuid.UUID
}

// TableName pins the table name.
func (MCPServer) TableName() string {
	return "mcp_servers"
}
