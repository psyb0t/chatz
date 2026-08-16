//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	chatWorkflowSearchText      = "production"
	chatWorkflowProjectName     = "Operations"
	chatWorkflowSettleMS        = 1500
	chatWorkflowSearchWaitMS    = 800
	chatWorkflowActionWaitSec   = 15
	chatWorkflowResponseWaitSec = 120

	chatWorkflowSearchSel          = "[data-testid=chat-search]"
	chatWorkflowProjectCreateInput = "[data-testid=chat-project-create-input]"
	chatWorkflowProjectCreate      = "[data-testid=chat-project-create]"
	chatWorkflowProjectAssignment  = "[data-testid=chat-project-assignment]"
	chatWorkflowPin                = "[data-testid=chat-pin]"
	chatWorkflowArchive            = "[data-testid=chat-archive]"
	chatWorkflowDelete             = "[data-testid=chat-delete]"
	chatWorkflowScreenshot         = "chat_workflow.png"
)

const chatWorkflowAssignProjectJS = `(() => {
	const select = document.querySelector(
		'[data-testid=chat-project-assignment]',
	);
	const option = [...select.options].find(
		(item) => item.textContent.trim() === 'Operations',
	);
	if (!option) {
		throw new Error('Operations project option was not rendered');
	}
	select.value = option.value;
	select.dispatchEvent(new Event('change', {bubbles: true}));
})()`

const chatWorkflowArchiveViewJS = `document
	.querySelectorAll('[data-testid=chat-archive-toggle] button')[1]
	.click()`

const chatWorkflowProbeJS = `JSON.stringify({
	chatCount: document
		.querySelectorAll('[data-testid=chat-project-assignment]')
		.length,
	projectNames: [...document
		.querySelectorAll('[data-testid=chat-project-filter] option')]
		.map((option) => option.textContent.trim()),
	pinned: document
		.querySelector('[data-testid=chat-pin]')
		?.textContent.trim(),
	empty: document.body.textContent.includes('NO CHATS YET'),
	noHorizontalOverflow:
		document.documentElement.scrollWidth <= window.innerWidth &&
		document.querySelector('.sidebar').scrollWidth <=
			document.querySelector('.sidebar').clientWidth,
})`

type chatWorkflowProbe struct {
	ChatCount            int      `json:"chatCount"`
	ProjectNames         []string `json:"projectNames"`
	Pinned               string   `json:"pinned"`
	Empty                bool     `json:"empty"`
	NoHorizontalOverflow bool     `json:"noHorizontalOverflow"`
}

// TestChatWorkflow drives the sidebar flow a user relies on for retaining and
// organizing a completed conversation: create a project, assign and pin the
// chat, search it, archive it, then delete it from the archive.
func TestChatWorkflow(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	stack, client := newStack(t, testinfra.E2EOptions{
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

	require.NoError(t, client.Fill(
		ctx,
		chatWorkflowProjectCreateInput,
		chatWorkflowProjectName,
	))
	require.NoError(t, client.Eval(ctx, clickJS(chatWorkflowProjectCreate)))
	require.NoError(t, client.Eval(ctx, settleJS(chatWorkflowSettleMS)))
	require.NoError(t, client.Eval(ctx, chatWorkflowAssignProjectJS))
	require.NoError(t, client.Eval(ctx, settleJS(chatWorkflowSettleMS)))
	require.NoError(t, client.Eval(ctx, clickJS(chatWorkflowPin)))
	require.NoError(t, client.Eval(ctx, settleJS(chatWorkflowSettleMS)))

	var active chatWorkflowProbe
	require.NoError(t, client.EvalJSON(ctx, chatWorkflowProbeJS, &active))
	assert.Equal(t, 1, active.ChatCount)
	assert.Contains(t, active.ProjectNames, chatWorkflowProjectName)
	assert.Equal(t, "Unpin", active.Pinned)
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

	require.NoError(t, client.Eval(ctx, clickJS(chatWorkflowArchive)))
	require.NoError(t, client.Eval(ctx, settleJS(chatWorkflowSettleMS)))

	var archivedFromActive chatWorkflowProbe
	require.NoError(t, client.EvalJSON(
		ctx,
		chatWorkflowProbeJS,
		&archivedFromActive,
	))
	assert.Equal(t, 0, archivedFromActive.ChatCount)
	assert.True(t, archivedFromActive.Empty)

	require.NoError(t, client.Eval(ctx, chatWorkflowArchiveViewJS))
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
