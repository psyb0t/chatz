package chats

import (
	"testing"

	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectPromptHistory_KeepsNewestMessagesWithinBudget(t *testing.T) {
	t.Parallel()

	systemMessage := "sticky system"
	userMessage := "current user question"
	history := []elelem.Message{
		{Role: elelem.RoleUser, Content: elelem.Text("oldest user message")},
		{Role: elelem.RoleAssistant, Content: elelem.Text("middle answer")},
		{
			Role:    elelem.RoleUser,
			Content: elelem.Text("newest history question"),
		},
	}

	base, err := promptTokenCount(systemMessage, nil, userMessage)
	require.NoError(t, err)
	newest, err := promptTokenCount(systemMessage, history[1:], userMessage)
	require.NoError(t, err)
	require.Greater(t, newest, base)

	selection, err := selectPromptHistory(
		systemMessage,
		history,
		userMessage,
		newest,
	)
	require.NoError(t, err)

	require.Len(t, selection.History, 2)
	assert.Equal(t, "middle answer", selection.History[0].Text())
	assert.Equal(t, "newest history question", selection.History[1].Text())
	assert.Equal(t, 1, selection.OmittedMessages)
	assert.Equal(t, 1, selection.OmittedTurns)
	assert.Equal(t, 2, selection.RetainedMessages)
	assert.Equal(t, 2, selection.RetainedTurns)
	assert.Equal(t, newest, selection.TotalTokens)
	assert.LessOrEqual(t, selection.TotalTokens, selection.BudgetTokens)
}

func TestSelectPromptHistory_KeepsToolExchangeWhole(t *testing.T) {
	t.Parallel()

	systemMessage := "sticky system"
	userMessage := "current user question"
	history := []elelem.Message{
		{Role: elelem.RoleUser, Content: elelem.Text("oldest question")},
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

	budget, err := promptTokenCount(systemMessage, history[1:], userMessage)
	require.NoError(t, err)

	selection, err := selectPromptHistory(
		systemMessage,
		history,
		userMessage,
		budget,
	)
	require.NoError(t, err)

	require.Len(t, selection.History, 2)
	assert.Len(t, selection.History[0].ToolCalls, 1)
	assert.Equal(t, elelem.RoleTool, selection.History[1].Role)
	assert.Equal(t, "call-1", selection.History[1].ToolCallID)
	assert.Equal(t, 1, selection.OmittedMessages)
	assert.Equal(t, 1, selection.OmittedTurns)
	assert.Equal(t, 1, selection.RetainedTurns)
}

func TestSelectPromptHistory_AlwaysKeepsSystemAndCurrentMessage(t *testing.T) {
	t.Parallel()

	systemMessage := "a deliberately nonempty sticky system prompt"
	userMessage := "the current question is deliberately nonempty"

	minimum, err := promptTokenCount(systemMessage, nil, userMessage)
	require.NoError(t, err)
	require.Greater(t, minimum, 1)

	selection, err := selectPromptHistory(
		systemMessage,
		[]elelem.Message{{
			Role:    elelem.RoleUser,
			Content: elelem.Text("old history"),
		}},
		userMessage,
		minimum-1,
	)
	require.NoError(t, err)

	assert.Empty(t, selection.History)
	assert.Equal(t, minimum, selection.TotalTokens)
	assert.Greater(t, selection.TotalTokens, selection.BudgetTokens)
	assert.Equal(t, 1, selection.OmittedMessages)
	assert.Equal(t, 1, selection.OmittedTurns)
}

func TestSelectPromptHistory_DropsStoredInjectionsBeforeCounting(t *testing.T) {
	t.Parallel()

	selection, err := selectPromptHistory(
		"system",
		[]elelem.Message{{
			Role:    elelem.RoleSystem,
			Content: elelem.Text("stored injection"),
			Origin:  elelem.MessageOriginInjection,
		}},
		"current",
		1000,
	)
	require.NoError(t, err)
	assert.Empty(t, selection.History)
	assert.Zero(t, selection.OmittedMessages)
}
