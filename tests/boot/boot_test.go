//go:build integration

// Package boot exercises the http-server service's real startup path in
// process: config parse, DB connect, upstream discovery, MCP connect, server
// assembly (New), then Run + Stop against a live listener. The api tier drives
// this same code through the built binary, but that binary is uninstrumented,
// so this in-process test is what credits the boot wiring toward coverage.
//
// It lives in its own package because New -> db.Connect calls
// repositories.SetDefault, re-pointing the global query set; keeping it out of
// the shared integration package avoids clobbering that suite's handle.
package boot

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	httpserver "github.com/psyb0t/chatz/internal/pkg/services/http-server"
	"github.com/psyb0t/chatz/tests/testinfra"
	_ "github.com/psyb0t/slogging/slogconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pgMappedPort = "5432"

//nolint:gochecknoglobals // shared across the package's boot test(s)
var testInfra *testinfra.Infra

func TestMain(m *testing.M) {
	ctx := context.Background()

	infra, err := testinfra.Setup(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "boot test infra setup failed: %v\n", err)
		os.Exit(1)
	}

	testInfra = infra
	code := m.Run()

	testInfra.Teardown(ctx)
	os.Exit(code)
}

// freeLoopbackAddr returns a currently-free 127.0.0.1 address to bind the HTTP
// listener the service boots on.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()

	lc := net.ListenConfig{}

	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	return addr
}

// TestBootLifecycle boots the real service against the testcontainer Postgres
// and proves the whole startup path works end to end: New assembles the server,
// Run serves an unauthenticated request, and Stop shuts the DB + MCP down
// cleanly. No upstream is configured, so the model registry is empty; the boot
// must succeed regardless (an install with no upstream still serves the UI and
// auth).
func TestBootLifecycle(t *testing.T) {
	ctx := t.Context()

	host, err := testInfra.PostgresContainer.Host(ctx)
	require.NoError(t, err)

	mapped, err := testInfra.PostgresContainer.MappedPort(ctx, pgMappedPort)
	require.NoError(t, err)

	httpAddr := freeLoopbackAddr(t)

	// Point config.Parse at the live testcontainer DB and a free HTTP port; the
	// observability listeners are off so they cannot collide with anything.
	t.Setenv("CHATZ_DB_HOSTNAME", host)
	t.Setenv("CHATZ_DB_PORT", mapped.Port())
	t.Setenv("CHATZ_DB_USERNAME", "test")
	t.Setenv("CHATZ_DB_PASSWORD", "test")
	t.Setenv("CHATZ_DB_NAME", "chatz")
	t.Setenv("CHATZ_HTTP_LISTENADDRESS", httpAddr)
	t.Setenv("CHATZ_METRICS_LISTENADDRESS", "")
	t.Setenv("CHATZ_PROFILING_LISTENADDRESS", "")

	svc, err := httpserver.New()
	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.Equal(t, "http-server", svc.Name())

	runCtx, cancel := context.WithCancel(context.Background())

	runErr := make(chan error, 1)

	go func() { runErr <- svc.Run(runCtx) }()

	statusURL := "http://" + httpAddr + "/api/v1/auth/status"

	require.Eventually(t, func() bool {
		req, reqErr := http.NewRequestWithContext(
			t.Context(), http.MethodGet, statusURL, nil,
		)
		if reqErr != nil {
			return false
		}

		resp, doErr := http.DefaultClient.Do(req)
		if doErr != nil {
			return false
		}

		defer func() { _ = resp.Body.Close() }()

		return resp.StatusCode == http.StatusOK
	}, 10*time.Second, 50*time.Millisecond, "service must serve /auth/status")

	cancel()
	require.NoError(t, <-runErr, "Run must return nil on clean shutdown")
	require.NoError(t, svc.Stop(context.Background()))
}

// TestBootWithUpstreamAndMCP boots with one configured upstream and one enabled
// MCP server row seeded in the DB, so the per-upstream client-build loop and
// the per-server MCP connect loop both run (both are skipped when nothing is
// configured). The upstream resolves to the in-test scripted driver and the
// MCP URL is unreachable, so neither touches the network; New must still
// assemble the server and Stop must tear it down cleanly.
func TestBootWithUpstreamAndMCP(t *testing.T) {
	ctx := t.Context()

	host, err := testInfra.PostgresContainer.Host(ctx)
	require.NoError(t, err)

	mapped, err := testInfra.PostgresContainer.MappedPort(ctx, pgMappedPort)
	require.NoError(t, err)

	seeded := &models.MCPServer{
		Name:      "boot-mcp",
		Transport: models.MCPTransportHTTP,
		URL:       "http://127.0.0.1:1/mcp",
		Enabled:   true,
	}
	require.NoError(t, testInfra.PG.GormDB.Create(seeded).Error)

	t.Cleanup(func() {
		testInfra.PG.GormDB.Exec(
			"TRUNCATE mcp_servers RESTART IDENTITY CASCADE",
		)
	})

	t.Setenv("CHATZ_DB_HOSTNAME", host)
	t.Setenv("CHATZ_DB_PORT", mapped.Port())
	t.Setenv("CHATZ_DB_USERNAME", "test")
	t.Setenv("CHATZ_DB_PASSWORD", "test")
	t.Setenv("CHATZ_DB_NAME", "chatz")
	t.Setenv("CHATZ_HTTP_LISTENADDRESS", freeLoopbackAddr(t))
	t.Setenv("CHATZ_METRICS_LISTENADDRESS", "")
	t.Setenv("CHATZ_PROFILING_LISTENADDRESS", "")

	// One upstream so buildUpstreamClients iterates; under test its driver
	// resolves to the scripted double, never a real provider.
	t.Setenv("BOOT_FAKE_KEY", "unused-under-test")
	t.Setenv(
		"CHATZ_UPSTREAMS",
		`[{"name":"fake","provider":"openai","apiKeyEnv":"BOOT_FAKE_KEY"}]`,
	)

	svc, err := httpserver.New()
	require.NoError(t, err)
	require.NotNil(t, svc)
	require.NoError(t, svc.Stop(context.Background()))
}
