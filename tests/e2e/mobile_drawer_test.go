//go:build e2e

package e2e

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	mobileViewport = "414x896"

	drawerAnimSettleMS = 400

	mobileBreakpointPx    = 640
	offCanvasRightMaxPx   = 1
	openMinVisibleWidthPx = 100

	sidebarOpenSel     = "[data-testid=sidebar-open]"
	sidebarBackdropSel = "[data-testid=sidebar-backdrop]"
	sidebarToggleSel   = "[data-testid=sidebar-toggle]"
	adminSystemSel     = "[data-testid=admin-system]"
	userCreateUserSel  = "[data-testid=user-create-username]"

	sidebarOpenWaitSec = 10
	adminNavWaitSec    = 15
)

// stateExpr is the DOM probe returning the drawer geometry + affordance
// presence as a JSON string in one round-trip.
const stateExpr = `(() => {
const aside = document.querySelector('[data-testid=chat-list]');
const ham = document.querySelector('[data-testid=sidebar-open]');
const backdrop = document.querySelector('[data-testid=sidebar-backdrop]');
const ar = aside ? aside.getBoundingClientRect() : null;
const hamShown = ham ? (
  getComputedStyle(ham).display !== 'none' &&
  ham.getBoundingClientRect().width > 0
) : false;
return JSON.stringify({
  innerWidth: window.innerWidth,
  matchesMobile: window.matchMedia('(max-width: 640px)').matches,
  path: location.pathname,
  aside: ar ? {
    left: Math.round(ar.left),
    right: Math.round(ar.right),
    width: Math.round(ar.width)
  } : null,
  hamburgerShown: hamShown,
  backdropPresent: !!backdrop
});
})()`

// mobileDrawerState is the decoded shape of stateExpr.
type mobileDrawerState struct {
	InnerWidth      int                `json:"innerWidth"`
	MatchesMobile   bool               `json:"matchesMobile"`
	Path            string             `json:"path"`
	Aside           *mobileDrawerAside `json:"aside"`
	HamburgerShown  bool               `json:"hamburgerShown"`
	BackdropPresent bool               `json:"backdropPresent"`
}

// mobileDrawerAside is the sidebar's bounding-rect geometry, rounded.
type mobileDrawerAside struct {
	Left  int `json:"left"`
	Right int `json:"right"`
	Width int `json:"width"`
}

// readState evaluates stateExpr and decodes the drawer/hamburger/backdrop
// geometry snapshot.
func readState(
	ctx context.Context,
	t *testing.T,
	client *testinfra.BrowserClient,
) mobileDrawerState {
	t.Helper()

	var st mobileDrawerState
	require.NoError(t, client.EvalJSON(ctx, stateExpr, &st))

	return st
}

// assertClosed settles past the drawer's 0.2s slide animation, then asserts
// the sidebar is off-canvas with the hamburger showing and no backdrop.
func assertClosed(
	ctx context.Context,
	t *testing.T,
	client *testinfra.BrowserClient,
) {
	t.Helper()

	require.NoError(t, client.Eval(ctx, settleJS(drawerAnimSettleMS)))
	st := readState(ctx, t, client)

	require.NotNil(t, st.Aside)
	assert.LessOrEqual(t, st.Aside.Right, offCanvasRightMaxPx)
	assert.True(t, st.HamburgerShown)
	assert.False(t, st.BackdropPresent)
}

// assertOpen settles past the drawer's 0.2s slide animation, then asserts
// the sidebar occupies the left of the viewport with a backdrop present.
func assertOpen(
	ctx context.Context,
	t *testing.T,
	client *testinfra.BrowserClient,
) {
	t.Helper()

	require.NoError(t, client.Eval(ctx, settleJS(drawerAnimSettleMS)))
	st := readState(ctx, t, client)

	require.NotNil(t, st.Aside)
	assert.GreaterOrEqual(t, st.Aside.Left, -offCanvasRightMaxPx)
	assert.GreaterOrEqual(t, st.Aside.Right, openMinVisibleWidthPx)
	assert.True(t, st.BackdropPresent)
}

// TestMobileDrawer verifies the phone off-canvas sidebar: it starts hidden
// behind a hamburger, slides in over the content with a dimming backdrop on
// tap, closes on a backdrop tap, closes on the in-drawer close button, and
// closes on navigation away from the drawer.
func TestMobileDrawer(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	stack, client := newStack(
		t,
		testinfra.E2EOptions{},
		testinfra.BrowserOptions{Viewport: mobileViewport},
	)

	setupAdmin(ctx, t, stack, client)
	require.NoError(t, client.WaitForElement(
		ctx, sidebarOpenSel, stateVisible, sidebarOpenWaitSec,
	))

	st := readState(ctx, t, client)
	require.LessOrEqual(t, st.InnerWidth, mobileBreakpointPx)
	require.True(t, st.MatchesMobile)

	assertClosed(ctx, t, client)
	_ = client.Screenshot(
		ctx, filepath.Join(t.TempDir(), "mobile_drawer_closed.png"),
	)

	require.NoError(t, client.Eval(ctx, clickJS(sidebarOpenSel)))
	assertOpen(ctx, t, client)
	_ = client.Screenshot(
		ctx, filepath.Join(t.TempDir(), "mobile_drawer_open.png"),
	)

	require.NoError(t, client.Eval(ctx, clickJS(sidebarBackdropSel)))
	assertClosed(ctx, t, client)
	_ = client.Screenshot(
		ctx, filepath.Join(t.TempDir(), "mobile_drawer_after_backdrop.png"),
	)

	require.NoError(t, client.Eval(ctx, clickJS(sidebarOpenSel)))
	assertOpen(ctx, t, client)
	require.NoError(t, client.Eval(ctx, clickJS(sidebarToggleSel)))
	assertClosed(ctx, t, client)

	require.NoError(t, client.Eval(ctx, clickJS(sidebarOpenSel)))
	assertOpen(ctx, t, client)
	// The drawer's admin entry is the gear that opens /admin, which lands on
	// the Users tab; navigating away must close the drawer.
	require.NoError(t, client.Eval(ctx, clickJS(adminSystemSel)))
	require.NoError(t, client.WaitForElement(
		ctx, userCreateUserSel, stateVisible, adminNavWaitSec,
	))
	assertClosed(ctx, t, client)
	_ = client.Screenshot(
		ctx, filepath.Join(t.TempDir(), "mobile_drawer_after_nav.png"),
	)

	navState := readState(ctx, t, client)
	assert.Contains(t, navState.Path, "admin/users")
}
