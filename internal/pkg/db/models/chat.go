package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Chat is one conversation, owned by a user. ModelID names the model profile
// (from chatz.yaml) the chat runs against. The generation settings are
// per-chat overrides: a nil pointer / empty ReasoningEffort means "unset" — the
// provider default applies (temperature/top_p/reasoning) or no cap is imposed
// (max_output/max_history tokens).
type Chat struct {
	Base

	UserID  uuid.UUID
	Title   string
	ModelID string

	PinnedAt *time.Time

	Temperature      *float64
	TopP             *float64
	ReasoningEffort  string
	MaxOutputTokens  *int
	MaxHistoryTokens *int

	// DisabledMCPServerIDs is the per-chat set of MCP server IDs whose tools
	// are withheld from this chat's turns. Empty (the default) means every
	// globally-enabled server is available. Explicit column tag: gorm's
	// default namer would mangle the MCP/IDs acronyms. See core/chats filter.
	//nolint:lll // struct tag + generic type won't fit 80 cols; not splittable
	DisabledMCPServerIDs datatypes.JSONSlice[uuid.UUID] `gorm:"column:disabled_mcp_server_ids"`
}

// TableName pins the table name.
func (Chat) TableName() string {
	return "chats"
}
