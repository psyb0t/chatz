package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/ctxerrors"
	"github.com/stretchr/testify/require"
)

const (
	// settleTimeout bounds how long a test waits for a status to reach its
	// terminal state. Generous on purpose: the assertions here are about WHAT
	// the manager settles on, never how fast, and a tight bound only buys
	// flakes on a loaded CI box.
	settleTimeout = 10 * time.Second

	// settleTick is how often the Eventually conditions re-check. Short enough
	// that a passing test does not idle, long enough not to spin a core.
	settleTick = 10 * time.Millisecond

	// testConnectTimeout / testConnectGrace shrink the connect deadlines so a
	// stalled-connect scenario settles in well under a second rather than the
	// real 10s+5s.
	testConnectTimeout = 200 * time.Millisecond
	testConnectGrace   = 200 * time.Millisecond

	// neverFiresBackoff is a retry backoff long enough that the timer is
	// guaranteed not to fire during a test that only asserts on whether one is
	// PENDING. Using a real duration rather than disabling retries keeps the
	// scheduling path identical to production.
	neverFiresBackoff = time.Hour
)

// newTestManager builds a Manager with shrunken connect deadlines. The
// timings are Manager FIELDS, set here before anything connects and never
// written again — so unlike the package-level vars this replaced, every test
// in this file can run t.Parallel() without racing its siblings.
func newTestManager(t *testing.T) *Manager {
	t.Helper()

	mgr := NewManager(testBox(t))
	mgr.connectTimeout = testConnectTimeout
	mgr.connectOuterGrace = testConnectGrace

	t.Cleanup(func() { _ = mgr.Close() })

	return mgr
}

// newRetryTestManager is newTestManager with an explicit retry backoff, for
// the tests that care about retry timing rather than connect timing.
func newRetryTestManager(t *testing.T, backoff time.Duration) *Manager {
	t.Helper()

	mgr := newTestManager(t)
	mgr.retryBackoff = backoff

	return mgr
}

// stalledStdioServer is an MCPServer whose command spawns but never speaks
// the MCP handshake — it simulates a server that accepts the stdio pipes and
// then hangs, the scenario that used to leave the manager's status stuck at
// StateConnecting forever (see manager.go's ConnectAsync doc comment).
func stalledStdioServer(name string) *models.MCPServer {
	return &models.MCPServer{
		Name:      name,
		Transport: models.MCPTransportStdio,
		Command:   "sleep",
		Args:      []byte(`["300"]`),
	}
}

// unreachableHTTPServer is an MCPServer whose URL nothing listens on, so
// Connect fails immediately (connection refused) rather than hanging —
// unlike stalledStdioServer, this settles to StateFailed fast without
// needing to wait out the connect deadlines, which keeps the retry tests
// focused on retry timing rather than connect timing.
func unreachableHTTPServer(name string) *models.MCPServer {
	return &models.MCPServer{
		Name:      name,
		Transport: models.MCPTransportHTTP,
		URL:       "http://127.0.0.1:1/mcp",
	}
}

// requireFailedState waits for name's status to reach StateFailed. Every
// caller here points at a deliberately unreachable target, so the state is
// fixed rather than a parameter.
func requireFailedState(t *testing.T, mgr *Manager, name string) {
	t.Helper()

	require.Eventually(t, func() bool {
		return mgr.Status(name).State == StateFailed
	}, settleTimeout, settleTick,
		"status for %q never reached %q", name, StateFailed)
}

// requireGenerationAbove waits for mgr's internal generation counter for name
// to exceed floor. Used to observe the retry timer's own ConnectAsync call
// bumping the generation, proving the retry actually fired rather than the
// test having driven it directly.
func requireGenerationAbove(
	t *testing.T,
	mgr *Manager,
	name string,
	floor uint64,
) {
	t.Helper()

	require.Eventually(t, func() bool {
		return genOf(mgr, name) > floor
	}, settleTimeout, settleTick,
		"generation for %q never advanced past %d", name, floor)
}

// genOf reads a server's current generation under the manager's lock — the
// generations map is otherwise unexported and mutated from background retry
// goroutines, so tests must not read it unsynchronized (that's a -race hit).
func genOf(mgr *Manager, name string) uint64 {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	return mgr.generations[name]
}

// retryPending reports whether a retry timer is armed for name, read under
// the retry lock for the same reason genOf takes the manager lock.
func retryPending(mgr *Manager, name string) bool {
	mgr.retryMu.Lock()
	defer mgr.retryMu.Unlock()

	_, pending := mgr.retries[name]

	return pending
}

// pendingRetryCount is the total number of armed retry timers.
func pendingRetryCount(mgr *Manager) int {
	mgr.retryMu.Lock()
	defer mgr.retryMu.Unlock()

	return len(mgr.retries)
}

// requirePendingRetry waits for a retry timer to be armed for name.
//
// Reaching StateFailed is NOT the same event: applyFailure calls
// setStatusIfCurrent with the failed status and only THEN scheduleRetry, so a
// test that waits for the state and immediately reads mgr.retries is racing
// that gap. It is small enough to pass locally forever and to lose under a
// loaded full-suite run, which is exactly how it showed up -- two failures in
// a release gate that had never flagged them before.
func requirePendingRetry(t *testing.T, mgr *Manager, name string) {
	t.Helper()

	require.Eventually(t, func() bool {
		return retryPending(mgr, name)
	}, settleTimeout, settleTick,
		"no retry timer was ever armed for %q", name)
}

func TestClassifyConnectErr(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  string
		want Reason
	}{
		{"refused", "dial: connection refused", ReasonUnreachable},
		{"no host", "dial tcp: lookup nope: no such host", ReasonUnreachable},
		{"401", "unexpected status 401 Unauthorized", ReasonAccessDenied},
		{"403", "server returned 403 forbidden", ReasonAccessDenied},
		{"deadline", "context deadline exceeded", ReasonNotResponding},
		{"timeout", "read: i/o timeout", ReasonNotResponding},
		{"other", "some weird protocol handshake error", ReasonFailed},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := classifyConnectErr(ctxerrors.New(tc.err))
			require.Equal(t, tc.want, got)
		})
	}
}

func TestManagerStatusLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	m := NewManager(nil)

	// Unknown server → zero status (the API layer maps this by enabled-ness).
	require.Equal(t, State(""), m.Status("nope").State)

	// Disable records a disabled status without any live client.
	m.Disable(ctx, "srv")
	require.Equal(t, StateDisabled, m.Status("srv").State)

	// Remove forgets the status entirely.
	m.Remove(ctx, "srv")
	require.Equal(t, State(""), m.Status("srv").State)
}

func TestMergeStatusRetainsTerminalTelemetry(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, time.August, 12, 2, 0, 0, 0, time.UTC)
	failedAt := startedAt.Add(250 * time.Millisecond)
	succeededAt := failedAt.Add(time.Minute)
	nextAttemptAt := succeededAt.Add(time.Minute)

	failed := Status{
		State:                   StateFailed,
		Reason:                  ReasonUnreachable,
		Error:                   "connection refused",
		LastConnectionAttemptAt: startedAt,
		LastConnectionFailureAt: failedAt,
		LastConnectionLatency:   250 * time.Millisecond,
		LastError:               "connection refused",
	}

	connecting := mergeStatus(failed, Status{
		State:                   StateConnecting,
		LastConnectionAttemptAt: nextAttemptAt,
	})
	require.Equal(t, nextAttemptAt, connecting.LastConnectionAttemptAt)
	require.Equal(t, failedAt, connecting.LastConnectionFailureAt)
	require.Equal(
		t,
		failed.LastConnectionLatency,
		connecting.LastConnectionLatency,
	)
	require.Equal(t, failed.LastError, connecting.LastError)

	connected := mergeStatus(connecting, Status{
		State:                      StateConnected,
		ToolCount:                  3,
		LastConnectionAttemptAt:    nextAttemptAt,
		LastSuccessfulConnectionAt: succeededAt,
		LastConnectionLatency:      100 * time.Millisecond,
	})
	require.Equal(t, succeededAt, connected.LastSuccessfulConnectionAt)
	require.Equal(t, failedAt, connected.LastConnectionFailureAt)
	require.Equal(t, 100*time.Millisecond, connected.LastConnectionLatency)
	require.Equal(t, failed.LastError, connected.LastError)
}

func TestManagerDisableRetainsHealthTelemetry(t *testing.T) {
	t.Parallel()

	mgr := NewManager(testBox(t))

	const name = "srv"

	startedAt := time.Date(2026, time.August, 12, 2, 0, 0, 0, time.UTC)
	failedAt := startedAt.Add(time.Second)
	gen := mgr.bumpGeneration(name)
	require.True(t, mgr.setStatusIfCurrent(name, gen, Status{
		State:                   StateFailed,
		Reason:                  ReasonUnreachable,
		Error:                   "connection refused",
		LastConnectionAttemptAt: startedAt,
		LastConnectionFailureAt: failedAt,
		LastConnectionLatency:   time.Second,
		LastError:               "connection refused",
	}))

	mgr.Disable(t.Context(), name)

	got := mgr.Status(name)
	require.Equal(t, StateDisabled, got.State)
	require.Equal(t, startedAt, got.LastConnectionAttemptAt)
	require.Equal(t, failedAt, got.LastConnectionFailureAt)
	require.Equal(t, time.Second, got.LastConnectionLatency)
	require.Equal(t, "connection refused", got.LastError)
}

func TestManagerServerTools_NotConnected(t *testing.T) {
	t.Parallel()

	// A server with no live client yields nil tools and no error — the admin UI
	// shows tools only for a connected server, so "not connected" is an empty
	// list, not a failure.
	tools, err := NewManager(nil).ServerTools(context.Background(), "nope")
	require.NoError(t, err)
	require.Nil(t, tools)
}

// TestConnectAsync_StalledConnectSettlesWithinBound proves a hung connect
// attempt (server spawns but never completes the handshake) settles to a
// terminal status — NOT indefinitely StateConnecting. This is the regression
// test for the stuck-at-"connecting" issue: previously, connectAndStore's own
// ctx timeout could be followed by ctx-unbound teardown (session Close()
// waiting out the subprocess's SIGTERM grace period), and nothing ever
// force-settled the status.
func TestConnectAsync_StalledConnectSettlesWithinBound(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	const name = "stalled"

	mgr.ConnectAsync(context.Background(), stalledStdioServer(name))
	requireFailedState(t, mgr, name)

	st := mgr.Status(name)
	require.Equal(t, ReasonNotResponding, st.Reason)
	require.NotEmpty(t, st.Error)
}

// TestConnectAsync_LateResultDoesNotClobberNewerStatus proves the generation
// guard: a connect attempt that is superseded by a newer one (e.g. the user
// clicks Reconnect while an old stalled attempt is still winding down in the
// background) must never have its eventual late result overwrite the newer
// attempt's outcome.
//
// Simulated directly against the generation primitives (bumpGeneration /
// setStatusIfCurrent) rather than via two real ConnectAsync calls, so the
// test is deterministic instead of racing real goroutine timing.
func TestConnectAsync_LateResultDoesNotClobberNewerStatus(t *testing.T) {
	t.Parallel()

	mgr := NewManager(testBox(t))

	const name = "srv"

	// Attempt 1 starts (e.g. the initial ConnectAsync from an edit-save).
	gen1 := mgr.bumpGeneration(name)
	require.True(t, mgr.setStatusIfCurrent(
		name, gen1, Status{State: StateConnecting},
	))

	// Attempt 2 supersedes it (e.g. the user clicks Reconnect) and completes
	// first, successfully.
	gen2 := mgr.bumpGeneration(name)
	require.True(t, mgr.setStatusIfCurrent(name, gen2, Status{
		State: StateConnected, ToolCount: 3,
	}))

	require.Equal(t, StateConnected, mgr.Status(name).State)

	// Attempt 1's result finally arrives late (the leaked goroutine from the
	// old stalled connect finishes). It must be discarded, not applied.
	applied := mgr.setStatusIfCurrent(name, gen1, Status{
		State:  StateFailed,
		Reason: ReasonNotResponding,
		Error:  "late failure from an abandoned attempt",
	})
	require.False(t, applied,
		"late result from a superseded generation must be discarded")

	// The newer, correct status must still be in place.
	st := mgr.Status(name)
	require.Equal(t, StateConnected, st.State)
	require.Equal(t, 3, st.ToolCount)
}

// TestConnectAsync_RemoveSupersedesInFlightAttempt proves Remove bumps the
// generation, so a connect attempt already in flight when the server is
// deleted can never resurrect a status afterward.
func TestConnectAsync_RemoveSupersedesInFlightAttempt(t *testing.T) {
	t.Parallel()

	mgr := NewManager(testBox(t))

	const name = "srv"

	gen := mgr.bumpGeneration(name)
	require.True(t, mgr.setStatusIfCurrent(
		name, gen, Status{State: StateConnecting},
	))

	mgr.Remove(context.Background(), name)
	require.Equal(t, State(""), mgr.Status(name).State)

	// The in-flight attempt's eventual result must not resurrect a status
	// for a server that was removed.
	applied := mgr.setStatusIfCurrent(name, gen, Status{State: StateConnected})
	require.False(t, applied)
	require.Equal(t, State(""), mgr.Status(name).State)
}

// TestAdd_SynchronousBehaviorUnchanged confirms Add still blocks and still
// returns the connect error directly — the generation guard must not alter
// its existing synchronous contract (tests/real and tests/mcp call Add and
// immediately check tools). Add does NOT apply the connect timeout itself; it
// trusts the caller's ctx for bounding, same as the real callers.
//
// The assertion window is generous (callerTimeout + a multi-second margin),
// NOT tight: once the ctx deadline fires, the SDK's error path closes the
// session synchronously, which waits out its own termination grace period
// for the still-running "sleep 300" to exit before sending SIGTERM. That
// teardown cost rides on top of the ctx deadline and is NOT bounded by the
// manager's connect deadlines — those only guard ConnectAsync's background
// path. Add keeps its pre-existing synchronous contract, unmodified;
// ConnectAsync (the path actually used by the admin UI) is the one that gets
// the hard bound.
func TestAdd_SynchronousBehaviorUnchanged(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	const (
		callerTimeout        = 300 * time.Millisecond
		closeTeardownHeadway = 8 * time.Second
	)

	ctx, cancel := context.WithTimeout(context.Background(), callerTimeout)
	defer cancel()

	start := time.Now()
	err := mgr.Add(ctx, stalledStdioServer("stalled-add"))
	elapsed := time.Since(start)

	require.Error(t, err)
	require.LessOrEqual(t, elapsed, callerTimeout+closeTeardownHeadway,
		"Add must still return once the ctx deadline plus teardown elapse")

	require.Equal(t, StateFailed, mgr.Status("stalled-add").State)
}

// TestConnectAsync_FailureSchedulesExactlyOneRetry proves an applied connect
// failure arms exactly one pending retry timer for that server name.
func TestConnectAsync_FailureSchedulesExactlyOneRetry(t *testing.T) {
	t.Parallel()

	mgr := newRetryTestManager(t, neverFiresBackoff)

	const name = "unreachable"

	mgr.ConnectAsync(context.Background(), unreachableHTTPServer(name))
	requireFailedState(t, mgr, name)

	// Waiting for the timer IS the "exactly one retry armed" assertion's first
	// half; the count below is the second.
	requirePendingRetry(t, mgr, name)
	require.Equal(t, 1, pendingRetryCount(mgr),
		"expected exactly one pending retry timer total")
}

// TestConnectAsync_RetryFiresAutomatically proves the retry timer actually
// fires on its own (no human/test driving it) and re-invokes ConnectAsync for
// the same server: the generation is observed advancing on its own, driven
// only by the backoff elapsing. The end-to-end "retry recovers against a
// real, now-reachable MCP server" path is covered by
// TestManager_AutoRetry_RecoversAfterServerComesUp in tests/mcp (needs the
// real Python MCP server fixture; this package has no working, non-python MCP
// transport to connect to).
func TestConnectAsync_RetryFiresAutomatically(t *testing.T) {
	t.Parallel()

	mgr := newRetryTestManager(t, 150*time.Millisecond)

	const name = "auto-fires"

	mgr.ConnectAsync(context.Background(), unreachableHTTPServer(name))
	requireFailedState(t, mgr, name)

	genAfterFirstFailure := genOf(mgr, name)

	// Nothing external drives this — just wait past the backoff. The retry
	// must fire on its own and bump the generation (ConnectAsync always
	// bumps), proving it actually re-attempted rather than silently doing
	// nothing.
	requireGenerationAbove(t, mgr, name, genAfterFirstFailure)

	// Then wait for the RE-ARMED retry, which is the observable that says the
	// second attempt's failure has been applied.
	//
	// Waiting on the status instead would prove nothing: it is still
	// StateFailed from the FIRST attempt and never observably leaves, so a
	// StateFailed check returns immediately and the assertions below race the
	// second failure. That is exactly how this test flaked.
	require.Eventually(t, func() bool {
		return retryPending(mgr, name)
	}, settleTimeout, settleTick,
		"a fresh failure must re-arm another retry")

	require.Equal(t, StateFailed, mgr.Status(name).State)
	require.Greater(t, genOf(mgr, name), genAfterFirstFailure,
		"the retry must have called ConnectAsync again, bumping the generation")
}

// TestConnectAsync_ManualReconnectCancelsPendingRetry proves that calling
// ConnectAsync again (as the admin Reconnect endpoint does) while a retry is
// pending cancels the stale timer — the old retry chain must not also fire.
func TestConnectAsync_ManualReconnectCancelsPendingRetry(t *testing.T) {
	t.Parallel()

	mgr := newRetryTestManager(t, neverFiresBackoff)

	const name = "reconnected"

	ctx := context.Background()

	mgr.ConnectAsync(ctx, unreachableHTTPServer(name))
	requireFailedState(t, mgr, name)
	requirePendingRetry(t, mgr, name)

	// Manual reconnect — same as the admin /reconnect endpoint calling
	// ConnectAsync again.
	mgr.ConnectAsync(ctx, unreachableHTTPServer(name))

	require.False(t, retryPending(mgr, name),
		"manual reconnect must cancel the stale pending retry")
}

// TestManager_DisableCancelsPendingRetry proves Disable cancels a pending
// retry so a disabled server never reconnects on its own.
func TestManager_DisableCancelsPendingRetry(t *testing.T) {
	t.Parallel()

	mgr := newRetryTestManager(t, neverFiresBackoff)

	const name = "disabled-mid-retry"

	ctx := context.Background()

	mgr.ConnectAsync(ctx, unreachableHTTPServer(name))
	requireFailedState(t, mgr, name)

	mgr.Disable(ctx, name)

	require.False(t, retryPending(mgr, name),
		"Disable must cancel the pending retry")
	require.Equal(t, StateDisabled, mgr.Status(name).State)
}

// TestManager_RemoveCancelsPendingRetry proves Remove cancels a pending
// retry so a deleted server never reconnects on its own.
func TestManager_RemoveCancelsPendingRetry(t *testing.T) {
	t.Parallel()

	mgr := newRetryTestManager(t, neverFiresBackoff)

	const name = "removed-mid-retry"

	ctx := context.Background()

	mgr.ConnectAsync(ctx, unreachableHTTPServer(name))
	requireFailedState(t, mgr, name)

	mgr.Remove(ctx, name)

	require.False(t, retryPending(mgr, name),
		"Remove must cancel the pending retry")
	require.Equal(t, State(""), mgr.Status(name).State)
}

// TestFireRetry_StaleGenerationNoOps proves the generation guard directly:
// a retry callback whose captured generation no longer matches the server's
// current generation (a Reconnect/Disable/Remove happened in between) must
// silently discard itself instead of reconnecting or rescheduling.
func TestFireRetry_StaleGenerationNoOps(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	const name = "stale"

	ctx := context.Background()

	gen := mgr.bumpGeneration(name)
	require.True(t,
		mgr.setStatusIfCurrent(name, gen, Status{State: StateFailed}))

	// Something supersedes this generation (e.g. a manual reconnect) before
	// the retry fires.
	newGen := mgr.bumpGeneration(name)
	require.True(t,
		mgr.setStatusIfCurrent(name, newGen, Status{State: StateConnected}))

	// The stale retry, still holding the OLD generation, fires.
	mgr.fireRetry(ctx, unreachableHTTPServer(name), gen)

	// It must not have reconnected (no StateConnecting transition triggered
	// by this call) and must not have scheduled a further retry.
	require.False(t, retryPending(mgr, name),
		"a stale retry must not reschedule itself")
}

// TestFireRetry_NotFailedAnymoreNoOps proves fireRetry only acts while the
// server is STILL StateFailed — e.g. it connected via some other path in
// between scheduling and firing.
func TestFireRetry_NotFailedAnymoreNoOps(t *testing.T) {
	t.Parallel()

	mgr := newTestManager(t)

	const name = "recovered-elsewhere"

	ctx := context.Background()

	gen := mgr.bumpGeneration(name)
	require.True(t,
		mgr.setStatusIfCurrent(name, gen, Status{State: StateFailed}))

	// Same generation, but status flips to Connected without a generation
	// bump (mirrors applyConnectResult recording a success at the same gen).
	require.True(t, mgr.setStatusIfCurrent(name, gen, Status{
		State: StateConnected, ToolCount: 1,
	}))

	mgr.fireRetry(ctx, unreachableHTTPServer(name), gen)

	require.Equal(t, StateConnected, mgr.Status(name).State,
		"fireRetry must not touch a server that is no longer StateFailed")
}

// TestManager_MultipleFailuresDoNotStackTimers proves repeated failures for
// the same server replace the pending timer rather than accumulate one per
// failure.
func TestManager_MultipleFailuresDoNotStackTimers(t *testing.T) {
	t.Parallel()

	mgr := newRetryTestManager(t, neverFiresBackoff)

	const name = "flaky"

	ctx := context.Background()

	for range 3 {
		mgr.ConnectAsync(ctx, unreachableHTTPServer(name))
		requireFailedState(t, mgr, name)
	}

	require.Equal(t, 1, pendingRetryCount(mgr),
		"repeated failures for the same server must not stack timers")
}

// TestManagerClose_StopsPendingRetriesCleanly proves Close stops every
// pending retry timer so nothing fires after the manager (and its
// DB/process) is torn down — no goroutine/timer leak.
func TestManagerClose_StopsPendingRetriesCleanly(t *testing.T) {
	t.Parallel()

	const backoff = 100 * time.Millisecond

	mgr := newRetryTestManager(t, backoff)

	const name = "closing"

	mgr.ConnectAsync(context.Background(), unreachableHTTPServer(name))
	requireFailedState(t, mgr, name)

	require.NoError(t, mgr.Close())

	require.Zero(t, pendingRetryCount(mgr),
		"Close must clear all pending retry timers")

	// Wait past the backoff window; if the timer had survived Close, it would
	// fire fireRetry here and (racily) touch mgr state — -race would catch a
	// concurrent unsynchronized access, and the generation bump in Close means
	// even a slipped-through callback is a guaranteed no-op. A fixed sleep is
	// correct here rather than an Eventually: the assertion is that NOTHING
	// happens, so waiting longer only strengthens it.
	time.Sleep(backoff + 300*time.Millisecond)

	require.Equal(t, StateFailed, mgr.Status(name).State,
		"no retry must have fired and changed status after Close")
}
