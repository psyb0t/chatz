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
	usersPageSuffix    = "/admin/users?log=debug"
	adminLoadingSel    = "[data-testid=admin-loading]"
	adminLoadingWaitS  = 15
	usersScreenshotPNG = "users.png"
	usersLoadedEvent   = "users.loaded"
)

// usersProbeJS inspects the /admin/users page for the create-user controls and
// confirms the forbidden/error states are absent.
const usersProbeJS = `(() => {
  const forbidden = document.querySelector('[data-testid=admin-forbidden]');
  const error = document.querySelector('[data-testid=admin-error]');
  const username = document.querySelector(
    '[data-testid=user-create-username]');
  const password = document.querySelector(
    '[data-testid=user-create-password]');
  const adminCheck = document.querySelector(
    '[data-testid=user-create-admin]');
  const submit = document.querySelector('[data-testid=user-create-submit]');
  return JSON.stringify({
    forbidden: !!forbidden,
    hasError: !!error,
    username: !!username,
    password: !!password,
    adminCheck: !!adminCheck,
    submit: !!submit,
  });
})()`

type usersProbeResult struct {
	Forbidden  bool `json:"forbidden"`
	HasError   bool `json:"hasError"`
	Username   bool `json:"username"`
	Password   bool `json:"password"`
	AdminCheck bool `json:"adminCheck"`
	Submit     bool `json:"submit"`
}

// TestUsers drives the /admin/users page and confirms the admin (non-forbidden)
// shell renders with create-user controls, and that the users.loaded structured
// console event was emitted.
func TestUsers(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	stack, client := newStack(
		t, testinfra.APIOptions{}, testinfra.BrowserOptions{},
	)
	setupAdmin(ctx, t, stack, client)

	require.NoError(t,
		client.Goto(ctx, stack.AppURL+usersPageSuffix, networkIdle))
	require.NoError(t, client.WaitForElement(
		ctx, adminLoadingSel, stateHidden, adminLoadingWaitS,
	))

	var probe usersProbeResult
	require.NoError(t, client.EvalJSON(ctx, usersProbeJS, &probe))

	assert.False(t, probe.Forbidden)
	assert.False(t, probe.HasError)
	assert.True(t, probe.Username)
	assert.True(t, probe.Password)
	assert.True(t, probe.AdminCheck)
	assert.True(t, probe.Submit)

	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), usersScreenshotPNG))

	events := consoleEvents(ctx, t, client)
	assert.True(t, events[usersLoadedEvent])
}
