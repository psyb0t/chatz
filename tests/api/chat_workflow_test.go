//go:build api

package api

import (
	"path/filepath"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	chatWorkflowSearchText      = "production"
	chatWorkflowSettleMS        = 1500
	chatWorkflowSearchWaitMS    = 800
	chatWorkflowActionWaitSec   = 15
	chatWorkflowResponseWaitSec = 120

	chatWorkflowSearchSel  = "[data-testid=chat-search]"
	chatWorkflowMenu       = "[data-testid=chat-menu]"
	chatWorkflowDelete     = "[data-testid=chat-delete]"
	chatWorkflowScreenshot = "chat_workflow.png"
)

// The per-row menu trigger exists once per chat, so counting it counts the
// visible chats without depending on a control only present in an open menu.
const chatWorkflowProbeJS = `JSON.stringify({
	chatCount: document
		.querySelectorAll('[data-testid=chat-menu]')
		.length,
	empty: document.body.textContent.includes('NO CHATS YET'),
	noHorizontalOverflow:
		document.documentElement.scrollWidth <= window.innerWidth &&
		document.querySelector('.sidebar').scrollWidth <=
			document.querySelector('.sidebar').clientWidth,
})`

type chatWorkflowProbe struct {
	ChatCount            int  `json:"chatCount"`
	Empty                bool `json:"empty"`
	NoHorizontalOverflow bool `json:"noHorizontalOverflow"`
}

// TestChatWorkflow drives the sidebar flow a user relies on for pruning a
// completed conversation: search for it, open the row's ⋮ menu, then delete it.
func TestChatWorkflow(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	stack, client := newStack(t, testinfra.APIOptions{
		ShowcaseMode: true,
	}, testinfra.BrowserOptions{})

	setupAdmin(ctx, t, stack, client)
	require.NoError(t, client.Fill(ctx, composerInput, showcasePrompt))
	require.NoError(t, client.Eval(ctx, submitJS(showcaseSendSel)))
	require.NoError(t, client.WaitForElement(
		ctx,
		showcaseTimeSeriesChartSel,
		stateVisible,
		chatWorkflowResponseWaitSec,
	))
	require.NoError(t, client.WaitForElement(
		ctx,
		showcaseStopSel,
		stateHidden,
		chatWorkflowResponseWaitSec,
	))

	var active chatWorkflowProbe
	require.NoError(t, client.EvalJSON(ctx, chatWorkflowProbeJS, &active))
	assert.Equal(t, 1, active.ChatCount)
	assert.False(t, active.Empty)
	assert.True(t, active.NoHorizontalOverflow)

	require.NoError(t, client.Fill(
		ctx,
		chatWorkflowSearchSel,
		chatWorkflowSearchText,
	))
	require.NoError(t, client.Eval(ctx, settleJS(chatWorkflowSearchWaitMS)))

	var searched chatWorkflowProbe
	require.NoError(t, client.EvalJSON(ctx, chatWorkflowProbeJS, &searched))
	assert.Equal(t, 1, searched.ChatCount)
	assert.True(t, searched.NoHorizontalOverflow)

	// Open the row's ⋮ menu; the delete item only exists while it is open.
	require.NoError(t, client.Eval(ctx, clickJS(chatWorkflowMenu)))
	require.NoError(t, client.WaitForElement(
		ctx,
		chatWorkflowDelete,
		stateVisible,
		chatWorkflowActionWaitSec,
	))
	_ = client.Screenshot(
		ctx,
		filepath.Join(t.TempDir(), chatWorkflowScreenshot),
	)

	require.NoError(t, client.Eval(ctx, clickJS(chatWorkflowDelete)))
	require.NoError(t, client.Eval(ctx, settleJS(chatWorkflowSettleMS)))

	var deleted chatWorkflowProbe
	require.NoError(t, client.EvalJSON(ctx, chatWorkflowProbeJS, &deleted))
	assert.Equal(t, 0, deleted.ChatCount)
	assert.True(t, deleted.Empty)
	assert.True(t, deleted.NoHorizontalOverflow)

	events := consoleEvents(ctx, t, client)
	assert.False(t, events["chats.error"])
}
