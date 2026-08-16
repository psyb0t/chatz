package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	chatzlogging "github.com/psyb0t/chatz/internal/pkg/logging"
	"github.com/psyb0t/ctxscope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	modelsPath         = "/api/v1/models"
	upstreamHealthPath = "/api/v1/upstreams"
)

// captureDefaultLogger points slog.Default() at a JSON buffer for the duration
// of the test (restored via Cleanup). scope.GetLogger builds from
// slog.Default() unconditionally, so this captures the whole chain.
func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	prev := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return buf
}

// captureDefaultLoggerAtLevel is captureDefaultLogger's DEBUG-level sibling —
// the default JSON handler (nil opts) gates at INFO, which would silently
// drop the /healthz DEBUG line that test needs to see.
func captureDefaultLoggerAtLevel(t *testing.T) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	prev := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(
		buf, &slog.HandlerOptions{Level: slog.LevelDebug},
	)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return buf
}

// logLineWith returns the first captured JSON log line whose msg matches and
// that carries the given field, or nil.
func logLineWith(
	t *testing.T,
	buf *bytes.Buffer,
	msg, field string,
) map[string]any {
	t.Helper()

	lines := bytes.SplitSeq(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	for line := range lines {
		var rec map[string]any
		if json.Unmarshal(line, &rec) != nil {
			continue
		}

		if rec["msg"] == msg {
			if _, ok := rec[field]; ok {
				return rec
			}
		}
	}

	return nil
}

// findRequestCompletedLine scans captured JSON log lines for a "request
// completed" line whose route field matches, returning it (or nil).
func findRequestCompletedLine(
	t *testing.T,
	buf *bytes.Buffer,
	route string,
) map[string]any {
	t.Helper()

	lines := bytes.SplitSeq(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	for line := range lines {
		var rec map[string]any
		if json.Unmarshal(line, &rec) != nil {
			continue
		}

		if rec["msg"] == "request completed" && rec["route"] == route {
			return rec
		}
	}

	return nil
}

func newRequestIDEcho() *echo.Echo {
	e := echo.New()
	e.Use(requestID())
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	return e
}

// TestRouting_APIGroupBoundary proves the /api/v1 echo.Group wiring in New:
// every generated API route is reachable ONLY under /api/v1 (never bare), and
// /healthz is reachable ONLY bare at root (never under /api/v1) — the group
// prefix lives in server.go, not in api/api.yml's own servers.url (/v1).
func TestRouting_APIGroupBoundary(t *testing.T) {
	t.Parallel()

	srv := New(Deps{})

	// The real wire path (/api/v1/models) reaches the generated handler and
	// its middleware chain: no session cookie => 401 from requireUser, not a
	// 404 from unmatched routing.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, modelsPath, nil)
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"/api/v1/models should route to the handler (401, not 404)")

	// The generated admin health route is also protected by the regular
	// session middleware, before its admin authorization check can run.
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, upstreamHealthPath, nil)
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"/api/v1/upstreams should route to the handler (401, not 404)")

	// The bare, un-prefixed path must NOT exist: proves the group boundary is
	// real, not accidentally permissive (e.g. handlers double-registered on
	// both e and the group).
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/models", nil)
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"bare /models (no /api/v1 prefix) must 404")

	// /healthz is the one path that stays bare at root, outside the group.
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/healthz", nil)
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "bare /healthz should serve")

	// It must NOT also be reachable under the API group prefix.
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/v1/healthz", nil)
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code,
		"/api/v1/healthz must not exist: /healthz stays outside the API group")
}

// TestGetCtxWithIdentity_EnrichesLogger asserts the helper stamps user_id +
// is_admin onto the ctx logger so a downstream GetLogger(ctx) carries them.
func TestGetCtxWithIdentity_EnrichesLogger(t *testing.T) {
	buf := captureDefaultLogger(t)

	id := uuid.New()
	user := &models.User{Base: models.Base{ID: id}, IsAdmin: true}

	ctx := getCtxWithIdentity(t.Context(), user)
	ctxscope.GetLogger(ctx).Info("downstream")

	rec := logLineWith(t, buf, "downstream", chatzlogging.ScopeKeyUserID)
	require.NotNil(t, rec, "expected user_id on a downstream log line: %s", buf)
	assert.Equal(t, id.String(), rec[chatzlogging.ScopeKeyUserID])
	assert.Equal(t, true, rec[chatzlogging.ScopeKeyIsAdmin])
}

// TestRequestLogger_CarriesIdentity reproduces the real middleware composition:
// requestLogger (outer) wraps an inner middleware that authenticates + enriches
// the ctx logger via getCtxWithIdentity + c.SetRequest. The request-completed
// line MUST carry the identity — this is the "who made the request" contract.
func TestRequestLogger_CarriesIdentity(t *testing.T) {
	buf := captureDefaultLogger(t)

	id := uuid.New()

	e := echo.New()
	e.Use(requestLogger())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user := &models.User{Base: models.Base{ID: id}, IsAdmin: true}
			ctx := getCtxWithIdentity(c.Request().Context(), user)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	})
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	line := logLineWith(
		t, buf, "request completed", chatzlogging.ScopeKeyUserID,
	)
	require.NotNil(t, line,
		"request-completed line must carry user_id; got: %s", buf)
	assert.Equal(t, id.String(), line[chatzlogging.ScopeKeyUserID])
	assert.Equal(t, true, line[chatzlogging.ScopeKeyIsAdmin])
}

// TestRequestID_MintsWhenAbsent proves a request with no X-Request-Id header
// gets a freshly minted UUID, echoed back on the response.
func TestRequestID_MintsWhenAbsent(t *testing.T) {
	t.Parallel()

	e := newRequestIDEcho()

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/x", nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	got := rec.Header().Get(aichteeteapee.HeaderNameXRequestID)
	assert.True(t, isValidRequestID(got),
		"minted id must be a well-formed UUID, got %q", got)
}

// TestRequestID_EchoesValidIncomingID proves a well-formed incoming
// X-Request-Id (UUID or ULID) is echoed back unchanged, not replaced.
func TestRequestID_EchoesValidIncomingID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		id   string
	}{
		{"uuid v4", "550e8400-e29b-41d4-a716-446655440000"},
		{"ulid", "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := newRequestIDEcho()

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/x", nil,
			)
			req.Header.Set(aichteeteapee.HeaderNameXRequestID, tc.id)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(
				t, tc.id, rec.Header().Get(aichteeteapee.HeaderNameXRequestID),
				"a well-formed incoming id must be echoed back unchanged",
			)
		})
	}
}

// TestRequestID_RejectsGarbageShapedIncomingID proves an attacker-controlled
// header that doesn't look like a UUID/ULID is never trusted blindly — a
// fresh id is minted instead, and the garbage value never comes back on the
// response (would otherwise be a log-injection / arbitrary-value vector).
func TestRequestID_RejectsGarbageShapedIncomingID(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		id   string
	}{
		{"newline injection", "abc\ninjected-log-line"},
		{"absurd length", strings.Repeat("a", 5000)},
		{"not uuid or ulid shaped", "not-a-valid-id-at-all"},
		{"empty-ish whitespace", "   "},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e := newRequestIDEcho()

			req := httptest.NewRequestWithContext(
				t.Context(), http.MethodGet, "/x", nil,
			)
			req.Header.Set(aichteeteapee.HeaderNameXRequestID, tc.id)

			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)

			got := rec.Header().Get(aichteeteapee.HeaderNameXRequestID)
			assert.NotEqual(t, tc.id, got,
				"garbage-shaped incoming id must never be echoed back")
			assert.True(t, isValidRequestID(got),
				"a fresh, well-formed id must be minted instead, got %q", got)
		})
	}
}

// TestRequestID_EnrichesDownstreamLogger proves the middleware stamps
// request_id onto the ctx logger BEFORE requestLogger runs, so the
// request-completed line carries it — the whole point of ordering requestID
// first in the chain.
func TestRequestID_EnrichesDownstreamLogger(t *testing.T) {
	buf := captureDefaultLogger(t)

	e := echo.New()
	e.Use(requestID())
	e.Use(requestLogger())
	e.GET("/x", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, "/x", nil,
	)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	line := logLineWith(
		t, buf, "request completed", chatzlogging.ScopeKeyRequestID,
	)
	require.NotNil(t, line,
		"request-completed line must carry request_id; got: %s", buf)
	assert.Equal(
		t, rec.Header().Get(aichteeteapee.HeaderNameXRequestID),
		line[chatzlogging.ScopeKeyRequestID],
	)
}

// TestRequestLogger_HealthzLogsAtDebugNotInfo proves /healthz completions log
// at DEBUG (silenced at the LOG_LEVEL=info production default), while every
// other route still logs its completion at INFO as before.
func TestRequestLogger_HealthzLogsAtDebugNotInfo(t *testing.T) {
	buf := captureDefaultLoggerAtLevel(t)

	srv := New(Deps{})

	// /healthz: a DEBUG-level capture sees it, but it must be tagged DEBUG,
	// never INFO.
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, healthzPath, nil,
	)
	srv.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	healthzLine := findRequestCompletedLine(t, buf, healthzPath)
	require.NotNil(t, healthzLine,
		"expected a /healthz completion line: %s", buf)
	assert.Equal(t, "DEBUG", healthzLine["level"],
		"/healthz completions must log at DEBUG, not INFO")

	buf.Reset()

	// A real API route: still INFO (short-circuited 401 from requireUser,
	// but requestLogger still emits a completion line regardless of outcome).
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(
		t.Context(), http.MethodGet, modelsPath, nil,
	)
	srv.Handler().ServeHTTP(rec, req)

	apiLine := findRequestCompletedLine(t, buf, modelsPath)
	require.NotNil(t, apiLine,
		"expected an /api/v1/models completion line: %s", buf)
	assert.Equal(t, "INFO", apiLine["level"],
		"non-healthz routes must keep logging their completion at INFO")
}
