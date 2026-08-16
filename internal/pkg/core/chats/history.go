package chats

import (
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/elelem"
)

const (
	// defaultHistoryTokenBudget is the maximum prompt-context budget when the
	// chat has no explicit user setting.
	defaultHistoryTokenBudget = 100_000
)

// historyTokenBudget returns the user's cap or the safe default. A stored cap
// is already validated as positive at the HTTP boundary.
func historyTokenBudget(chat *models.Chat) int {
	if chat != nil && chat.MaxHistoryTokens != nil {
		return *chat.MaxHistoryTokens
	}

	return defaultHistoryTokenBudget
}

// buildPrompt assembles the selected conversation: sticky system message,
// retained ordered history, then this turn's question. selectPromptHistory
// applies Chatz's configured tiktoken cap before this builder runs.
//
// A stored injection keeps its own origin all the way in, because deciding
// whether a past injection may be replayed is elelem's call, not ours;
// WithHistory is what acts on that origin.
//
// The current user message is added as SEED, not via UserText. Origin is what
// turnRows persists on — it keeps MessageOriginTurn — and this message is
// already written by savePendingUserMessage before the turn runs. Letting
// UserText stamp it Turn would store it a second time, so every question would
// appear twice in the chat's history.
func buildPrompt(
	systemMessage string,
	history []elelem.Message,
	userMessage string,
) elelem.Prompt {
	return elelem.NewPrompt().
		WithSystem(systemMessage).
		WithHistory(history).
		Add(elelem.Message{
			Role:    elelem.RoleUser,
			Content: elelem.Text(userMessage),
			Origin:  elelem.MessageOriginSeed,
		})
}
