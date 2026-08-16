package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/heartbeat"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
)

// The three timings below are defaults for per-Manager FIELDS, not mutable
// package globals. They used to be globals that tests reassigned, which forced
// every test touching them to skip t.Parallel() to avoid clobbering a sibling
// — and still left a genuine data race with the parallel tests in the same
// package, since a background retry goroutine reads them while another test
// writes. A field is set once at construction and only read afterwards, which
// removes the whole class of problem and lets those tests run parallel again.
const (
	// defaultConnectTimeout bounds a single connect attempt so an unreachable
	// or hung server resolves to a failed status instead of blocking a
	// request forever.
	defaultConnectTimeout = 10 * time.Second

	// defaultConnectOuterGrace is added on top of the connect timeout before
	// ConnectAsync stops waiting on connectAndStore and force-settles the
	// status itself. connectAndStore's own ctx-bound work can be followed by
	// ctx-UNBOUND teardown (e.g. the MCP session's Close() blocking on a
	// subprocess's SIGTERM grace period, or a third-party transport that
	// doesn't select on ctx for every blocking I/O call) — this is the hard
	// backstop guaranteeing the recorded status leaves StateConnecting within
	// a bounded time no matter what the inner call does. See ConnectAsync.
	defaultConnectOuterGrace = 5 * time.Second

	// defaultRetryBackoff is the fixed delay between an applied connect
	// failure and the automatic retry attempt for that server. A self-hosted
	// deployment runs unattended, so a transiently-slow or restarting
	// dependency MCP server must heal on its own instead of sitting at
	// StateFailed until an admin clicks Reconnect. Fixed (not exponential) is
	// a deliberate simplicity choice: every MCP server here is either a local
	// sidecar process or a same-network HTTP endpoint, both of which come back
	// within seconds of a restart, not minutes — there's no remote-service
	// backoff-storm risk to guard against here, and a fixed interval keeps the
	// scheduling/cancellation logic (and its test) trivial to reason about.
	defaultRetryBackoff = 15 * time.Second
)

// State is a server's live connection state, surfaced to the admin UI.
type State string

const (
	StateConnecting State = "connecting"
	StateConnected  State = "connected"
	StateFailed     State = "failed"
	StateDisabled   State = "disabled"
)

// Reason classifies WHY a connect failed so the UI can say "unreachable" /
// "access denied" / "not responding" instead of a raw error blob.
type Reason string

const (
	ReasonUnreachable   Reason = "unreachable"
	ReasonAccessDenied  Reason = "access_denied"
	ReasonNotResponding Reason = "not_responding"
	ReasonFailed        Reason = "failed"
)

// Status is a server's last-known connection status (keyed by server name).
type Status struct {
	State                      State
	Reason                     Reason
	Error                      string
	ToolCount                  int
	LastConnectionAttemptAt    time.Time
	LastSuccessfulConnectionAt time.Time
	LastConnectionFailureAt    time.Time
	LastConnectionLatency      time.Duration
	LastError                  string
}

// Manager holds live connections to multiple MCP servers, aggregates their
// tools (namespaced <server>__<tool>), and routes a qualified tool call back to
// the owning server. Safe for concurrent use.
type Manager struct {
	box      *secrets.Box
	mu       sync.RWMutex
	clients  map[string]*Client
	statuses map[string]Status

	// connectTimeout / connectOuterGrace / retryBackoff are set once by
	// NewManager and never written again, so the background connect and retry
	// goroutines can read them without synchronisation. Tests override them
	// straight after construction, before anything is connected.
	connectTimeout    time.Duration
	connectOuterGrace time.Duration
	retryBackoff      time.Duration

	// generations tracks the most recently started connect attempt per server
	// name. ConnectAsync bumps it before launching a goroutine and captures its
	// own generation number; a late-arriving result from a superseded attempt
	// checks its generation against the current one before applying a status,
	// so an abandoned goroutine can never clobber a newer Reconnect's outcome.
	generations map[string]uint64

	// retryMu guards retries, separate from mu so scheduling/cancelling a
	// retry timer never contends with the (potentially slow, lock-held)
	// connect path.
	retryMu sync.Mutex
	// retries holds the pending auto-retry timer per server name, keyed so a
	// new attempt (manual Reconnect, an edit-triggered rewire, Disable,
	// Remove, or another scheduled retry) can cancel/replace any timer
	// already pending for that name — never two retry chains stacked for the
	// same server.
	retries map[string]*time.Timer
}

// NewManager builds an empty manager. box decrypts each server's env/header
// secrets at connect time.
func NewManager(box *secrets.Box) *Manager {
	return &Manager{
		box:               box,
		clients:           make(map[string]*Client),
		statuses:          make(map[string]Status),
		generations:       make(map[string]uint64),
		retries:           make(map[string]*time.Timer),
		connectTimeout:    defaultConnectTimeout,
		connectOuterGrace: defaultConnectOuterGrace,
		retryBackoff:      defaultRetryBackoff,
	}
}

// Add connects to srv synchronously, registering it under its name (replacing
// and closing any existing client) and recording its status. A failure is
// recorded as a failed status AND returned so callers that want the error get
// it. Prefer ConnectAsync from request handlers so a slow server can't block.
//
// Bumps the server's generation first so this attempt always wins the status
// write over any in-flight ConnectAsync goroutine for the same name — Add is
// synchronous and its result is authoritative for its caller.
func (m *Manager) Add(ctx context.Context, srv *models.MCPServer) error {
	m.cancelRetry(ctx, srv.Name)

	gen := m.bumpGeneration(srv.Name)
	attemptStartedAt := time.Now().UTC()

	st, err := m.connectAndStore(ctx, srv, gen, attemptStartedAt)
	m.setStatusIfCurrent(srv.Name, gen, st)

	if err != nil {
		return ctxerrors.Wrapf(err, "add mcp server %q", srv.Name)
	}

	return nil
}

// ConnectAsync marks the server connecting and connects in the background,
// so the caller's request returns immediately and the recorded status settles
// to connected/failed on its own.
//
// The status is GUARANTEED to leave StateConnecting within connectTimeout +
// connectOuterGrace, regardless of how the inner connect attempt behaves.
// connectAndStore's own ctx (bounded by connectTimeout) relies on the MCP SDK
// selecting on ctx.Done() for its I/O, which it does for the read/response
// path — but ctx-UNBOUND work can still follow (e.g. Close() waiting out a
// stuck subprocess's SIGTERM grace period). Rather than trust every layer to
// honor ctx, this races connectAndStore against cctx's own deadline with an
// explicit select: if the deadline wins, the status is force-settled to
// failed/not_responding immediately and the inner attempt is left to finish
// (or leak) in the background, logged as a WARN.
//
// A monotonic per-server generation number, bumped here before the goroutine
// starts, guards against a late result from an abandoned attempt clobbering a
// newer one: if a Reconnect (or another ConnectAsync/Add) supersedes this
// attempt before it finishes, its result — early or late — is discarded.
func (m *Manager) ConnectAsync(ctx context.Context, srv *models.MCPServer) {
	name := srv.Name

	m.cancelRetry(ctx, name)

	gen := m.bumpGeneration(name)
	attemptStartedAt := time.Now().UTC()

	m.setStatusIfCurrent(name, gen, Status{
		State:                   StateConnecting,
		LastConnectionAttemptAt: attemptStartedAt,
	})

	bg := context.WithoutCancel(ctx)
	logger := ctxscope.GetLogger(bg)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("mcp connect panic",
					"server", name, "panic", r, "reason", "mcp_connect_panic")
			}
		}()

		m.runConnectAttempt(bg, logger, srv, gen, attemptStartedAt)
	}()
}

// runConnectAttempt races connectAndStore against the outer connectTimeout+
// connectOuterGrace deadline (see ConnectAsync's doc comment for why) and
// applies whichever settles first.
func (m *Manager) runConnectAttempt(
	bg context.Context,
	logger *slog.Logger,
	srv *models.MCPServer,
	gen uint64,
	attemptStartedAt time.Time,
) {
	name := srv.Name

	cctx, cancel := context.WithTimeout(bg, m.connectTimeout)
	defer cancel()

	resultCh := make(chan connectResult, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("mcp connect inner panic",
					"server", name,
					"panic", r,
					"reason", "mcp_connect_inner_panic")
			}
		}()

		st, err := m.connectAndStore(cctx, srv, gen, attemptStartedAt)
		resultCh <- connectResult{status: st, err: err}
	}()

	select {
	case res := <-resultCh:
		m.applyConnectResult(bg, logger, srv, gen, res)
	case <-time.After(m.connectTimeout + m.connectOuterGrace):
		m.forceFailOuterDeadline(
			bg,
			logger,
			srv,
			gen,
			attemptStartedAt,
			resultCh,
		)
	}
}

// forceFailOuterDeadline runs when a connect attempt's inner call hasn't
// settled within connectTimeout+connectOuterGrace — it force-records
// StateFailed immediately and schedules a retry, then lets the inner
// goroutine finish (or leak) in the background; applyConnectResult's
// generation check discards its eventual result rather than scheduling a
// second retry on top of the one already armed here.
func (m *Manager) forceFailOuterDeadline(
	bg context.Context,
	logger *slog.Logger,
	srv *models.MCPServer,
	gen uint64,
	attemptStartedAt time.Time,
	resultCh <-chan connectResult,
) {
	name := srv.Name

	logger.Warn(
		"mcp connect exceeded outer deadline, force-failing status",
		"server", name,
		"timeout", m.connectTimeout,
		"grace", m.connectOuterGrace,
		"reason", "mcp_connect_outer_deadline",
	)

	outerBound := m.connectTimeout + m.connectOuterGrace
	forced := failedConnectionStatus(
		attemptStartedAt,
		ReasonNotResponding,
		fmt.Sprintf("connect timed out after %s", outerBound),
	)

	if m.setStatusIfCurrent(name, gen, forced) {
		m.scheduleRetry(bg, srv, gen)
	}

	go func() {
		res := <-resultCh
		m.applyConnectResult(bg, logger, srv, gen, res)
	}()
}

// connectResult carries connectAndStore's outcome across the inner goroutine
// that ConnectAsync races against its outer deadline.
type connectResult struct {
	status Status
	err    error
}

// applyConnectResult records a connect attempt's outcome, guarded by
// setStatusIfCurrent so a superseded/late attempt never clobbers a newer one.
// On an APPLIED failure it schedules an automatic retry (see scheduleRetry) so
// a self-hosted, unattended deployment heals from a transiently-slow or
// restarting dependency MCP server without an admin clicking Reconnect.
func (m *Manager) applyConnectResult(
	bg context.Context,
	logger *slog.Logger,
	srv *models.MCPServer,
	gen uint64,
	res connectResult,
) {
	name := srv.Name
	applied := m.setStatusIfCurrent(name, gen, res.status)

	if !applied {
		logger.Warn(
			"mcp connect result discarded, superseded by a newer attempt",
			"server", name, "reason", "mcp_connect_superseded",
		)

		return
	}

	if res.err != nil {
		logger.Warn("mcp connect failed",
			"server", name, "err", res.err, "reason", "mcp_connect_failed")

		m.scheduleRetry(bg, srv, gen)

		return
	}

	logger.Info("mcp connected", "server", name, "tools", res.status.ToolCount)
}

// connectAndStore dials srv, stores the live client (replacing any prior one),
// and returns the resulting status. The error mirrors the status so both the
// sync (Add) and async (ConnectAsync) callers can use it.
//
// gen is the generation this connect attempt is for — passed through to the
// background death-watcher (see watchClientDeath) so a later, unexpected
// death of the client this call stores is only acted on if gen is still
// current when that happens.
func (m *Manager) connectAndStore(
	ctx context.Context,
	srv *models.MCPServer,
	gen uint64,
	attemptStartedAt time.Time,
) (Status, error) {
	client, err := Connect(ctx, srv, m.box)
	if err != nil {
		return failedConnectionStatus(
			attemptStartedAt,
			classifyConnectErr(err),
			err.Error(),
		), err
	}

	tools, terr := client.ListTools(ctx)
	if terr != nil {
		ctxscope.GetLogger(ctx).Warn("mcp connected but list tools failed",
			"server", srv.Name, "err", terr, "reason", "list_tools_failed")
	}

	m.mu.Lock()

	if old, ok := m.clients[srv.Name]; ok {
		if cerr := old.Close(); cerr != nil {
			ctxscope.GetLogger(ctx).Warn("close replaced mcp client",
				"server", srv.Name, "err", cerr, "reason", "replaced")
		}
	}

	m.clients[srv.Name] = client
	m.mu.Unlock()

	go m.watchClientDeath(context.WithoutCancel(ctx), srv, gen, client)

	return successfulConnectionStatus(attemptStartedAt, len(tools)), nil
}

// watchClientDeath blocks on client's underlying session until it closes,
// then — ONLY if that close was NOT a deliberate Close() call — treats it as
// an unexpected async failure and runs the exact same generation-guarded
// fail+retry path a failed initial connect takes (see applyConnectResult).
//
// Without this, the Manager has no way to learn that a server it already
// recorded as StateConnected has silently died in the background: e.g. the
// vendored MCP SDK's streamable-HTTP transport gives up on its background
// SSE listener after exceeding its own retry budget and closes the whole
// session. m.clients/m.statuses would then stay stuck showing "connected"
// forever, and every call through that dead client (ServerTools, Call) keeps
// failing until an admin notices and clicks Reconnect. This closes that gap
// so a self-hosted, unattended deployment heals here too, matching
// scheduleRetry's doc comment.
func (m *Manager) watchClientDeath(
	bg context.Context,
	srv *models.MCPServer,
	gen uint64,
	client *Client,
) {
	err := client.Wait()

	if client.WasIntentionalClose() {
		return
	}

	name := srv.Name
	logger := ctxscope.GetLogger(bg)

	reason := ReasonFailed
	msg := "mcp connection closed by server"

	if err != nil {
		reason = classifyConnectErr(err)
		msg = err.Error()
	}

	failed := failedConnectionStatus(time.Time{}, reason, msg)

	if !m.setStatusIfCurrent(name, gen, failed) {
		logger.Debug("mcp connection death discarded, superseded",
			"server", name, "reason", "superseded")

		return
	}

	logger.Warn("mcp connection died unexpectedly",
		"server", name, "err", err, "reason", "mcp_connection_died")

	m.scheduleRetry(bg, srv, gen)
}

// Remove disconnects a server (if connected) and forgets its status
// entirely — used when a server is deleted.
//
// Bumps the generation first so a ConnectAsync attempt already in flight for
// this name (started before the delete) can never resurrect a status after
// removal, early or late. Also cancels any pending auto-retry — a deleted
// server must never reconnect on its own.
func (m *Manager) Remove(ctx context.Context, name string) {
	m.bumpGeneration(name)
	m.cancelRetry(ctx, name)

	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.clients[name]; ok {
		if err := c.Close(); err != nil {
			ctxscope.GetLogger(ctx).Warn("close removed mcp client",
				"server", name, "err", err, "reason", "removed")
		}

		delete(m.clients, name)
	}

	delete(m.statuses, name)
}

// Disable disconnects a server and records it as disabled — used when a
// server is edited to enabled=false (the row stays, its tools go away).
//
// Bumps the generation first for the same reason as Remove: a superseded
// in-flight connect attempt must not overwrite the disabled status. Also
// cancels any pending auto-retry — a disabled server must never reconnect on
// its own.
func (m *Manager) Disable(ctx context.Context, name string) {
	m.bumpGeneration(name)
	m.cancelRetry(ctx, name)

	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.clients[name]; ok {
		if err := c.Close(); err != nil {
			ctxscope.GetLogger(ctx).Warn("close disabled mcp client",
				"server", name, "err", err, "reason", "disabled")
		}

		delete(m.clients, name)
	}

	disabled := Status{State: StateDisabled}
	m.statuses[name] = mergeStatus(m.statuses[name], disabled)
}

// Status returns the last-known status for a server name. An unknown server
// yields the zero Status (State ""), which the API layer maps by enabled-ness.
func (m *Manager) Status(name string) Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.statuses[name]
}

// bumpGeneration starts a new attempt generation for name and returns it.
// Callers capture the returned number and pass it to setStatusIfCurrent so a
// status write from an older, superseded attempt is discarded rather than
// clobbering whatever the newer attempt (or Remove/Disable) already decided.
func (m *Manager) bumpGeneration(name string) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.generations[name]++

	return m.generations[name]
}

// setStatusIfCurrent writes st for name only if gen still matches the
// server's latest generation (i.e. no newer ConnectAsync/Add/Remove/Disable
// call has superseded this attempt since it started). Returns whether the
// write was applied.
func (m *Manager) setStatusIfCurrent(name string, gen uint64, st Status) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.generations[name] != gen {
		return false
	}

	m.statuses[name] = mergeStatus(m.statuses[name], st)

	return true
}

func successfulConnectionStatus(
	attemptStartedAt time.Time,
	toolCount int,
) Status {
	completedAt := time.Now().UTC()
	latency := connectionLatency(attemptStartedAt, completedAt)

	return Status{
		State:                      StateConnected,
		ToolCount:                  toolCount,
		LastConnectionAttemptAt:    attemptStartedAt,
		LastSuccessfulConnectionAt: completedAt,
		LastConnectionLatency:      latency,
	}
}

func failedConnectionStatus(
	attemptStartedAt time.Time,
	reason Reason,
	message string,
) Status {
	completedAt := time.Now().UTC()
	latency := connectionLatency(attemptStartedAt, completedAt)

	return Status{
		State:                   StateFailed,
		Reason:                  reason,
		Error:                   message,
		LastConnectionAttemptAt: attemptStartedAt,
		LastConnectionFailureAt: completedAt,
		LastConnectionLatency:   latency,
		LastError:               message,
	}
}

func connectionLatency(
	startedAt time.Time,
	completedAt time.Time,
) time.Duration {
	if startedAt.IsZero() {
		return 0
	}

	return completedAt.Sub(startedAt)
}

func mergeStatus(previous Status, next Status) Status {
	if next.LastConnectionAttemptAt.IsZero() {
		next.LastConnectionAttemptAt = previous.LastConnectionAttemptAt
	}

	if next.LastSuccessfulConnectionAt.IsZero() {
		next.LastSuccessfulConnectionAt = previous.LastSuccessfulConnectionAt
	}

	if next.LastConnectionFailureAt.IsZero() {
		next.LastConnectionFailureAt = previous.LastConnectionFailureAt
	}

	if next.LastConnectionLatency == 0 {
		next.LastConnectionLatency = previous.LastConnectionLatency
	}

	if next.LastError == "" {
		next.LastError = previous.LastError
	}

	return next
}

// scheduleRetry arms a one-shot timer that re-attempts ConnectAsync for srv
// after retryBackoff, replacing any timer already pending for srv.Name so a
// server never has two retry chains stacked. gen is the generation that was
// current when the failure this retry is compensating for was recorded — the
// fired timer re-checks it (see the retry firing below) so a retry scheduled
// for an attempt that's since been superseded (manual Reconnect, edit,
// disable, delete) silently no-ops instead of reconnecting a server the admin
// has already acted on, and — critically — does NOT reschedule itself, which
// is what naturally terminates a superseded retry chain.
//
// bg is the connect's already-detached (context.WithoutCancel) background
// ctx — reused here so the retry's own logging carries the same trace
// lineage as the failure that triggered it, and so the retry's eventual
// ConnectAsync call isn't tied to a request ctx that may already be gone.
func (m *Manager) scheduleRetry(
	bg context.Context,
	srv *models.MCPServer,
	gen uint64,
) {
	name := srv.Name
	logger := ctxscope.GetLogger(bg)

	logger.Debug("mcp retry scheduled",
		"server", name,
		"attempt", gen,
		"backoff", m.retryBackoff,
		"reason", "connect_failed",
	)

	m.retryMu.Lock()
	defer m.retryMu.Unlock()

	if existing, ok := m.retries[name]; ok {
		existing.Stop()
	}

	m.retries[name] = time.AfterFunc(m.retryBackoff, func() {
		m.retryMu.Lock()
		delete(m.retries, name)
		m.retryMu.Unlock()

		m.fireRetry(bg, srv, gen)
	})
}

// fireRetry runs when a scheduled retry's backoff elapses. It re-validates
// that the server is STILL StateFailed at the SAME generation that was
// current when the retry was scheduled before reconnecting — if a manual
// Reconnect, edit, disable, or delete happened in the meantime (bumping the
// generation, per bumpGeneration's callers), this retry is stale and must
// silently no-op rather than fire a redundant/conflicting connect attempt or
// reschedule itself on top of whatever superseded it.
func (m *Manager) fireRetry(
	bg context.Context,
	srv *models.MCPServer,
	gen uint64,
) {
	name := srv.Name
	logger := ctxscope.GetLogger(bg)

	m.mu.RLock()
	curGen := m.generations[name]
	curState := m.statuses[name].State
	m.mu.RUnlock()

	if curGen != gen || curState != StateFailed {
		logger.Debug("mcp retry discarded",
			"server", name, "reason", "superseded")

		return
	}

	logger.Info("mcp retry firing", "server", name, "attempt", gen)

	m.ConnectAsync(bg, srv)
}

// cancelRetry stops and forgets any pending auto-retry timer for name.
// time.Timer.Stop does not guarantee an already-fired callback is prevented
// from running, but that race is harmless here: fireRetry re-validates the
// generation/state before doing anything, so a callback that slips through
// this Stop() call still safely no-ops once bumpGeneration (called by every
// path that reaches cancelRetry: Add, ConnectAsync, Disable, Remove) has
// moved the generation forward.
func (m *Manager) cancelRetry(ctx context.Context, name string) {
	m.retryMu.Lock()
	defer m.retryMu.Unlock()

	timer, ok := m.retries[name]
	if !ok {
		return
	}

	timer.Stop()
	delete(m.retries, name)

	ctxscope.GetLogger(ctx).Debug("mcp retry cancelled",
		"server", name, "reason", "superseded")
}

// classifyConnectErr buckets a connect error into a coarse Reason for the UI.
// Best-effort string matching — the SDK/transport errors aren't typed.
func classifyConnectErr(err error) Reason {
	s := strings.ToLower(err.Error())

	switch {
	case strings.Contains(s, "connection refused"),
		strings.Contains(s, "no such host"),
		strings.Contains(s, "no route to host"):
		return ReasonUnreachable
	case strings.Contains(s, "401"),
		strings.Contains(s, "403"),
		strings.Contains(s, "unauthorized"),
		strings.Contains(s, "forbidden"):
		return ReasonAccessDenied
	case strings.Contains(s, "deadline exceeded"),
		strings.Contains(s, "timeout"),
		strings.Contains(s, "timed out"):
		return ReasonNotResponding
	default:
		return ReasonFailed
	}
}

// Tools aggregates the tools across all registered servers, sorted by qualified
// name. Best-effort: a server that fails to list is warned + skipped so one bad
// server doesn't hide the rest.
func (m *Manager) Tools(ctx context.Context) []Tool {
	logger := ctxscope.GetLogger(ctx)
	snapshot := m.snapshot()

	var (
		all    []Tool
		failed int
	)

	for name, client := range snapshot {
		tools, err := client.ListTools(ctx)
		if err != nil {
			logger.Warn("mcp list tools failed, skipping server",
				"server", name, "err", err, "reason", "list_tools_failed")

			failed++

			continue
		}

		all = append(all, tools...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].QualifiedName() < all[j].QualifiedName()
	})

	logger.Info("mcp tools aggregated",
		"servers", len(snapshot), "tools", len(all), "failed", failed)

	return all
}

// ServerTools lists a connected server's tools (unqualified names +
// description + input schema). A server that is not connected yields nil,
// nil — the admin UI shows tools for a live server only.
func (m *Manager) ServerTools(
	ctx context.Context,
	name string,
) ([]Tool, error) {
	m.mu.RLock()
	client, ok := m.clients[name]
	m.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	tools, err := client.ListTools(ctx)
	if err != nil {
		return nil, ctxerrors.Wrapf(err, "list tools for mcp server %q", name)
	}

	return tools, nil
}

// Call routes a qualified tool name (<server>__<tool>) to its server and
// invokes it with args. Logs start/end with timing, and heartbeats every
// heartbeat.Interval while the call is still in flight — CallTool is a single
// blocking round trip to (potentially) a slow external server with no
// intermediate progress signal, so without a heartbeat a slow tool looks
// identical to a hang in the logs.
//
// Only argument KEY NAMES are logged, never values — tool-call arguments can
// carry arbitrary (and potentially sensitive) caller-supplied data.
func (m *Manager) Call(
	ctx context.Context,
	qualifiedName string,
	args map[string]any,
) (*ToolResult, error) {
	logger := ctxscope.GetLogger(ctx)

	server, tool, ok := splitToolName(qualifiedName)
	if !ok {
		return nil, ctxerrors.Wrapf(
			ErrInvalidToolName, "name %q", qualifiedName,
		)
	}

	m.mu.RLock()
	client, ok := m.clients[server]
	m.mu.RUnlock()

	if !ok {
		return nil, ctxerrors.Wrapf(ErrServerNotFound, "server %q", server)
	}

	logger.Debug("mcp tool call started",
		"server", server,
		"tool", tool,
		"arg_keys", slices.Sorted(maps.Keys(args)))

	start := time.Now()

	hb := heartbeat.Start(ctx, "still waiting on tool call",
		"server", server, "tool", tool)

	result, err := client.CallTool(ctx, tool, args)

	hb.Stop()

	duration := time.Since(start)

	if err != nil {
		logger.Warn("mcp tool call failed",
			"server", server, "tool", tool,
			"duration_ms", duration.Milliseconds(),
			"reason", "tool_call_error", "err", err)

		return nil, err
	}

	logger.Debug("mcp tool call finished",
		"server", server, "tool", tool,
		"duration_ms", duration.Milliseconds(),
		"is_error", result.IsError,
		"result_len", len(result.Text))

	return result, nil
}

// Close disconnects every registered server, joining any close errors, and
// stops every pending auto-retry timer so nothing fires after the manager
// (and the DB/process it's part of) is torn down.
func (m *Manager) Close() error {
	m.stopAllRetries()

	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			errs = append(errs, ctxerrors.Wrapf(err, "close %q", name))
		}
	}

	m.clients = make(map[string]*Client)

	// Bump every name's generation too: time.Timer.Stop does not guarantee
	// an already-fired callback is prevented from running (see cancelRetry),
	// so this is the belt-and-suspenders backstop — even a callback that
	// slips through Stop() sees a superseded generation in fireRetry and
	// no-ops instead of reconnecting after shutdown.
	for name := range m.generations {
		m.generations[name]++
	}

	return errors.Join(errs...)
}

// stopAllRetries stops every pending auto-retry timer and clears the map.
func (m *Manager) stopAllRetries() {
	m.retryMu.Lock()
	defer m.retryMu.Unlock()

	for _, timer := range m.retries {
		timer.Stop()
	}

	m.retries = make(map[string]*time.Timer)
}

func (m *Manager) snapshot() map[string]*Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]*Client, len(m.clients))
	maps.Copy(out, m.clients)

	return out
}

func splitToolName(qualified string) (string, string, bool) {
	server, tool, ok := strings.Cut(qualified, toolNameSep)
	if !ok || server == "" || tool == "" {
		return "", "", false
	}

	return server, tool, true
}
