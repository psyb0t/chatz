package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// MCPToolExecution records one MCP tool call made while answering a message:
// which server + tool, the params, the result, and whether it errored. Keeps
// the tool timeline for a reloaded chat and feeds tool-latency metrics.
type MCPToolExecution struct {
	Base

	MessageID  uuid.UUID
	Server     string
	Tool       string
	Params     datatypes.JSON
	Result     string
	IsError    bool
	DurationMs int64
}

// TableName pins the table name.
func (MCPToolExecution) TableName() string {
	return "mcp_tool_executions"
}
