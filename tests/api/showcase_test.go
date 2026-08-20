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
	showcasePrompt = "Show me what's happening across the " +
		"production platform right now."
	showcaseExpectedModel = "api-fake-gpt"
	showcaseScreenshot    = "showcase.png"

	showcaseModelPickerSel     = "[data-testid=model-picker]"
	showcaseTimeSeriesChartSel = `[data-jr-type="TimeSeriesChart"]`
	showcaseSendSel            = "[data-testid=send]"
	showcaseStopSel            = "[data-testid=stop]"
	showcaseTurnStatusSel      = "[data-testid=chat-turn-status]"

	showcaseModelPickerWaitSec = 30
	showcaseChartWaitSec       = 45
	showcaseStopHiddenWaitSec  = 120
	showcaseChartReloadWaitSec = 30
	showcaseTurnStatusWaitSec  = 10
)

const showcaseModelProbeJS = `JSON.stringify(
	document
		.querySelector('[data-testid=model-picker] .picker__value')
		.textContent.trim()
)`

const showcaseBeforeReloadProbeJS = `JSON.stringify({
	messages: document.querySelectorAll('[data-testid=message]').length,
	components: document.querySelectorAll('[data-jr-type]').length,
	thinking: document.querySelectorAll(
		'[data-testid=thinking-block]'
	).length,
	tools: [...document.querySelectorAll('[data-testid=tool-card]')].map(
		(el) => ({name: el.dataset.tool, done: el.dataset.done})
	),
	order: Array.from(
		document.querySelector('.message__assistant')?.children ?? []
	).map(
		(el) => el.dataset.testid ??
			(el.querySelector('[data-jr-type]') ? 'dashboard' : 'unknown')
	),
	text: document.body.textContent.includes('REQUEST VOLUME')
})`

const showcaseChatURLProbeJS = `JSON.stringify(window.location.href)`

const showcaseAfterReloadProbeJS = `JSON.stringify({
	messages: document.querySelectorAll('[data-testid=message]').length,
	text: document.body.textContent.includes('REQUEST VOLUME')
})`

type showcaseToolCard struct {
	Name string `json:"name"`
	Done string `json:"done"`
}

type showcaseBeforeReload struct {
	Messages   int                `json:"messages"`
	Components int                `json:"components"`
	Thinking   int                `json:"thinking"`
	Tools      []showcaseToolCard `json:"tools"`
	Order      []string           `json:"order"`
	Text       bool               `json:"text"`
}

type showcaseAfterReload struct {
	Messages int  `json:"messages"`
	Text     bool `json:"text"`
}

// TestShowcase drives the exact showcase prompt, asserts a streamed GenUI
// dashboard (thinking + two tool cards + TimeSeriesChart) rendered in the
// right order, then reloads the chat URL and asserts the turn is durable.
func TestShowcase(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	stack, client := newStack(t, testinfra.APIOptions{
		ShowcaseMode: true,
	}, testinfra.BrowserOptions{})

	require.NoError(t, client.EnableNetworkLog(ctx))
	setupAdmin(ctx, t, stack, client)

	require.NoError(t, client.WaitForElement(
		ctx, showcaseModelPickerSel, stateVisible,
		showcaseModelPickerWaitSec,
	))

	var model string
	require.NoError(t,
		client.EvalJSON(ctx, showcaseModelProbeJS, &model))
	assert.Equal(t, showcaseExpectedModel, model)

	require.NoError(t,
		client.Fill(ctx, composerInput, showcasePrompt))
	require.NoError(t, client.Eval(ctx, submitJS(showcaseSendSel)))
	require.NoError(t, client.WaitForElement(
		ctx, showcaseTurnStatusSel, stateVisible,
		showcaseTurnStatusWaitSec,
	))
	require.NoError(t, client.WaitForElement(
		ctx, showcaseTurnStatusSel, stateHidden,
		showcaseStopHiddenWaitSec,
	))

	require.NoError(t, client.WaitForElement(
		ctx, showcaseTimeSeriesChartSel, stateVisible,
		showcaseChartWaitSec,
	))
	require.NoError(t, client.WaitForElement(
		ctx, showcaseStopSel, stateHidden, showcaseStopHiddenWaitSec,
	))

	var beforeReload showcaseBeforeReload
	require.NoError(t, client.EvalJSON(
		ctx, showcaseBeforeReloadProbeJS, &beforeReload,
	))

	assert.Equal(t, 2, beforeReload.Messages)
	assert.GreaterOrEqual(t, beforeReload.Components, 1)
	assert.True(t, beforeReload.Text)
	assert.Equal(t, 1, beforeReload.Thinking)
	assert.Equal(t, []showcaseToolCard{
		{Name: "operations__get_service_health", Done: "true"},
		{Name: "operations__list_recent_events", Done: "true"},
	}, beforeReload.Tools)

	if assert.GreaterOrEqual(t, len(beforeReload.Order), 3) {
		assert.Equal(t, []string{
			"thinking-block", "tool-card", "tool-card",
		}, beforeReload.Order[:3])
	}

	var chatURL string
	require.NoError(t,
		client.EvalJSON(ctx, showcaseChatURLProbeJS, &chatURL))

	require.NoError(t, client.Goto(ctx, chatURL, networkIdle))
	require.NoError(t, client.WaitForElement(
		ctx, showcaseTimeSeriesChartSel, stateVisible,
		showcaseChartReloadWaitSec,
	))

	var afterReload showcaseAfterReload
	require.NoError(t, client.EvalJSON(
		ctx, showcaseAfterReloadProbeJS, &afterReload,
	))
	assert.Equal(t, 2, afterReload.Messages)
	assert.True(t, afterReload.Text)

	_ = client.Screenshot(
		ctx, filepath.Join(t.TempDir(), showcaseScreenshot),
	)
}
