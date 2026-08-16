// Package heartbeat is a tiny idle-heartbeat helper for long blocking calls
// (an LLM stream between deltas, a single MCP tool call) that otherwise
// produce zero log output while waiting. Silence during a multi-minute wait is
// indistinguishable from a hang; a heartbeat every Interval turns that silence
// into an explicit "still waiting" signal.
package heartbeat

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/psyb0t/ctxscope"
)

// Interval is how often an idle heartbeat fires while Touch hasn't been
// called. Mutable for testing (shrunk so a heartbeat test doesn't eat real
// wall-clock time), mirroring the connectTimeout / connectOuterGrace
// mutable-for-testing pattern in internal/pkg/mcp/manager.go.
//
//nolint:gochecknoglobals // mutable for testing
var Interval = 30 * time.Second

// H ticks a background goroutine that logs a DEBUG "still waiting" line every
// Interval while the guarded call hasn't completed or produced activity.
// Touch resets the idle clock (call it on every unit of progress — a stream
// delta, a progress notification); Stop cancels the ticker goroutine cleanly.
// Safe for concurrent use: lastActivity is updated via atomic so the ticker
// goroutine reading it never races Touch's writer.
type H struct {
	cancel       context.CancelFunc
	done         chan struct{}
	lastActivity atomic.Int64 // unix nanos
}

// Start launches the idle-heartbeat goroutine, logging msg (plus attrs) via
// ctx's logger (scope.GetLogger) every Interval of inactivity. Stop must be
// called (typically deferred) once the guarded call returns, success or
// error, so the goroutine doesn't leak.
func Start(ctx context.Context, msg string, attrs ...any) *H {
	hctx, cancel := context.WithCancel(ctx)

	h := &H{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	h.lastActivity.Store(time.Now().UnixNano())

	go h.run(hctx, msg, attrs)

	return h
}

func (h *H) run(ctx context.Context, msg string, attrs []any) {
	defer close(h.done)

	logger := ctxscope.GetLogger(ctx)

	ticker := time.NewTicker(Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			idle := time.Since(time.Unix(0, h.lastActivity.Load()))
			if idle < Interval {
				continue
			}

			logger.Debug(msg, append(
				append([]any{}, attrs...), "idle_s", idle.Seconds(),
			)...)
		}
	}
}

// Touch resets the idle clock — call it on every unit of progress so the
// heartbeat only fires during genuinely idle stretches.
func (h *H) Touch() {
	h.lastActivity.Store(time.Now().UnixNano())
}

// Stop cancels the ticker goroutine and waits for it to exit, so no goroutine
// leaks past the guarded call's return.
func (h *H) Stop() {
	h.cancel()
	<-h.done
}
