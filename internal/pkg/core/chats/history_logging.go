package chats

import (
	"context"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	chatzlogging "github.com/psyb0t/chatz/internal/pkg/logging"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
)

// logOutboundPrompt emits one exact post-limit, post-repair model round.
// Secret-shaped values are masked before reaching a log attribute.
func logOutboundPrompt(
	ctx context.Context,
	chat *models.Chat,
	round int,
	messages []elelem.Message,
) {
	logger := ctxscope.GetLogger(ctx)

	logger.Debug(
		"send chat prompt to llm",
		"chat_id", chat.ID,
		"model", chat.ModelID,
		"round", round,
		"context_budget", historyTokenBudget(chat),
		"message_count", len(messages),
	)

	for messageIndex, message := range messages {
		logger.Debug(
			"send chat prompt message to llm",
			"chat_id", chat.ID,
			"message_index", messageIndex,
			"round", round,
			"role", message.Role,
			"content", chatzlogging.RedactText(message.Text()),
			"reasoning", chatzlogging.RedactText(message.Reasoning),
			"tool_call_id", chatzlogging.RedactText(message.ToolCallID),
			"is_error", message.ToolResultIsError,
			"origin", message.Origin,
		)

		for toolCallIndex, call := range message.ToolCalls {
			arguments := chatzlogging.RedactText(string(call.Arguments))
			logger.Debug(
				"send chat prompt tool call to llm",
				"chat_id", chat.ID,
				"message_index", messageIndex,
				"round", round,
				"tool_call_index", toolCallIndex,
				"tool_call_id", chatzlogging.RedactText(call.ID),
				"tool_name", call.Name,
				"tool_arguments", arguments,
			)
		}
	}
}
