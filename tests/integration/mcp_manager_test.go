//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/heartbeat"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

const (
	markerValue    = "marker-secret-42"
	readyTimeout   = 20 * time.Second
	connectTimeout = 60 * time.Second

	// retryRecoveryTimeout bounds how long the auto-retry recovery test
	// waits for the manager's own auto-retry to fire and succeed. It must
	// exceed the mcp package's real (unexported, un-shrinkable from this
	// external test package) production retryBackoff — 15s at the time of
	// writing — plus margin for the retry's own connect + tool-list round
	// trip.
	retryRecoveryTimeout = 45 * time.Second

	mcpURLFmt       = "http://127.0.0.1:%d/mcp"
	pythonCmd       = "python3"
	stdioServerName = "stdio"
)

// serverScript resolves tests/mcpserver/server.py relative to this test file,
// so it works regardless of the test's working directory.
func serverScript(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	script := filepath.Join(
		filepath.Dir(thisFile), "..", "mcpserver", "server.py",
	)
	require.FileExists(t, script)

	return script
}

func freePort(t *testing.T) int {
	t.Helper()

	lc := net.ListenConfig{}

	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	require.NoError(t, ln.Close())

	return addr.Port
}

// startHTTPServer launches the Python MCP server in streamable-HTTP mode and
// waits until its /mcp endpoint answers. Returns the running process so a
// test that needs to kill it mid-test (simulating an unexpected crash,
// rather than the normal end-of-test t.Cleanup teardown) can do so.
func startHTTPServer(t *testing.T, script string, port int) *exec.Cmd {
	t.Helper()

	//nolint:gosec,noctx // test fixture; process killed in t.Cleanup
	cmd := exec.Command(pythonCmd, script, "http", strconv.Itoa(port))
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	url := fmt.Sprintf(mcpURLFmt, port)
	deadline := time.Now().Add(readyTimeout)

	for time.Now().Before(deadline) {
		//nolint:gosec,noctx // readiness probe; body closed below
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()

			return cmd
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("http mcp server on :%d not ready within %s", port, readyTimeout)

	return cmd
}

func toolNames(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.QualifiedName())
	}

	return names
}

// addStdioServer registers the Python MCP server under stdio transport
// against mgr, optionally sealing envEnc onto the server row.
func addStdioServer(
	ctx context.Context,
	t *testing.T,
	mgr *mcp.Manager,
	script string,
	envEnc []byte,
) {
	t.Helper()

	argsJSON, err := json.Marshal([]string{script, stdioServerName})
	require.NoError(t, err)

	require.NoError(t, mgr.Add(ctx, &models.MCPServer{
		Name:      stdioServerName,
		Transport: models.MCPTransportStdio,
		Command:   pythonCmd,
		Args:      datatypes.JSON(argsJSON),
		EnvEnc:    envEnc,
	}))
}

// TestManager_BothTransports drives chatz's MCP client against a real Python
// MCP server over BOTH transports (stdio + streamable-HTTP) in one test:
// discovers tools across both, calls echo on each, and proves stdio env-secret
// injection via the marker tool.
func TestManager_BothTransports(t *testing.T) {
	box := testBox(t)
	script := serverScript(t)

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	mgr := mcp.NewManager(box)

	t.Cleanup(func() { _ = mgr.Close() })

	// stdio server + a sealed env secret (echoed back by the marker tool).
	markerEnv := map[string]string{"CHATZ_MCP_MARKER": markerValue}

	envEnc, err := box.SealMap(markerEnv)
	require.NoError(t, err)

	addStdioServer(ctx, t, mgr, script, envEnc)

	// --- streamable-HTTP (SSE over HTTP) server ---
	port := freePort(t)
	startHTTPServer(t, script, port)

	require.NoError(t, mgr.Add(ctx, &models.MCPServer{
		Name:      "http",
		Transport: models.MCPTransportHTTP,
		URL:       fmt.Sprintf(mcpURLFmt, port),
	}))

	// --- both servers' tools discovered, namespaced per server ---
	names := toolNames(mgr.Tools(ctx))
	assert.Contains(t, names, "stdio__echo")
	assert.Contains(t, names, "stdio__marker")
	assert.Contains(t, names, "http__echo")

	// --- per-server tools (unqualified names + input schema) for the admin
	// tools view; an unknown server yields an empty list, not an error ---
	stdioTools, err := mgr.ServerTools(ctx, stdioServerName)
	require.NoError(t, err)

	stdioNames := make([]string, 0, len(stdioTools))
	for _, tool := range stdioTools {
		stdioNames = append(stdioNames, tool.Name)

		if tool.Name == "echo" {
			assert.NotEmpty(t, tool.InputSchema,
				"echo tool should carry an input schema")
		}
	}

	assert.Contains(t, stdioNames, "echo")
	assert.Contains(t, stdioNames, "marker")

	unknown, err := mgr.ServerTools(ctx, "nope")
	require.NoError(t, err)
	assert.Empty(t, unknown)

	// --- echo works over each transport ---
	stdioEcho, err := mgr.Call(ctx, "stdio__echo", map[string]any{"text": "hi"})
	require.NoError(t, err)
	assert.False(t, stdioEcho.IsError)
	assert.Contains(t, stdioEcho.Text, "echo: hi")

	httpEcho, err := mgr.Call(ctx, "http__echo", map[string]any{"text": "yo"})
	require.NoError(t, err)
	assert.False(t, httpEcho.IsError)
	assert.Contains(t, httpEcho.Text, "echo: yo")

	// --- the sealed env secret reached the stdio subprocess ---
	marker, err := mgr.Call(ctx, "stdio__marker", nil)
	require.NoError(t, err)
	assert.Equal(t, markerValue, marker.Text)
}

// TestManager_Call_IdleHeartbeatFiresDuringSlowTool proves Manager.Call's
// idle-heartbeat actually fires while a real MCP tool call is still in
// flight — the fix for the live incident where a single blocking tool call
// with no intermediate progress signal produced zero log output for
// minutes, indistinguishable from a hang. Uses a real stdio MCP server (the
// sleep tool) rather than a fake Client, since mcp.Client wraps the SDK
// session directly with no seam to mock.
func TestManager_Call_IdleHeartbeatFiresDuringSlowTool(t *testing.T) {
	origInterval := heartbeat.Interval
	heartbeat.Interval = 300 * time.Millisecond

	t.Cleanup(func() { heartbeat.Interval = origInterval })

	// scope.GetLogger builds from slog.Default(), so capturing output means
	// swapping the process default for the test and restoring it after.
	buf := &bytes.Buffer{}

	previousLogger := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	box := testBox(t)
	script := serverScript(t)

	mgr := mcp.NewManager(box)

	t.Cleanup(func() { _ = mgr.Close() })

	addStdioServer(ctx, t, mgr, script, nil)

	// 1.2s of sleep against a 300ms heartbeat interval: several heartbeats
	// must fire before the call returns.
	res, err := mgr.Call(ctx, "stdio__sleep", map[string]any{"seconds": 1.2})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "slept:")

	heartbeats := 0

	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}

		var rec map[string]any

		require.NoError(t, json.Unmarshal([]byte(line), &rec))

		if rec["msg"] == "still waiting on tool call" {
			heartbeats++
		}
	}

	assert.GreaterOrEqual(t, heartbeats, 2,
		//nolint:lll // one assertion message; splitting the literal is banned
		"expected multiple idle heartbeats during the slow tool call, got %d\nlog:\n%s",
		heartbeats, buf.String())
}

// TestManager_AutoRetry_RecoversAfterServerComesUp proves the full
// self-healing loop end to end against a REAL MCP server, using the
// package's real (unexported, unshrinkable from outside the mcp package)
// production retryBackoff: a connect fails because nothing is listening on
// the target port yet, then — with NO test code calling ConnectAsync or
// Reconnect again — the manager's own retry timer fires on its own after the
// backoff and reconnects once a real streamable-HTTP MCP server starts
// listening on that exact port. This is the scenario the whole feature
// exists for: a self-hosted, unattended deployment where the dependency MCP
// server is slow to start or restarts transiently.
func TestManager_AutoRetry_RecoversAfterServerComesUp(t *testing.T) {
	box := testBox(t)
	script := serverScript(t)

	port := freePort(t)
	url := fmt.Sprintf(mcpURLFmt, port)

	// syncBuffer, not bytes.Buffer: ConnectAsync's retry runs on a
	// background goroutine that can still be logging (e.g. the NEXT
	// self-armed retry, scheduled right after this one succeeds) after the
	// test observes StateConnected and starts reading the log — a plain
	// bytes.Buffer would race under -race.
	buf := &syncBuffer{}

	previousLogger := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	ctx := context.Background()

	mgr := mcp.NewManager(box)

	t.Cleanup(func() { _ = mgr.Close() })

	const name = "heals-for-real"

	srv := &models.MCPServer{
		Name:      name,
		Transport: models.MCPTransportHTTP,
		URL:       url,
	}

	// Nothing is listening on `port` yet — this connect must fail.
	mgr.ConnectAsync(ctx, srv)

	waitForMCPState(t, mgr, name, mcp.StateFailed, connectTimeout)

	// Start the real server on the SAME port the failed attempt targeted,
	// well within the production retryBackoff window, then just wait — no
	// test code drives a second attempt.
	startHTTPServer(t, script, port)

	waitForMCPState(t, mgr, name, mcp.StateConnected, retryRecoveryTimeout)

	st := mgr.Status(name)
	assert.Equal(t, mcp.StateConnected, st.State)
	assert.Positive(t, st.ToolCount)

	assertLoggedMsgWithField(t, buf, "mcp retry firing", "server", name)
}

// TestManager_UnexpectedDeath_TransitionsToFailedAndAutoRecovers proves the
// Manager detects a server dying AFTER a successful connect — not just an
// initial connect failure — and self-heals via the same auto-retry path.
//
// Before this fix, an unexpected async death (in production: the vendored
// SDK's background standalone-SSE listener exhausting its own reconnect
// budget and closing the whole session) left the Manager's status stuck at
// StateConnected forever, so every subsequent call (ServerTools, Call) kept
// failing with no way for an unattended deployment to recover short of an
// admin manually clicking Reconnect.
func TestManager_UnexpectedDeath_TransitionsToFailedAndAutoRecovers(
	t *testing.T,
) {
	box := testBox(t)
	script := serverScript(t)

	port := freePort(t)
	url := fmt.Sprintf(mcpURLFmt, port)

	mgr := mcp.NewManager(box)

	t.Cleanup(func() { _ = mgr.Close() })

	const name = "dies-unexpectedly"

	srv := &models.MCPServer{
		Name:      name,
		Transport: models.MCPTransportHTTP,
		URL:       url,
	}

	cmd := startHTTPServer(t, script, port)

	require.NoError(t, mgr.Add(context.Background(), srv))
	require.Equal(t, mcp.StateConnected, mgr.Status(name).State)

	// Kill the server out from under the live connection — no graceful
	// close, matching an actual crash/restart/network-partition in prod —
	// rather than waiting for the test's own t.Cleanup teardown.
	require.NoError(t, cmd.Process.Kill())
	_ = cmd.Wait()

	waitForMCPState(t, mgr, name, mcp.StateFailed, connectTimeout)

	// Bring a fresh server back up on the SAME port, well within the
	// production retryBackoff window, then just wait — no test code drives
	// a second attempt.
	startHTTPServer(t, script, port)

	waitForMCPState(t, mgr, name, mcp.StateConnected, retryRecoveryTimeout)

	st := mgr.Status(name)
	assert.Equal(t, mcp.StateConnected, st.State)
	assert.Positive(t, st.ToolCount)
}

// waitForMCPState polls mgr.Status(name).State until it equals want or the
// timeout elapses.
func waitForMCPState(
	t *testing.T,
	mgr *mcp.Manager,
	name string,
	want mcp.State,
	timeout time.Duration,
) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if mgr.Status(name).State == want {
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("status for %q did not reach %q within %s (got %q)",
		name, want, timeout, mgr.Status(name).State)
}

// syncBuffer wraps bytes.Buffer with a mutex so it's safe to read from the
// test goroutine while the manager's background retry goroutines keep
// writing log lines to it concurrently (real background timers, not
// something the test can join on before reading).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// assertLoggedMsgWithField scans buf's JSON log lines for one whose msg
// matches and whose fields contain key=value, proving the manager's own
// retry-firing log line — not just the resulting status — actually emitted.
func assertLoggedMsgWithField(
	t *testing.T,
	buf *syncBuffer,
	msg string,
	key string,
	value string,
) {
	t.Helper()

	for _, line := range strings.Split(buf.String(), "\n") {
		if line == "" {
			continue
		}

		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}

		if rec["msg"] == msg && rec[key] == value {
			return
		}
	}

	t.Fatalf(
		"expected a log line %q with %s=%q, got:\n%s",
		msg,
		key,
		value,
		buf.String(),
	)
}
