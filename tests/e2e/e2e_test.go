//go:build e2e

// Package e2e drives a real headless browser (stealthy-auto-browse) against the
// full prod stack (app image + Postgres + fake upstream, plus the MCP fixture
// or a real upstream when a driver needs it), all on one testcontainers net.
// Each test is fully self-contained: it brings up its own fresh stack + browser
// (a virgin DB is required — first-run /setup needs no existing user), drives
// the flow action-by-action, and tears down via t.Cleanup. This file is the
// shared scaffolding; one <driver>_test.go per flow calls only these helpers.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	_ "github.com/psyb0t/slogging/slogconf"
	"github.com/stretchr/testify/require"
)

const (
	setupSuffix   = "/setup?log=debug"
	composerInput = "#composer-input"
	consolePrefix = "CHATZ_LOG "

	stateVisible = "visible"
	stateHidden  = "hidden"
	networkIdle  = "networkidle"

	setupWaitSec = 15

	setupUsernameSel = "[data-testid=setup-username]"
	setupPasswordSel = "[data-testid=setup-password]" //nolint:gosec,lll // G101: DOM selector, not a credential
	setupSubmitSel   = "[data-testid=setup-submit]"
	appShellSel      = "[data-testid=app-shell]"
)

// newStack brings up a fresh e2e stack + browser for one test and registers
// teardown via t.Cleanup, so tests are independent and parallel-safe. Bring-up
// uses context.Background() (long image-build/boot waits have their own inner
// timeouts); the returned client is driven with the test's own ctx.
func newStack(
	t *testing.T,
	e2eOpts testinfra.E2EOptions,
	browserOpts testinfra.BrowserOptions,
) (*testinfra.E2EStack, *testinfra.BrowserClient) {
	t.Helper()

	setupCtx := context.Background()

	stack, err := testinfra.SetupE2E(setupCtx, e2eOpts)
	require.NoError(t, err, "bring up e2e stack")
	t.Cleanup(func() {
		require.NoError(t, stack.Teardown(context.Background()))
	})

	browser, err := testinfra.SetupBrowser(
		setupCtx,
		stack.NetworkName(),
		stack.AppURL,
		browserOpts,
	)
	require.NoError(t, err, "bring up browser")
	t.Cleanup(func() {
		require.NoError(t, browser.Close(context.Background()))
	})

	return stack, browser.Client
}

// setupAdmin enables console logging then runs the first-run /setup flow (make
// the admin, wait for the app shell). The ?log=debug query is what makes the
// client emit its CHATZ_LOG structured console events, so every driver's
// console assertions have something to read.
func setupAdmin(
	ctx context.Context,
	t *testing.T,
	stack *testinfra.E2EStack,
	client *testinfra.BrowserClient,
) {
	t.Helper()

	require.NoError(t, client.EnableConsoleLog(ctx))
	require.NoError(t,
		client.Goto(ctx, stack.AppURL+setupSuffix, networkIdle))
	require.NoError(t, client.WaitForElement(
		ctx, setupUsernameSel, stateVisible, setupWaitSec,
	))
	require.NoError(t, client.Fill(ctx, setupUsernameSel, testinfra.E2EUser))
	require.NoError(t, client.Fill(ctx, setupPasswordSel, testinfra.E2EPass))
	// Camoufox hangs on a click of this app's submit buttons; requestSubmit
	// runs the identical Svelte onsubmit handler.
	require.NoError(t, client.Eval(ctx, submitJS(setupSubmitSel)))
	require.NoError(t, client.WaitForElement(
		ctx, appShellSel, stateVisible, setupWaitSec,
	))
}

// consoleEvents collects the set of CHATZ_LOG structured event names emitted to
// the browser console since EnableConsoleLog.
func consoleEvents(
	ctx context.Context,
	t *testing.T,
	client *testinfra.BrowserClient,
) map[string]bool {
	t.Helper()

	entries, err := client.ConsoleLog(ctx)
	require.NoError(t, err)

	events := map[string]bool{}

	for _, entry := range entries {
		rest, ok := strings.CutPrefix(entry.Text, consolePrefix)
		if !ok {
			continue
		}

		var payload struct {
			Event string `json:"event"`
		}

		if json.Unmarshal([]byte(rest), &payload) != nil {
			continue
		}

		if payload.Event != "" {
			events[payload.Event] = true
		}
	}

	return events
}

// submitJS returns JS that submits the form owning selector. Camoufox hangs on
// a click of this app's submit buttons; requestSubmit runs the Svelte onsubmit.
func submitJS(selector string) string {
	return "document.querySelector('" + selector + "').form.requestSubmit()"
}

// clickJS returns JS that clicks selector. Camoufox hangs on the click action,
// so every click in a driver goes through eval.
func clickJS(selector string) string {
	return "document.querySelector('" + selector + "').click()"
}

// settleJS returns JS that resolves after ms — the load-bearing async/animation
// settle waits the drivers depend on, expressed as an awaited eval.
func settleJS(ms int) string {
	return fmt.Sprintf("new Promise((r) => setTimeout(r, %d))", ms)
}
