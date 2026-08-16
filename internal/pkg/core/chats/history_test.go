package chats

import (
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryTokenBudget(t *testing.T) {
	t.Parallel()

	explicit := 123
	testCases := []struct {
		name string
		chat *models.Chat
		want int
	}{
		{name: "nil chat uses default", want: defaultHistoryTokenBudget},
		{
			name: "unset uses default",
			chat: &models.Chat{},
			want: defaultHistoryTokenBudget,
		},
		{
			name: "explicit cap wins",
			chat: &models.Chat{MaxHistoryTokens: &explicit},
			want: explicit,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.want, historyTokenBudget(testCase.chat))
		})
	}
}

func TestBuildPromptPreservesFullOrderedTranscript(t *testing.T) {
	t.Parallel()

	history := []elelem.Message{
		{Role: elelem.RoleUser, Content: elelem.Text("oldest question")},
		{Role: elelem.RoleAssistant, Content: elelem.Text("oldest answer")},
		{
			Role: elelem.RoleAssistant,
			ToolCalls: []elelem.ToolCall{{
				ID:        "call-1",
				Name:      "lookup",
				Arguments: []byte(`{"query":"status"}`),
			}},
		},
		{
			Role:       elelem.RoleTool,
			ToolCallID: "call-1",
			Content:    elelem.Text("healthy"),
		},
	}

	got := buildPrompt("sticky system", history, "current question").Messages()

	require.Len(t, got, 6)

	assert.Equal(t, elelem.RoleSystem, got[0].Role)
	assert.Equal(t, "sticky system", got[0].Text())

	assert.Equal(t, "oldest question", got[1].Text())
	assert.Equal(t, "oldest answer", got[2].Text())
	assert.Equal(t, "call-1", got[3].ToolCalls[0].ID)
	assert.Equal(t, "healthy", got[4].Text())

	assert.Equal(t, elelem.RoleUser, got[5].Role)
	assert.Equal(t, "current question", got[5].Text())
}

// A stored injection never gets replayed. It is scoped to the run that produced
// it, and its injector re-creates it when the situation recurs — replaying one
// steers the model about a tool result that is no longer the subject, and every
// later turn inherits it. elelem owns that rule; buildPrompt just has to feed
// history through WithHistory rather than hand-assembling a message slice,
// which is what makes the rule apply.
func TestBuildPromptDropsAStoredInjection(t *testing.T) {
	t.Parallel()

	history := []elelem.Message{
		{Role: elelem.RoleUser, Content: elelem.Text("oldest question")},
		{
			Role:    elelem.RoleSystem,
			Content: elelem.Text("stored injection"),
			Origin:  elelem.MessageOriginInjection,
		},
	}

	got := buildPrompt("sticky system", history, "current").Messages()

	require.Len(t, got, 3)
	assert.Equal(t, "sticky system", got[0].Text())
	assert.Equal(t, "oldest question", got[1].Text())
	assert.Equal(t, "current", got[2].Text())

	for _, message := range got {
		assert.NotEqual(t, elelem.MessageOriginInjection, message.Origin)
	}
}

// EVERY message the prompt carries is seed history — the system message, the
// stored turns, and this turn's question alike. None of it is this run's own
// output, and Origin is exactly what turnRows persists on: it keeps
// MessageOriginTurn. The user message is already written by
// savePendingUserMessage before the turn runs, so one stamped Turn here would
// be stored a second time and show up twice in the chat.
func TestBuildPromptStampsEveryMessageAsSeed(t *testing.T) {
	t.Parallel()

	history := []elelem.Message{
		{
			Role:    elelem.RoleUser,
			Content: elelem.Text("stored"),
			Origin:  elelem.MessageOriginTurn,
		},
	}

	got := buildPrompt("system", history, "current").Messages()

	require.Len(t, got, 3)

	for _, message := range got {
		assert.Equal(
			t, elelem.MessageOriginSeed, message.Origin, message.Text(),
		)
	}

	// The concrete consequence, asserted against the code that does the
	// persisting rather than restated as an origin check.
	rows, err := turnRows(uuid.New(), got)
	require.NoError(t, err)
	assert.Empty(t, rows, "a seed prompt must contribute no stored rows")
}
