//go:build e2e

package e2e

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	chatMCPServerName = "e2e-chat-mcp"

	//nolint:lll // one logical prompt string; must match the showcase trigger exactly
	chatMCPShowcasePrompt = "Show me what's happening across the production platform right now."

	chatMCPSendSel       = "[data-testid=send]"
	chatMCPJRTypeSel     = "[data-jr-type]"
	chatMCPToggleSel     = "[data-testid=chat-mcp-toggle]"
	chatMCPPanelSel      = "[data-testid=chat-mcp-panel]"
	chatMCPItemSel       = "[data-testid=chat-mcp-item]"
	chatMCPItemToggleSel = "[data-testid=chat-mcp-item-toggle]"

	chatMCPJRTypeWaitSec  = 30
	chatMCPToggleWaitSec  = 10
	chatMCPPanelWaitSec   = 10
	chatMCPItemWaitSec    = 10
	chatMCPHandshakeMS    = 3000
	chatMCPToggleSettleMS = 1500

	chatMCPCreateServerJSFmt = `fetch('/api/v1/mcp/servers', {
	method: 'POST',
	credentials: 'include',
	headers: {'content-type': 'application/json'},
	body: JSON.stringify({
		name: 'e2e-chat-mcp',
		transport: 'http',
		url: '%s',
	}),
}).then((r) => JSON.stringify({status: r.status}))`

	chatMCPItemProbeJS = `(() => {
	const item = document.querySelector('[data-testid=chat-mcp-item]');
	const cb = document.querySelector(
		'[data-testid=chat-mcp-item-toggle]',
	);
	return JSON.stringify({
		text: item ? item.textContent.replace(/\s+/g, ' ').trim() : '',
		checked: cb ? cb.checked : false,
	});
})()`

	chatMCPToggleProbeJS = `(() => {
	const cb = document.querySelector(
		'[data-testid=chat-mcp-item-toggle]',
	);
	return JSON.stringify({checked: cb ? cb.checked : true});
})()`
)

// TestChatMCP ports the chat_mcp Python browser driver: register a real HTTP
// MCP server, open a chat via a showcase prompt, open the per-chat MCP picker,
// confirm the server renders enabled, then toggle it off and confirm the
// checkbox flips and both console events fired.
func TestChatMCP(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	stack, client := newStack(t, testinfra.E2EOptions{
		ShowcaseMode:  true,
		WithMCPServer: true,
	}, testinfra.BrowserOptions{})

	setupAdmin(ctx, t, stack, client)

	var createResp struct {
		Status int `json:"status"`
	}

	createJS := fmt.Sprintf(chatMCPCreateServerJSFmt, stack.MCPServerURL)
	require.NoError(t, client.EvalJSON(ctx, createJS, &createResp))
	assert.Contains(
		t,
		[]int{200, 201},
		createResp.Status,
		"create MCP server",
	)

	require.NoError(t,
		client.Fill(ctx, composerInput, chatMCPShowcasePrompt))
	require.NoError(t, client.Eval(ctx, submitJS(chatMCPSendSel)))
	require.NoError(t, client.WaitForElement(
		ctx, chatMCPJRTypeSel, stateVisible, chatMCPJRTypeWaitSec,
	))

	require.NoError(t, client.Eval(ctx, settleJS(chatMCPHandshakeMS)))
	require.NoError(t, client.WaitForElement(
		ctx, chatMCPToggleSel, stateVisible, chatMCPToggleWaitSec,
	))
	require.NoError(t, client.Eval(ctx, clickJS(chatMCPToggleSel)))
	require.NoError(t, client.WaitForElement(
		ctx, chatMCPPanelSel, stateVisible, chatMCPPanelWaitSec,
	))
	require.NoError(t, client.WaitForElement(
		ctx, chatMCPItemSel, stateVisible, chatMCPItemWaitSec,
	))

	var before struct {
		Text    string `json:"text"`
		Checked bool   `json:"checked"`
	}

	require.NoError(t,
		client.EvalJSON(ctx, chatMCPItemProbeJS, &before))
	assert.Contains(t, before.Text, chatMCPServerName)
	assert.True(t, before.Checked)

	require.NoError(t, client.Eval(ctx, clickJS(chatMCPItemToggleSel)))
	require.NoError(t, client.Eval(ctx, settleJS(chatMCPToggleSettleMS)))

	var after struct {
		Checked bool `json:"checked"`
	}

	require.NoError(t, client.EvalJSON(ctx, chatMCPToggleProbeJS, &after))
	assert.False(t, after.Checked)

	events := consoleEvents(ctx, t, client)
	assert.False(t, events["chat_mcp.error"])
	assert.True(t, events["chat_mcp.loaded"])
	assert.True(t, events["chat_mcp.toggle"])

	_ = client.Screenshot(
		ctx, filepath.Join(t.TempDir(), "chat_mcp.png"),
	)
}
