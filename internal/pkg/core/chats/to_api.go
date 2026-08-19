package chats

import (
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/ctxerrors"
)

// ChatSummaryToAPI projects a stored chat row to the wire summary shape.
func ChatSummaryToAPI(c *models.Chat) api.ChatSummary {
	return api.ChatSummary{
		Id:        c.ID,
		Title:     c.Title,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
		PinnedAt:  c.PinnedAt,
	}
}

// MessageToAPI projects a stored message row to the wire shape. Thinking,
// toolCallId, and isError are omitted (nil) when the row carries none of them,
// so a plain user/text-only row's JSON is unchanged from before these fields
// existed. ToolCalls is unmarshalled from the row's stored JSON; a decode
// failure returns an error rather than silently dropping the tool calls a
// reload is specifically trying to reconstruct.
func MessageToAPI(m *models.Message) (api.Message, error) {
	out := api.Message{
		Id:        m.ID,
		Role:      api.MessageRole(m.Role),
		Content:   m.Content,
		CreatedAt: m.CreatedAt,
	}

	if m.Reasoning != "" {
		out.Thinking = &m.Reasoning
	}

	if m.ModelID != "" {
		out.Model = &m.ModelID
	}

	if m.TurnComplete != nil && !*m.TurnComplete {
		out.Incomplete = new(true)
	}

	if m.ToolCallID != "" {
		out.ToolCallId = &m.ToolCallID
		out.IsError = &m.IsError
	}

	if len(m.ToolCalls) > 0 {
		calls, err := storedToolCallsFromJSON(m.ToolCalls)
		if err != nil {
			return api.Message{},
				ctxerrors.Wrap(err, "unmarshal stored tool calls")
		}

		apiCalls := make([]api.MessageToolCall, 0, len(calls))
		for _, c := range calls {
			apiCalls = append(apiCalls, api.MessageToolCall{
				Id:        c.ID,
				Name:      c.Name,
				Arguments: string(c.Arguments),
			})
		}

		out.ToolCalls = &apiCalls
	}

	return out, nil
}

// ChatSettingsToAPI projects a chat's stored generation settings to the API
// shape. Unset numeric settings stay nil; an empty ReasoningEffort maps to a
// nil pointer (the wire omits it) rather than an invalid empty enum value.
func ChatSettingsToAPI(c *models.Chat) *api.ChatSettings {
	out := &api.ChatSettings{
		Temperature:      c.Temperature,
		TopP:             c.TopP,
		MaxOutputTokens:  c.MaxOutputTokens,
		MaxHistoryTokens: c.MaxHistoryTokens,
	}

	if c.ReasoningEffort != "" {
		effort := api.ChatSettingsReasoningEffort(c.ReasoningEffort)
		out.ReasoningEffort = &effort
	}

	return out
}

// PromptContextToAPI projects the backend-authoritative prompt selection to
// the compact composer meter shape.
func PromptContextToAPI(context PromptContext) api.PromptContextPreview {
	availableTokens := max(context.BudgetTokens-context.TotalTokens, 0)

	return api.PromptContextPreview{
		AvailableTokens:      availableTokens,
		BudgetTokens:         context.BudgetTokens,
		CurrentMessageTokens: context.CurrentMessageTokens,
		HistoryTokens:        context.HistoryTokens,
		OmittedMessages:      context.OmittedMessages,
		OmittedTurns:         context.OmittedTurns,
		RetainedMessages:     context.RetainedMessages,
		RetainedTurns:        context.RetainedTurns,
		SystemTokens:         context.SystemTokens,
		TotalTokens:          context.TotalTokens,
	}
}

// ChatMCPServerToAPI maps a chat's view of an MCP server to the wire shape.
func ChatMCPServerToAPI(sv ChatMCPServer) api.ChatMCPServer {
	return api.ChatMCPServer{
		Id:      sv.ID,
		Name:    sv.Name,
		Status:  api.ChatMCPServerStatus(sv.Status),
		Enabled: sv.Enabled,
	}
}
