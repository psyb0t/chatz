//go:build api

package api

import (
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	turnRecoveryPrompt      = "Start a reply that I will stop and reload."
	turnRecoveryPartialText = "Recovered partial answer."

	turnRecoveryAssistantSel = "[data-testid=assistant-text]"
	turnRecoverySendSel      = "[data-testid=send]"
	turnRecoveryStopSel      = "[data-testid=stop]"
	turnRecoveryWaitSec      = 30
)

const turnRecoveryChatURLProbe = "JSON.stringify(window.location.href)"

const turnRecoveryProbeJS = `JSON.stringify({
	partial: document.body.textContent.includes('Recovered partial answer.'),
	interrupted: document.body.textContent.includes('Interrupted response'),
	messages: document.querySelectorAll('[data-testid=message]').length
})`

type turnRecoveryProbe struct {
	Partial     bool `json:"partial"`
	Interrupted bool `json:"interrupted"`
	Messages    int  `json:"messages"`
}

func TestTurnRecovery(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	stack, client := newStack(
		t,
		testinfra.APIOptions{},
		testinfra.BrowserOptions{},
	)
	setupAdmin(ctx, t, stack, client)

	require.NoError(t, client.Fill(ctx, composerInput, turnRecoveryPrompt))
	require.NoError(t, client.Eval(ctx, submitJS(turnRecoverySendSel)))
	require.NoError(t, client.WaitForElement(
		ctx,
		turnRecoveryAssistantSel,
		stateVisible,
		turnRecoveryWaitSec,
	))
	require.NoError(t, client.Eval(ctx, clickJS(turnRecoveryStopSel)))
	require.NoError(t, client.WaitForElement(
		ctx,
		turnRecoveryStopSel,
		stateHidden,
		turnRecoveryWaitSec,
	))

	var chatURL string
	require.NoError(t, client.EvalJSON(
		ctx,
		turnRecoveryChatURLProbe,
		&chatURL,
	))
	require.NoError(t, client.Goto(ctx, chatURL, networkIdle))
	require.NoError(t, client.WaitForElement(
		ctx,
		turnRecoveryAssistantSel,
		stateVisible,
		turnRecoveryWaitSec,
	))

	var probe turnRecoveryProbe
	require.NoError(t, client.EvalJSON(ctx, turnRecoveryProbeJS, &probe))
	assert.True(t, probe.Partial)
	assert.True(t, probe.Interrupted)
	assert.Equal(t, 2, probe.Messages)
}
