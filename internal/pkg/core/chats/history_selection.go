package chats

import (
	"encoding/json"

	"github.com/psyb0t/chatz/internal/pkg/tiktoken"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/elelem"
)

// PromptContext describes the exact history selection Chatz made before an
// outbound model request. Component counts use the same tiktoken codec as the
// total; TotalTokens is counted over the fully assembled prompt representation.
type PromptContext struct {
	BudgetTokens         int
	SystemTokens         int
	HistoryTokens        int
	CurrentMessageTokens int
	TotalTokens          int
	OmittedMessages      int
	OmittedTurns         int
	RetainedMessages     int
	RetainedTurns        int

	History []elelem.Message
}

type promptTokenPayload struct {
	System  string               `json:"system"`
	History []promptTokenMessage `json:"history"`
	Current promptTokenMessage   `json:"current"`
}

type promptTokenMessage struct {
	Role          elelem.Role      `json:"role"`
	Content       string           `json:"content,omitempty"`
	Reasoning     string           `json:"reasoning,omitempty"`
	ToolCalls     []promptToolCall `json:"toolCalls,omitempty"`
	ToolCallID    string           `json:"toolCallId,omitempty"`
	ToolError     bool             `json:"toolError,omitempty"`
	ProviderState json.RawMessage  `json:"providerState,omitempty"`
}

type promptToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type historyUnit struct {
	messages []elelem.Message
}

// selectPromptHistory retains the newest complete history units that fit the
// supplied prompt budget. It adds units from newest to oldest and stops at the
// first unit that would exceed the cap; the sticky system message and current
// user message are never removed. A tool-call assistant message and all of its
// immediately following results are one unit, so the retained transcript never
// contains an orphan tool result.
func selectPromptHistory(
	systemMessage string,
	history []elelem.Message,
	userMessage string,
	budget int,
) (PromptContext, error) {
	retained := make([]elelem.Message, 0, len(history))
	units := historyUnits(history)
	omittedMessages := 0
	omittedTurns := 0

	for index := len(units) - 1; index >= 0; index-- {
		candidate := append(
			append([]elelem.Message(nil), units[index].messages...),
			retained...,
		)

		total, err := promptTokenCount(systemMessage, candidate, userMessage)
		if err != nil {
			return PromptContext{}, ctxerrors.Wrap(
				err,
				"count candidate prompt",
			)
		}

		if total > budget {
			for _, omittedUnit := range units[:index+1] {
				omittedMessages += len(omittedUnit.messages)
			}

			omittedTurns = index + 1

			break
		}

		retained = candidate
	}

	return newPromptContext(
		systemMessage,
		retained,
		userMessage,
		budget,
		omittedMessages,
		omittedTurns,
	)
}

func newPromptContext(
	systemMessage string,
	history []elelem.Message,
	userMessage string,
	budget, omittedMessages, omittedTurns int,
) (PromptContext, error) {
	systemTokens, err := tiktoken.Count(systemMessage)
	if err != nil {
		return PromptContext{}, ctxerrors.Wrap(
			err,
			"count sticky system prompt",
		)
	}

	historyTokens, err := promptHistoryTokenCount(history)
	if err != nil {
		return PromptContext{}, ctxerrors.Wrap(err, "count retained history")
	}

	currentTokens, err := tiktoken.Count(userMessage)
	if err != nil {
		return PromptContext{}, ctxerrors.Wrap(
			err,
			"count current user message",
		)
	}

	totalTokens, err := promptTokenCount(systemMessage, history, userMessage)
	if err != nil {
		return PromptContext{}, ctxerrors.Wrap(err, "count selected prompt")
	}

	return PromptContext{
		BudgetTokens:         budget,
		SystemTokens:         systemTokens,
		HistoryTokens:        historyTokens,
		CurrentMessageTokens: currentTokens,
		TotalTokens:          totalTokens,
		OmittedMessages:      omittedMessages,
		OmittedTurns:         omittedTurns,
		RetainedMessages:     len(history),
		RetainedTurns:        len(historyUnits(history)),
		History:              history,
	}, nil
}

func historyUnits(history []elelem.Message) []historyUnit {
	units := make([]historyUnit, 0, len(history))

	for index := 0; index < len(history); {
		message := history[index]
		if message.Origin == elelem.MessageOriginInjection {
			index++

			continue
		}

		end := index + 1
		if len(message.ToolCalls) > 0 {
			for end < len(history) && history[end].Role == elelem.RoleTool {
				end++
			}
		}

		units = append(units, historyUnit{
			messages: append([]elelem.Message(nil), history[index:end]...),
		})
		index = end
	}

	return units
}

func promptHistoryTokenCount(history []elelem.Message) (int, error) {
	payload, err := json.Marshal(promptTokenMessages(history))
	if err != nil {
		return 0, ctxerrors.Wrap(err, "marshal history for token count")
	}

	count, err := tiktoken.Count(string(payload))
	if err != nil {
		return 0, ctxerrors.Wrap(err, "tokenize history")
	}

	return count, nil
}

func promptTokenCount(
	systemMessage string,
	history []elelem.Message,
	userMessage string,
) (int, error) {
	payload, err := json.Marshal(promptTokenPayload{
		System:  systemMessage,
		History: promptTokenMessages(history),
		Current: promptTokenMessage{
			Role:    elelem.RoleUser,
			Content: userMessage,
		},
	})
	if err != nil {
		return 0, ctxerrors.Wrap(err, "marshal prompt for token count")
	}

	count, err := tiktoken.Count(string(payload))
	if err != nil {
		return 0, ctxerrors.Wrap(err, "tokenize prompt")
	}

	return count, nil
}

func promptTokenMessages(messages []elelem.Message) []promptTokenMessage {
	result := make([]promptTokenMessage, 0, len(messages))
	for _, message := range messages {
		calls := make([]promptToolCall, 0, len(message.ToolCalls))
		for _, call := range message.ToolCalls {
			calls = append(calls, promptToolCall{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
			})
		}

		result = append(result, promptTokenMessage{
			Role:          message.Role,
			Content:       message.Text(),
			Reasoning:     message.Reasoning,
			ToolCalls:     calls,
			ToolCallID:    message.ToolCallID,
			ToolError:     message.ToolResultIsError,
			ProviderState: message.ProviderReasoning,
		})
	}

	return result
}
