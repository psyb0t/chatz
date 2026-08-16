//go:build real

// HTTP-transport helpers for the tool round-trip: the tool server is reached
// over streamable-HTTP (the shape of a real remote MCP like a hosted brain
// endpoint), instead of stdio. Wired into the shared table-driven
// TestReal_ToolCallRoundTrip in tooling_real_test.go as the "http" case.
package realtest

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/stretchr/testify/require"
)

const (
	httpEchoTool     = "http__echo"
	httpReadyTimeout = 20 * time.Second
)

// freePort grabs an unused localhost TCP port for the Python server.
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

// startPythonHTTP launches the Python MCP test server in streamable-HTTP mode
// and blocks until its /mcp endpoint answers.
func startPythonHTTP(t *testing.T, script string, port int) {
	t.Helper()

	//nolint:gosec,noctx // test fixture; process killed in t.Cleanup
	cmd := exec.Command("python3", script, "http", strconv.Itoa(port))
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())

	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	url := fmt.Sprintf("http://127.0.0.1:%d/mcp", port)
	deadline := time.Now().Add(httpReadyTimeout)

	for time.Now().Before(deadline) {
		//nolint:gosec,noctx // readiness probe; body closed immediately
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()

			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf(
		"http mcp server on :%d not ready within %s",
		port,
		httpReadyTimeout,
	)
}

// httpManager wires an MCP manager to the Python test server over HTTP.
func httpManager(
	ctx context.Context,
	t *testing.T,
	port int,
) *mcp.Manager {
	t.Helper()

	mgr := mcp.NewManager(testBox(t))
	t.Cleanup(func() { _ = mgr.Close() })

	require.NoError(t, mgr.Add(ctx, &models.MCPServer{
		Name:      "http",
		Transport: models.MCPTransportHTTP,
		URL:       fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
	}))

	return mgr
}
