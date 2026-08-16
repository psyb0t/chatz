package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withShortInterval shrinks Interval for the duration of a test so a heartbeat
// test doesn't eat real wall-clock time, and restores it on cleanup — mirrors
// internal/pkg/mcp/manager_reconnect_test.go's withShortConnectDeadlines
// pattern for the package-level connectTimeout/connectOuterGrace vars.
func withShortInterval(t *testing.T, interval time.Duration) {
	t.Helper()

	orig := Interval
	Interval = interval

	t.Cleanup(func() { Interval = orig })
}

// testMsg is the heartbeat message every test in this file scripts and
// counts — factored out so countHeartbeats doesn't need a msg parameter it
// would only ever receive one value for.
const testMsg = "still waiting on thing"

// logCapture swaps slog.Default() for a buffer-backed JSON logger, restoring it
// on cleanup. scope.GetLogger builds from slog.Default() — a context carries
// attributes, never a logger — so swapping the default is the only way to read
// what was emitted.
//
// That is process-wide state, which is why the tests in this file are not
// parallel: two swapping at once would each read the other's output.
func logCapture(t *testing.T) (context.Context, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}

	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return context.Background(), buf
}

func countHeartbeats(t *testing.T, buf *bytes.Buffer) int {
	t.Helper()

	count := 0

	for line := range strings.SplitSeq(buf.String(), "\n") {
		if line == "" {
			continue
		}

		var rec map[string]any

		require.NoError(t, json.Unmarshal([]byte(line), &rec))

		if rec["msg"] == testMsg {
			count++
		}
	}

	return count
}

// TestHeartbeat_FiresOnIdle proves the heartbeat logs a "still waiting" DEBUG
// line while no Touch call resets the idle clock — the actual missing piece
// from the live-log analysis: a long silent gap must not stay silent.
func TestHeartbeat_FiresOnIdle(t *testing.T) {
	withShortInterval(t, 20*time.Millisecond)

	ctx, buf := logCapture(t)

	h := Start(ctx, testMsg, "id", "x")

	time.Sleep(90 * time.Millisecond) // several intervals, no Touch
	h.Stop()

	count := countHeartbeats(t, buf)
	assert.GreaterOrEqual(t, count, 2,
		"expected multiple idle heartbeats over ~4 intervals, got %d\nlog:\n%s",
		count, buf.String())
}

// TestHeartbeat_TouchSuppressesHeartbeat proves Touch resets the idle clock so
// a heartbeat does NOT fire while activity is ongoing (e.g. deltas still
// streaming in) — the heartbeat is for IDLE gaps only, not a periodic no-op.
func TestHeartbeat_TouchSuppressesHeartbeat(t *testing.T) {
	// run() logs only once the idle gap REACHES Interval, so a scheduler
	// stall longer than Interval yields a heartbeat that is correct
	// behaviour, not a regression. The original shape -- Touch in a loop
	// with a fixed sleep, then assert zero -- trusted every sleep to return
	// on time, and failed with 2 heartbeats when this package ran alongside
	// the rest of the suite. The sleep still paces the loop, but is no
	// longer TRUSTED: measure the gaps actually achieved, and assert the
	// invariant only on a run that kept up.
	const (
		interval    = 100 * time.Millisecond
		touchEvery  = interval / 20
		runFor      = 3 * interval
		maxAttempts = 5
	)

	withShortInterval(t, interval)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, buf := logCapture(t)

		h := Start(ctx, testMsg)

		var widestGap time.Duration

		previousTouch := time.Now()
		deadline := previousTouch.Add(runFor)

		for time.Now().Before(deadline) {
			h.Touch()

			now := time.Now()
			if gap := now.Sub(previousTouch); gap > widestGap {
				widestGap = gap
			}

			previousTouch = now

			time.Sleep(touchEvery)
		}

		// The stretch between the last Touch and Stop is an idle gap too, so it
		// counts toward whether the clock stayed reset for the whole run.
		if gap := time.Since(previousTouch); gap > widestGap {
			widestGap = gap
		}

		h.Stop()

		if widestGap >= interval {
			t.Logf(
				"attempt %d/%d stalled: gap %s >= interval %s, retrying",
				attempt, maxAttempts, widestGap, interval)

			continue
		}

		assert.Zero(t, countHeartbeats(t, buf),
			//nolint:lll // single assert format string
			"every touch gap stayed under Interval (widest %s < %s), so no heartbeat may fire\nlog:\n%s",
			widestGap, interval, buf.String())

		return
	}

	t.Fatalf(
		"no attempt of %d kept gaps under %s: machine too loaded to test",
		maxAttempts, interval)
}

// TestHeartbeat_StopLeavesNoGoroutineRunning proves Stop cancels the ticker
// goroutine and waits for it to exit (not a leaked goroutine still ticking
// after the guarded call returns) — Stop blocks on the done channel, so by the
// time it returns, sleeping past several more intervals must produce no
// further heartbeat lines.
func TestHeartbeat_StopLeavesNoGoroutineRunning(t *testing.T) {
	withShortInterval(t, 15*time.Millisecond)

	ctx, buf := logCapture(t)

	h := Start(ctx, testMsg)

	time.Sleep(40 * time.Millisecond)
	h.Stop()

	countAtStop := countHeartbeats(t, buf)

	time.Sleep(60 * time.Millisecond) // several more intervals post-Stop

	countAfter := countHeartbeats(t, buf)
	assert.Equal(t, countAtStop, countAfter,
		"heartbeat kept firing after Stop returned — goroutine leaked")
}
