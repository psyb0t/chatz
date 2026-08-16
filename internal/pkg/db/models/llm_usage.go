package models

import "github.com/google/uuid"

// LLMUsage is one row per upstream LLM call, written best-effort by the usage
// decorator. Reasoning + cached tokens are broken out from totals so cost
// dashboards can attribute spend to the owning chat / message.
type LLMUsage struct {
	Base

	Service            string
	Stage              string
	Model              string
	ReasoningEffort    string
	PromptTokens       int64
	CachedPromptTokens int64
	CompletionTokens   int64
	ReasoningTokens    int64
	TotalTokens        int64
	DurationMs         int64
	UserID             *uuid.UUID
	ChatID             *uuid.UUID
	MessageID          *uuid.UUID
	RequestID          string
	ErrorMessage       string
}

// TableName pins the table name.
func (LLMUsage) TableName() string {
	return "llm_usage"
}
