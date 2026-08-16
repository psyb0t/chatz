package models

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// Message is one turn item in a chat. Role is user | assistant | tool, plus
// system for a stored injection (see IsInjection).
// Assistant rows may carry a Reasoning trace, streamed ToolCalls, and a
// rendered UISpec; tool rows carry the result keyed by ToolCallID plus
// whether it was an error (IsError). The set reconstructs the timeline.
type Message struct {
	Base

	Position          int64     `gorm:"default:(-)"`
	TurnID            uuid.UUID `gorm:"default:(-)"`
	TurnComplete      *bool     `gorm:"default:(-)"`
	ModelID           string
	ChatID            uuid.UUID
	Role              MessageRole
	Content           string
	Reasoning         string
	ProviderReasoning datatypes.JSON
	ToolCalls         datatypes.JSON
	ToolCallID        string
	IsError           bool

	// IsInjection marks a row a tool injected into the model's context rather
	// than something the user or the model said. Injections are per-run
	// scaffolding, so only one is ever stored: the trailing one a run ended on,
	// kept so an interrupted turn is not silently missing the instruction that
	// was pending when it stopped. The flag is what keeps it OUT of a rebuilt
	// history — a row that exists as a record, not as conversation.
	IsInjection bool

	UISpec       datatypes.JSON
	InputTokens  int64
	OutputTokens int64
}

// TableName pins the table name.
func (Message) TableName() string {
	return "messages"
}
