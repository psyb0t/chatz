//go:build api

package api

import (
	"context"
	"io"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	turnFailurePrompt    = "Retry this failed upstream request."
	turnFailureSendSel   = "[data-testid=send]"
	turnFailureStatusSel = "[data-testid=chat-turn-status]"
	turnFailureRetrySel  = "[data-testid=chat-retry-turn]"
	turnFailureErrorSel  = "[data-testid=chat-error]"
	turnFailureStopSel   = "[data-testid=stop]"
	turnFailureWaitSec   = 30
	turnFailureElapsedMS = 1200
)

const turnFailureStatusTextProbe = `JSON.stringify(
	document.querySelector('[data-testid=chat-turn-status]')?.textContent ?? ''
)`

const turnFailureUserCountProbe = `JSON.stringify(
	document.querySelectorAll('[data-role=user]').length
)`

func TestTurnFailureRetry(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	stack, client := newStack(
		t,
		testinfra.APIOptions{FailFirstUpstreamStream: true},
		testinfra.BrowserOptions{},
	)
	setupAdmin(ctx, t, stack, client)

	require.NoError(t, client.Fill(ctx, composerInput, turnFailurePrompt))
	require.NoError(t, client.Eval(ctx, submitJS(turnFailureSendSel)))
	require.NoError(t, client.WaitForElement(
		ctx,
		turnFailureStatusSel,
		stateVisible,
		turnFailureWaitSec,
	))
	require.NoError(t, client.Eval(ctx, settleJS(turnFailureElapsedMS)))

	var statusText string
	require.NoError(t,
		client.EvalJSON(ctx, turnFailureStatusTextProbe, &statusText))
	assert.Contains(t, statusText, "elapsed")

	err := client.WaitForElement(
		ctx,
		turnFailureErrorSel,
		stateVisible,
		turnFailureWaitSec,
	)
	if err != nil {
		logContainerOutput(ctx, t, "app", stack.App)
		logContainerOutput(ctx, t, "fake upstream", stack.Upstream)

		require.NoError(t, err)
	}

	require.NoError(t, client.WaitForElement(
		ctx,
		turnFailureRetrySel,
		stateVisible,
		turnFailureWaitSec,
	))
	require.NoError(t, client.Eval(ctx, clickJS(turnFailureRetrySel)))
	require.NoError(t, client.WaitForElement(
		ctx,
		turnFailureRetrySel,
		stateHidden,
		turnFailureWaitSec,
	))
	require.NoError(t, client.WaitForElement(
		ctx,
		turnFailureStopSel,
		stateHidden,
		turnFailureWaitSec,
	))

	var userCount int
	require.NoError(t,
		client.EvalJSON(ctx, turnFailureUserCountProbe, &userCount))
	assert.Equal(t, 2, userCount)
}

func logContainerOutput(
	ctx context.Context,
	t *testing.T,
	name string,
	container interface {
		Logs(context.Context) (io.ReadCloser, error)
	},
) {
	t.Helper()

	logs, err := container.Logs(ctx)
	if err != nil {
		return
	}

	data, err := io.ReadAll(logs)
	if err != nil {
		return
	}

	t.Log(name + " logs after missing terminal error:")
	t.Log(string(data))
}
