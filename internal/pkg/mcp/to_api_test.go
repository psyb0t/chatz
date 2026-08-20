package mcp

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestToolToAPI covers both projection branches: a tool with no input schema
// surfaces an empty (non-null) parameters object, and a populated schema is
// passed through unchanged.
func TestToolToAPI(t *testing.T) {
	t.Parallel()

	t.Run("nil schema becomes empty object", func(t *testing.T) {
		t.Parallel()

		got := ToolToAPI(Tool{Name: "ping", Description: "health check"})
		assert.Equal(t, "ping", got.Name)
		assert.Equal(t, "health check", got.Description)
		assert.Equal(t, map[string]any{}, got.Parameters)
	})

	t.Run("populated schema passes through", func(t *testing.T) {
		t.Parallel()

		schema := map[string]any{"type": "object"}
		got := ToolToAPI(Tool{Name: "query", InputSchema: schema})
		assert.Equal(t, schema, got.Parameters)
	})
}

func TestServerToAPIProjectsHealthTelemetry(t *testing.T) {
	t.Parallel()

	attemptedAt := time.Date(2026, time.August, 12, 2, 0, 0, 0, time.UTC)
	succeededAt := attemptedAt.Add(120 * time.Millisecond)
	failedAt := succeededAt.Add(time.Minute)
	server := &models.MCPServer{
		Base:      models.Base{ID: uuid.New()},
		Name:      "operations",
		Transport: models.MCPTransportHTTP,
		URL:       "https://mcp.example.test",
		Enabled:   true,
	}

	got := ServerToAPI(server, Status{
		State:                      StateConnected,
		ToolCount:                  4,
		LastConnectionAttemptAt:    attemptedAt,
		LastSuccessfulConnectionAt: succeededAt,
		LastConnectionFailureAt:    failedAt,
		LastConnectionLatency:      120 * time.Millisecond,
		LastError:                  "previous connection refused",
	})

	require.Equal(t, attemptedAt, *got.LastConnectionAttemptAt)
	require.Equal(t, succeededAt, *got.LastSuccessfulConnectionAt)
	require.Equal(t, failedAt, *got.LastConnectionFailureAt)
	require.Equal(t, 120, *got.LastConnectionLatencyMs)
	require.Equal(t, "previous connection refused", *got.LastError)
	require.Equal(t, 4, *got.ToolCount)
	require.Nil(t, got.StatusError)
}

func TestServerToAPIOmitsMissingHealthTelemetry(t *testing.T) {
	t.Parallel()

	got := ServerToAPI(&models.MCPServer{
		Base:      models.Base{ID: uuid.New()},
		Name:      "operations",
		Transport: models.MCPTransportHTTP,
		Enabled:   true,
	}, Status{State: StateConnecting})

	require.Nil(t, got.LastConnectionAttemptAt)
	require.Nil(t, got.LastSuccessfulConnectionAt)
	require.Nil(t, got.LastConnectionFailureAt)
	require.Nil(t, got.LastConnectionLatencyMs)
	require.Nil(t, got.LastError)
}
