package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRedactingJSONLogger(buf *bytes.Buffer) *slog.Logger {
	inner := slog.NewJSONHandler(buf, nil)

	return slog.New(NewRedactingHandler(inner))
}

func decodeLastLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.NotEmpty(t, lines)

	var rec map[string]any
	require.NoError(t, json.Unmarshal(lines[len(lines)-1], &rec))

	return rec
}

// TestRedactingHandler_MasksHeadersGroupCarryingAuthorization proves the
// exact MCP-admin-response shape from the task: a field literally named
// "headers" whose nested Authorization value must never reach the sink in
// plaintext, while an unrelated field on the same record passes through.
func TestRedactingHandler_MasksHeadersGroupCarryingAuthorization(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := newRedactingJSONLogger(buf)

	logger.Info("mcp server connected",
		"server_name", "github",
		slog.Group("headers", "Authorization", "Bearer xyz"),
	)

	rec := decodeLastLine(t, buf)

	assert.Equal(t, "github", rec["server_name"],
		"normal field must pass through unredacted")

	headers, ok := rec["headers"].(map[string]any)
	require.True(t, ok,
		"headers group must still be present, just masked inside")
	assert.Equal(t, redactedValue, headers["Authorization"])
}

// TestRedactingHandler_MasksTopLevelSensitiveKeys covers each key pattern the
// mandate lists explicitly, at the top level (no group nesting).
func TestRedactingHandler_MasksTopLevelSensitiveKeys(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		key  string
	}{
		{"password", "password"},
		{"token", "token"},
		{"secret", "secret"},
		{"api_key snake", "api_key"},
		{"api-key kebab", "api-key"},
		{"apiKey camel", "apiKey"},
		{"authorization", "authorization"},
		{"cookie", "cookie"},
		{"mixed case", "Authorization"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buf := &bytes.Buffer{}
			logger := newRedactingJSONLogger(buf)

			logger.Info("event", tc.key, "super-secret-value")

			rec := decodeLastLine(t, buf)
			assert.Equal(t, redactedValue, rec[tc.key])
		})
	}
}

// TestRedactingHandler_PassesThroughNormalFields proves fields with no
// secret-shaped key name are left exactly as logged.
func TestRedactingHandler_PassesThroughNormalFields(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := newRedactingJSONLogger(buf)

	logger.Info("request completed",
		"method", "GET",
		"route", "/api/v1/chats",
		"status", 200,
		"duration_ms", int64(12),
	)

	rec := decodeLastLine(t, buf)
	assert.Equal(t, "GET", rec["method"])
	assert.Equal(t, "/api/v1/chats", rec["route"])
	assert.InDelta(t, float64(200), rec["status"], 0)
	assert.InDelta(t, float64(12), rec["duration_ms"], 0)
}

// TestRedactingHandler_WithAttrsRedactsBoundFields proves attrs bound via
// logger.With(...) (not just per-call attrs) are also redacted.
func TestRedactingHandler_WithAttrsRedactsBoundFields(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	base := newRedactingJSONLogger(buf)
	scoped := base.With("session_token", "abc123", "user_id", "u-1")

	scoped.Info("session validated")

	rec := decodeLastLine(t, buf)
	assert.Equal(t, redactedValue, rec["session_token"])
	assert.Equal(t, "u-1", rec["user_id"])
}

// TestRedactingHandler_EnabledDelegatesToInner proves the wrapper doesn't
// change the level gate — a DEBUG record is dropped when the inner handler is
// configured for INFO.
func TestRedactingHandler_EnabledDelegatesToInner(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	inner := slog.NewJSONHandler(
		buf, &slog.HandlerOptions{Level: slog.LevelInfo},
	)
	logger := slog.New(NewRedactingHandler(inner))

	logger.Debug("should not appear", "token", "abc")
	logger.Info("should appear", "token", "abc")

	out := buf.String()
	assert.NotContains(t, out, "should not appear")
	assert.Contains(t, out, "should appear")
	assert.Contains(t, out, redactedValue)
}

// TestRedactingHandler_WithGroupScopesInnerHandler proves WithGroup still
// nests attrs under the inner handler's group, and per-record attrs added
// under that group are still redacted.
func TestRedactingHandler_WithGroupScopesInnerHandler(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	logger := newRedactingJSONLogger(buf).WithGroup("mcp")

	logger.Info("connected", "token", "abc123", "server_name", "github")

	rec := decodeLastLine(t, buf)
	group, ok := rec["mcp"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, redactedValue, group["token"])
	assert.Equal(t, "github", group["server_name"])
}

// TestRedactingHandler_HandleDoesNotMutateCaller sanity-checks the redacted
// record is a fresh copy — logging through the handler concurrently from
// multiple goroutines must not race on shared state.
func TestRedactingHandler_HandleDoesNotMutateCaller(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	inner := slog.NewJSONHandler(buf, nil)
	handler := NewRedactingHandler(inner)

	ctx := context.Background()
	done := make(chan struct{})

	for range 10 {
		go func() {
			defer func() { done <- struct{}{} }()

			r := slog.NewRecord(time.Now(), slog.LevelInfo, "concurrent", 0)
			r.AddAttrs(slog.String("token", "x"), slog.String("ok", "y"))
			_ = handler.Handle(ctx, r)
		}()
	}

	for range 10 {
		<-done
	}
}

func TestRedactText_MasksNestedJSONSecrets(t *testing.T) {
	t.Parallel()

	got := RedactText(`{"request":{"api_key":"do-not-log","safe":"ok"}}`)

	assert.NotContains(t, got, "do-not-log")
	assert.Contains(t, got, redactedValue)
	assert.Contains(t, got, `"safe":"ok"`)
}

func TestRedactText_MasksPlainTextSecretAssignment(t *testing.T) {
	t.Parallel()

	got := RedactText("authorization: Bearer do-not-log; mode=debug")

	assert.NotContains(t, got, "do-not-log")
	assert.Contains(t, got, redactedValue)
	assert.Contains(t, got, "mode=debug")
}
