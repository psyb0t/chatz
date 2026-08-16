//go:build e2e

package e2e

import (
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	readinessRoute        = "/admin/readiness"
	readinessPageSel      = "[data-testid=admin-readiness-page]"
	readinessContentSel   = "[data-testid=admin-readiness-content]"
	readinessPageWaitSec  = 30
	readinessExpectedName = "fake"
)

const readinessProbeJS = `
(() => {
  const page = document.querySelector('[data-testid=admin-readiness-page]');
  return JSON.stringify({
    text: page ? page.textContent.replace(/\s+/g, ' ').trim() : '',
    noHorizontalOverflow: !page || page.scrollWidth <= page.clientWidth,
  });
})()
`

type readinessProbe struct {
	Text                 string `json:"text"`
	NoHorizontalOverflow bool   `json:"noHorizontalOverflow"`
}

func TestAdminReadiness(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	stack, client := newStack(
		t,
		testinfra.E2EOptions{},
		testinfra.BrowserOptions{},
	)
	setupAdmin(ctx, t, stack, client)

	require.NoError(t, client.Goto(
		ctx,
		stack.AppURL+readinessRoute,
		networkIdle,
	))

	if err := client.WaitForElement(
		ctx,
		readinessContentSel,
		stateVisible,
		readinessPageWaitSec,
	); err != nil {
		var failedProbe readinessProbe
		require.NoError(t, client.EvalJSON(ctx, readinessProbeJS, &failedProbe))
		require.Failf(
			t,
			"readiness content becomes visible",
			"wait error: %v; page text: %q",
			err,
			failedProbe.Text,
		)
	}

	var probe readinessProbe
	require.NoError(t, client.EvalJSON(ctx, readinessProbeJS, &probe))
	assert.Contains(t, probe.Text, "Runtime")
	assert.Contains(t, probe.Text, "Backup")
	assert.Contains(t, probe.Text, "not_recorded")
	assert.Contains(t, probe.Text, readinessExpectedName)
	assert.True(t, probe.NoHorizontalOverflow)
}
