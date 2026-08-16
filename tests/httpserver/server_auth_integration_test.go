//go:build integration

package httpservertest

import (
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/core/auth"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_DeleteUser covers the admin user-delete flow: an admin removes a
// second user (204, DB-cascades their sessions/chats), cannot delete their own
// account (400), and a missing id is 404.
func TestServer_DeleteUser(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	var admin map[string]any

	status := requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		&admin,
	)
	require.Equal(t, http.StatusOK, status)

	adminID, ok := admin["id"].(string)
	require.True(t, ok)

	// Provision a second user to delete.
	var second map[string]any

	status = requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathUsers,
		map[string]string{"username": "second", "password": "second-pass-xyz"},
		&second,
	)
	require.Equal(t, http.StatusCreated, status)

	secondID, ok := second["id"].(string)
	require.True(t, ok)

	// Delete the second user -> 204, then the list shows only the admin.
	status = requestJSON(t, client, http.MethodDelete,
		ts.URL+pathUsers+"/"+secondID, nil, nil)
	require.Equal(t, http.StatusNoContent, status)

	var users []map[string]any

	requestJSON(t, client, http.MethodGet, ts.URL+pathUsers, nil, &users)
	require.Len(t, users, 1)
	assert.Equal(t, adminUser, users[0]["username"])

	// An admin cannot delete their own account.
	status = requestJSON(t, client, http.MethodDelete,
		ts.URL+pathUsers+"/"+adminID, nil, nil)
	assert.Equal(t, http.StatusBadRequest, status)

	// Deleting an already-gone user is 404.
	status = requestJSON(t, client, http.MethodDelete,
		ts.URL+pathUsers+"/"+secondID, nil, nil)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestServer_SessionCookieIsHttpOnly asserts the minted session cookie is
// HttpOnly + SameSite=Lax — JS can't read the token, and cross-site requests
// don't carry it (the app's CSRF posture).
func TestServer_SessionCookieIsHttpOnly(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)

	req := buildReq(t, http.MethodPost, ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword})

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var session *http.Cookie

	for _, c := range resp.Cookies() {
		if c.Name == auth.CookieName {
			session = c
		}
	}

	require.NotNil(t, session, "expected a session cookie on setup")
	assert.True(t, session.HttpOnly, "session cookie must be HttpOnly")
	assert.Equal(t, http.SameSiteLaxMode, session.SameSite)
	assert.Equal(t, "/", session.Path)
}

// TestServer_RequiresAuth confirms a protected endpoint rejects an anonymous
// caller with the error envelope.
func TestServer_RequiresAuth(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)

	var env map[string]any

	status := requestJSON(t, &http.Client{}, http.MethodGet,
		ts.URL+pathChats, nil, &env)
	require.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, errCodeUnauthorized, env["code"])
}

// TestServer_PasswordlessSingleUserAutoLogin proves the wired auto-login: with
// passwordless enabled AND exactly one user, GET /auth/status mints a session
// for the sole admin and sets the cookie — no login screen.
func TestServer_PasswordlessSingleUserAutoLogin(t *testing.T) {
	resetDB(t)

	ts := newTestServerWith(t, true)

	_, err := auth.New(repositories.Q, false).
		Bootstrap(t.Context(), adminUser, adminPassword)
	require.NoError(t, err)

	body, gotCookie := authStatus(t, ts)
	assert.Equal(t, true, body["authenticated"])
	assert.True(t, gotCookie, "expected a session cookie to be minted")

	user, ok := body["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, adminUser, user["username"])
}

// TestServer_PasswordlessSecondUserNoAutoLogin proves a second user disables
// auto-login: even with passwordless enabled, the install is multi-user, so an
// anonymous caller stays unauthenticated with no cookie.
func TestServer_PasswordlessSecondUserNoAutoLogin(t *testing.T) {
	resetDB(t)

	ts := newTestServerWith(t, true)

	svc := auth.New(repositories.Q, false)
	_, err := svc.Bootstrap(t.Context(), adminUser, adminPassword)
	require.NoError(t, err)
	_, err = svc.CreateUser(t.Context(), "second", "second-pass-xyz", false)
	require.NoError(t, err)

	body, gotCookie := authStatus(t, ts)
	assert.Equal(t, false, body["authenticated"])
	assert.False(t, gotCookie, "multi-user install must not auto-login")
}

// TestServer_PasswordlessDisabledNoAutoLogin proves that with passwordless off,
// a single-user install still requires an explicit login.
func TestServer_PasswordlessDisabledNoAutoLogin(t *testing.T) {
	resetDB(t)

	ts := newTestServerWith(t, false)

	_, err := auth.New(repositories.Q, false).
		Bootstrap(t.Context(), adminUser, adminPassword)
	require.NoError(t, err)

	body, gotCookie := authStatus(t, ts)
	assert.Equal(t, false, body["authenticated"])
	assert.False(t, gotCookie, "passwordless disabled must not auto-login")
}

// TestServer_LogsCarryUserIdentity drives the REAL request path (sessionAuth ->
// enriched ctx logger -> oapi strict handler -> requestLogger) and asserts the
// per-request log line says WHO made the request: the auth middleware must
// stamp user_id + is_admin onto the request-scoped logger for every authed
// request.
func TestServer_LogsCarryUserIdentity(t *testing.T) {
	resetDB(t)

	// Capture the whole server's structured logs for this (serial) test.
	buf := &syncBuf{}
	prev := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ts := newTestServer(t)
	client := newClient(t)

	var admin map[string]any

	status := requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		&admin,
	)
	require.Equal(t, http.StatusOK, status)

	adminID, ok := admin["id"].(string)
	require.True(t, ok)

	// An authed request drives the identity middleware.
	status = requestJSON(t, client, http.MethodGet,
		ts.URL+pathAuthStatus, nil, nil)
	require.Equal(t, http.StatusOK, status)

	// requestLogger writes just after the response returns — poll for it.
	var line map[string]any

	require.Eventually(t, func() bool {
		line = findIdentityLog(buf)

		return line != nil
	}, 2*time.Second, 10*time.Millisecond,
		"request-completed line must carry user_id; got: %s", buf)

	assert.Equal(t, adminID, line["user_id"])
	assert.Equal(t, true, line["is_admin"])
}
