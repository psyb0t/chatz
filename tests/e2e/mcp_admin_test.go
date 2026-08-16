//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const rowJSFmt = `(() => {
	const row = Array.from(
		document.querySelectorAll('.mcp-row'),
	).find((r) => r.textContent.includes('%s'));
	if (!row) return JSON.stringify({found:false});
	const status = row.querySelector('.mcp-row__status');
	const health = row.querySelector('[data-testid=mcp-health]');
	return JSON.stringify({
		found:true,
		text: row.textContent.replace(/\s+/g,' ').trim(),
		title: status ? status.getAttribute('title') : '',
		health: health ? health.textContent.replace(/\s+/g,' ').trim() : '',
	});
})()`

// TestMCPAdmin drives the admin MCP server page. It
// adds a real http-transport MCP server pointed at the mcpserver fixture,
// polls the row until it settles to "Connected", expands the tools panel,
// verifies the edit modal prefills the server's fields, reconnects, then
// covers the failure path — a server pointed at an unreachable host settles
// to "Failed" with a reason in the status title.
func TestMCPAdmin(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	const (
		mcpAddToggleSel    = "[data-testid=mcp-add-toggle]"
		mcpModalSel        = "[data-testid=mcp-modal]"
		mcpCreateNameSel   = "[data-testid=mcp-create-name]"
		mcpCreateURLSel    = "[data-testid=mcp-create-url]"
		mcpCreateSubmitSel = "[data-testid=mcp-create-submit]"
		mcpToolsToggleSel  = "[data-testid=mcp-tools-toggle]"
		mcpToolsPanelSel   = "[data-testid=mcp-tools-panel]"
		mcpEditSel         = "[data-testid=mcp-edit]"
		mcpEditSubmitSel   = "[data-testid=mcp-edit-submit]"
		mcpReconnectSel    = "[data-testid=mcp-reconnect]"

		httpTransport = "http"

		mcpName  = "e2e-test-server"
		failName = "e2e-unreachable-server"
		failURL  = "http://nonexistent-host-e2e:9999/mcp"

		statusConnected = "Connected"
		statusFailed    = "Failed"

		waitAddToggleSec   = 15
		waitModalSec       = 10
		waitToolsToggleSec = 15
		waitToolsPanelSec  = 10

		toolsSettleMs       = 1000
		reconnectSettleMs   = 3000
		connectPollMs       = 1000
		failPollMs          = 250
		connectPollAttempts = 10
		failSettleWindowMs  = 20_000
		failPollAttempts    = failSettleWindowMs / failPollMs
	)

	setTransportJS := `(() => {
	const sel = document.querySelector(
		'[data-testid=mcp-create-transport]',
	);
	const setter = Object.getOwnPropertyDescriptor(
		window.HTMLSelectElement.prototype, 'value',
	).set;
	setter.call(sel, 'http');
	sel.dispatchEvent(new Event('change', {bubbles:true}));
	return JSON.stringify(sel.value);
})()`

	toolsPanelTextJS := `(() => {
	const p = document.querySelector('[data-testid=mcp-tools-panel]');
	return JSON.stringify(
		p ? p.textContent.replace(/\s+/g,' ').trim() : '',
	);
})()`

	prefillJS := `(() => {
	return JSON.stringify({
		name: document.querySelector(
			'[data-testid=mcp-create-name]',
		).value,
		transport: document.querySelector(
			'[data-testid=mcp-create-transport]',
		).value,
		url: document.querySelector(
			'[data-testid=mcp-create-url]',
		).value,
	});
})()`

	rowJS := func(name string) string {
		return fmt.Sprintf(rowJSFmt, name)
	}

	type mcpRow struct {
		Found  bool   `json:"found"`
		Text   string `json:"text"`
		Title  string `json:"title"`
		Health string `json:"health"`
	}

	readMCPRow := func(
		ctx context.Context,
		t *testing.T,
		client *testinfra.BrowserClient,
		name string,
	) mcpRow {
		t.Helper()

		var row mcpRow
		require.NoError(t, client.EvalJSON(ctx, rowJS(name), &row))

		return row
	}

	waitRowStatus := func(
		ctx context.Context,
		t *testing.T,
		client *testinfra.BrowserClient,
		name, status string,
		pollMs, attempts int,
	) {
		t.Helper()

		var row mcpRow
		for i := 0; i < attempts; i++ {
			row = readMCPRow(ctx, t, client, name)
			if row.Found && strings.Contains(row.Text, status) {
				return
			}

			require.NoError(t, client.Eval(ctx, settleJS(pollMs)))
		}

		require.Fail(
			t,
			"row never reached expected status",
			"name=%s status=%s last=%+v", name, status, row,
		)
	}

	addMCPServer := func(
		ctx context.Context,
		t *testing.T,
		client *testinfra.BrowserClient,
		name, url string,
	) {
		t.Helper()

		require.NoError(t, client.Eval(ctx, clickJS(mcpAddToggleSel)))
		require.NoError(t, client.WaitForElement(
			ctx, mcpModalSel, stateVisible, waitModalSec,
		))
		require.NoError(t, client.Fill(ctx, mcpCreateNameSel, name))

		var gotTransport string
		require.NoError(t,
			client.EvalJSON(ctx, setTransportJS, &gotTransport))
		require.Equal(t, httpTransport, gotTransport)

		require.NoError(t, client.Fill(ctx, mcpCreateURLSel, url))
		require.NoError(t, client.Eval(ctx, submitJS(mcpCreateSubmitSel)))
		require.NoError(t, client.WaitForElement(
			ctx, mcpModalSel, stateHidden, waitModalSec,
		))
	}

	stack, client := newStack(
		t,
		testinfra.E2EOptions{WithMCPServer: true},
		testinfra.BrowserOptions{},
	)
	setupAdmin(ctx, t, stack, client)

	require.NoError(t,
		client.Goto(ctx, stack.AppURL+"/admin/mcp", networkIdle))
	require.NoError(t, client.WaitForElement(
		ctx, mcpAddToggleSel, stateVisible, waitAddToggleSec,
	))

	addMCPServer(ctx, t, client, mcpName, stack.MCPServerURL)
	waitRowStatus(
		ctx, t, client, mcpName, statusConnected,
		connectPollMs, connectPollAttempts,
	)
	connectedRow := readMCPRow(ctx, t, client, mcpName)
	assert.Contains(t, connectedRow.Health, "Last attempt:")
	assert.Contains(t, connectedRow.Health, "Last success:")
	assert.Contains(t, connectedRow.Health, "Connect latency:")
	_ = client.Screenshot(
		ctx, filepath.Join(t.TempDir(), "mcp_admin_connected.png"),
	)

	require.NoError(t, client.WaitForElement(
		ctx, mcpToolsToggleSel, stateVisible, waitToolsToggleSec,
	))
	require.NoError(t, client.Eval(ctx, clickJS(mcpToolsToggleSel)))
	require.NoError(t, client.WaitForElement(
		ctx, mcpToolsPanelSel, stateVisible, waitToolsPanelSec,
	))
	require.NoError(t, client.Eval(ctx, settleJS(toolsSettleMs)))

	var panelText string
	require.NoError(t,
		client.EvalJSON(ctx, toolsPanelTextJS, &panelText))
	assert.Greater(t, len(panelText), 0)

	require.NoError(t, client.Eval(ctx, clickJS(mcpEditSel)))
	require.NoError(t, client.WaitForElement(
		ctx, mcpModalSel, stateVisible, waitModalSec,
	))

	var prefill struct {
		Name      string `json:"name"`
		Transport string `json:"transport"`
		URL       string `json:"url"`
	}
	require.NoError(t, client.EvalJSON(ctx, prefillJS, &prefill))
	assert.Equal(t, mcpName, prefill.Name)
	assert.Equal(t, httpTransport, prefill.Transport)
	assert.Equal(t, stack.MCPServerURL, prefill.URL)

	require.NoError(t, client.Eval(ctx, submitJS(mcpEditSubmitSel)))
	require.NoError(t, client.WaitForElement(
		ctx, mcpModalSel, stateHidden, waitModalSec,
	))

	require.NoError(t, client.Eval(ctx, clickJS(mcpReconnectSel)))
	require.NoError(t, client.Eval(ctx, settleJS(reconnectSettleMs)))

	row := readMCPRow(ctx, t, client, mcpName)
	assert.True(t, row.Found)
	assert.Contains(t, row.Text, statusConnected)

	addMCPServer(ctx, t, client, failName, failURL)
	waitRowStatus(
		ctx, t, client, failName, statusFailed,
		failPollMs, failPollAttempts,
	)

	failRow := readMCPRow(ctx, t, client, failName)
	assert.NotEmpty(t, failRow.Title)
	assert.Contains(t, failRow.Health, "Last attempt:")
	assert.Contains(t, failRow.Health, "Last failure:")
	assert.Contains(t, failRow.Health, "Last error:")

	consoleEvents(ctx, t, client)
}
