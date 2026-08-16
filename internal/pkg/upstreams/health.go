package upstreams

import (
	"context"
	"errors"
	"sync"
	"time"
)

// HealthState is the current client-visible condition of one configured
// upstream. It records classes rather than provider text so diagnostics never
// retain prompts, headers, credentials, or response bodies.
type HealthState string

const (
	HealthStateUnknown  HealthState = "unknown"
	HealthStateHealthy  HealthState = "healthy"
	HealthStateDegraded HealthState = "degraded"
)

// Health is a snapshot of the latest operation outcome for one upstream.
type Health struct {
	Upstream           string
	State              HealthState
	LastOperation      string
	LastLatency        time.Duration
	LastSuccessAt      time.Time
	LastFailureAt      time.Time
	LastFailureClass   string
	ConsecutiveFailure int
}

// HealthTracker holds per-upstream snapshots. It is safe for concurrent stream
// callbacks and lets registry/discovery and runtime calls contribute to one
// source of truth.
type HealthTracker struct {
	mu        sync.RWMutex
	upstreams map[string]Health
}

// NewHealthTracker creates unknown snapshots for every configured upstream.
func NewHealthTracker(upstreamNames []string) *HealthTracker {
	tracker := &HealthTracker{
		upstreams: make(map[string]Health, len(upstreamNames)),
	}
	for _, name := range upstreamNames {
		tracker.upstreams[name] = Health{
			Upstream: name,
			State:    HealthStateUnknown,
		}
	}

	return tracker
}

// RecordSuccess records a completed idempotent or turn operation.
func (t *HealthTracker) RecordSuccess(
	upstream, operation string,
	latency time.Duration,
) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	health := t.current(upstream)
	health.State = HealthStateHealthy
	health.LastOperation = operation
	health.LastLatency = latency
	health.LastSuccessAt = time.Now().UTC()
	health.LastFailureClass = ""
	health.ConsecutiveFailure = 0
	t.upstreams[upstream] = health
}

// RecordFailure stores only a stable failure class, never raw provider text.
func (t *HealthTracker) RecordFailure(
	upstream, operation string,
	latency time.Duration,
	err error,
) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	health := t.current(upstream)
	health.State = HealthStateDegraded
	health.LastOperation = operation
	health.LastLatency = latency
	health.LastFailureAt = time.Now().UTC()
	health.LastFailureClass = failureClass(err)
	health.ConsecutiveFailure++
	t.upstreams[upstream] = health
}

// Snapshot returns one immutable health value.
func (t *HealthTracker) Snapshot(upstream string) (Health, bool) {
	if t == nil {
		return Health{}, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	health, ok := t.upstreams[upstream]

	return health, ok
}

// Snapshots returns known upstream health in caller-selected order.
func (t *HealthTracker) Snapshots(upstreamNames []string) []Health {
	if t == nil {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	health := make([]Health, 0, len(upstreamNames))
	for _, name := range upstreamNames {
		status, ok := t.upstreams[name]
		if !ok {
			continue
		}

		health = append(health, status)
	}

	return health
}

func (t *HealthTracker) current(upstream string) Health {
	health, ok := t.upstreams[upstream]
	if ok {
		return health
	}

	return Health{Upstream: upstream, State: HealthStateUnknown}
}

func failureClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, ErrFirstTokenTimeout):
		return "first_token_timeout"
	case errors.Is(err, ErrTurnTimeout):
		return "turn_timeout"
	case errors.Is(err, ErrDiscoveryTimeout):
		return "discovery_timeout"
	default:
		return "upstream_error"
	}
}
