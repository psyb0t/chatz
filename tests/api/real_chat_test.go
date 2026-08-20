//go:build api

package api

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// realChatSkipMsg explains why the test is skipped without a real upstream.
//
//nolint:lll // single message; splitting with `+` is banned
const realChatSkipMsg = "real_chat needs CHATZ_UPSTREAMS plus the apiKeyEnv it names (the make test-real .env)"

// realChatPrompt must stay one logical line — it is typed verbatim into the
// composer, so a literal newline would change what gets submitted.
//
//nolint:lll // single prompt line; splitting with `+` is banned
const realChatPrompt = "Reply in one short sentence. Include exactly one **bold** word and one `code` span."

const (
	modelPickerSel       = "[data-testid=model-picker]"
	modelPickerSearchSel = "[data-testid=model-picker-search]"
	sendSel              = "[data-testid=send]"
	assistantTextSel     = "[data-testid=assistant-text]"
	stopSel              = "[data-testid=stop]"

	modelPickerWaitSec = 20
	modelSearchWaitSec = 10
	assistantWaitSec   = 45
	streamSettleMS     = 8000
	stopHiddenWaitSec  = 30
)

// pickModelJS returns JS that opens the searchable model picker's option
// list, clicks the option whose text exactly matches the given JSON-encoded
// model id, and returns whether it was found. JS backticks are avoided (they
// would clash with the surrounding Go backtick string), so the match uses
// plain string concatenation.
func pickModelJS(modelJSON string) string {
	return "(() => {" +
		"const opt = [...document.querySelectorAll('.picker__option')]" +
		".find((b) => b.textContent.trim() === " + modelJSON + ");" +
		"if (!opt) return JSON.stringify(false);" +
		"opt.click();" +
		"return JSON.stringify(true);" +
		"})()"
}

const pickerShownJS = "JSON.stringify(" +
	"document.querySelector('[data-testid=model-picker]')" +
	".textContent.trim())"

const markdownProbeJS = "JSON.stringify({" +
	"strong: document.querySelectorAll(" +
	"'[data-testid=message] strong').length," +
	"code: document.querySelectorAll(" +
	"'[data-testid=message] code').length," +
	"hasText: !!((document.querySelector(" +
	"'[data-testid=assistant-text]')||{}).textContent)" +
	"})"

type markdownProbe struct {
	Strong  int  `json:"strong"`
	Code    int  `json:"code"`
	HasText bool `json:"hasText"`
}

// TestRealChat drives a real streamed model turn: selects a real model,
// sends a prompt asking for **bold** + `code`, and asserts the reply
// rendered as markdown plus the SSE lifecycle console events fired.
func TestRealChat(t *testing.T) {
	if !testinfra.RealUpstreamConfigured() {
		t.Skip(realChatSkipMsg)
	}

	t.Parallel()

	ctx := t.Context()
	model := testinfra.RealModel()

	stack, client := newStack(
		t,
		testinfra.APIOptions{RealUpstream: true},
		testinfra.BrowserOptions{},
	)

	require.NoError(t, client.EnableNetworkLog(ctx))
	setupAdmin(ctx, t, stack, client)

	require.NoError(t, client.WaitForElement(
		ctx, modelPickerSel, stateVisible, modelPickerWaitSec,
	))
	require.NoError(t, client.Eval(ctx, clickJS(modelPickerSel)))
	require.NoError(t, client.WaitForElement(
		ctx, modelPickerSearchSel, stateVisible, modelSearchWaitSec,
	))
	require.NoError(t, client.Fill(ctx, modelPickerSearchSel, model))

	modelJSON, err := json.Marshal(model)
	require.NoError(t, err)

	var picked bool
	require.NoError(t, client.EvalJSON(
		ctx, pickModelJS(string(modelJSON)), &picked,
	))
	require.True(t, picked, "model %q not offered by the picker", model)

	var shown string
	require.NoError(t, client.EvalJSON(ctx, pickerShownJS, &shown))
	assert.Contains(t, shown, model)

	require.NoError(t, client.Fill(ctx, composerInput, realChatPrompt))
	require.NoError(t, client.Eval(ctx, submitJS(sendSel)))

	require.NoError(t, client.WaitForElement(
		ctx, assistantTextSel, stateVisible, assistantWaitSec,
	))
	require.NoError(t, client.Eval(ctx, settleJS(streamSettleMS)))
	require.NoError(t, client.WaitForElement(
		ctx, stopSel, stateHidden, stopHiddenWaitSec,
	))

	var probe markdownProbe
	require.NoError(t, client.EvalJSON(ctx, markdownProbeJS, &probe))
	assert.True(t, probe.HasText)
	assert.Greater(t, probe.Strong, 0)
	assert.Greater(t, probe.Code, 0)

	events := consoleEvents(ctx, t, client)
	assert.False(t, events["sse.error"])
	assert.True(t, events["sse.open"])
	assert.True(t, events["sse.text_delta"])
	assert.True(t, events["sse.done"])

	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), "real_chat.png"))
}
