//go:build integration

package authtest

import (
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/core/auth"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUser = "admin"
	testPass = "s3cr3t-p@ss"
)

// These tests mutate the shared users/sessions tables, so they run serially
// (no t.Parallel) and each resets the tables first.

func newSvc(passwordless bool) *auth.Service {
	return auth.New(repositories.Q, passwordless)
}

func resetDB(t *testing.T) {
	t.Helper()

	err := testInfra.PG.GormDB.
		Exec("TRUNCATE users, sessions RESTART IDENTITY CASCADE").Error
	require.NoError(t, err)
}

func TestAuth_NeedsSetupAndBootstrap(t *testing.T) {
	resetDB(t)

	ctx := t.Context()
	svc := newSvc(false)

	need, err := svc.NeedsSetup(ctx)
	require.NoError(t, err)
	assert.True(t, need)

	user, err := svc.Bootstrap(ctx, testUser, testPass)
	require.NoError(t, err)
	assert.Equal(t, testUser, user.Username)
	assert.True(t, user.IsAdmin)
	require.NotNil(t, user.PasswordHash)

	need, err = svc.NeedsSetup(ctx)
	require.NoError(t, err)
	assert.False(t, need)

	// Setup is one-shot once a user exists.
	_, err = svc.Bootstrap(ctx, "other", "another-pass")
	require.ErrorIs(t, err, auth.ErrSetupClosed)
}

func TestAuth_BootstrapRequiresPassword(t *testing.T) {
	resetDB(t)

	_, err := newSvc(false).Bootstrap(t.Context(), testUser, "   ")
	require.ErrorIs(t, err, auth.ErrInvalidInput)
}

func TestAuth_LoginAuthenticateRoundTrip(t *testing.T) {
	resetDB(t)

	ctx := t.Context()
	svc := newSvc(false)

	created, err := svc.Bootstrap(ctx, testUser, testPass)
	require.NoError(t, err)

	token, user, err := svc.Login(ctx, testUser, testPass)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, created.ID, user.ID)

	got, err := svc.Authenticate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, testUser, got.Username)
}

func TestAuth_LoginRejectsBadCredentials(t *testing.T) {
	resetDB(t)

	ctx := t.Context()
	svc := newSvc(false)

	_, err := svc.Bootstrap(ctx, testUser, testPass)
	require.NoError(t, err)

	_, _, err = svc.Login(ctx, testUser, "wrong-password")
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)

	// Unknown user is indistinguishable from a bad password.
	_, _, err = svc.Login(ctx, "ghost", testPass)
	require.ErrorIs(t, err, auth.ErrInvalidCredentials)
}

func TestAuth_CreateUserRejectsDuplicate(t *testing.T) {
	resetDB(t)

	ctx := t.Context()
	svc := newSvc(false)

	_, err := svc.Bootstrap(ctx, testUser, testPass)
	require.NoError(t, err)

	_, err = svc.CreateUser(ctx, testUser, "different-pass", false)
	require.ErrorIs(t, err, auth.ErrUserExists)
}

func TestAuth_LogoutRevokesSession(t *testing.T) {
	resetDB(t)

	ctx := t.Context()
	svc := newSvc(false)

	_, err := svc.Bootstrap(ctx, testUser, testPass)
	require.NoError(t, err)

	token, _, err := svc.Login(ctx, testUser, testPass)
	require.NoError(t, err)

	_, err = svc.Authenticate(ctx, token)
	require.NoError(t, err)

	require.NoError(t, svc.Logout(ctx, token))

	_, err = svc.Authenticate(ctx, token)
	require.ErrorIs(t, err, auth.ErrSessionInvalid)

	// Logout is idempotent — revoking an already-gone session is fine.
	require.NoError(t, svc.Logout(ctx, token))
}

func TestAuth_ExpiredSessionRejected(t *testing.T) {
	resetDB(t)

	ctx := t.Context()
	svc := newSvc(false)

	_, err := svc.Bootstrap(ctx, testUser, testPass)
	require.NoError(t, err)

	token, user, err := svc.Login(ctx, testUser, testPass)
	require.NoError(t, err)

	// Force the session into the past, then it must be rejected.
	err = testInfra.PG.GormDB.
		Model(&models.Session{}).
		Where("user_id = ?", user.ID).
		Update("expires_at", time.Now().Add(-time.Hour)).Error
	require.NoError(t, err)

	_, err = svc.Authenticate(ctx, token)
	require.ErrorIs(t, err, auth.ErrSessionInvalid)
}

func TestAuth_Passwordless(t *testing.T) {
	resetDB(t)

	ctx := t.Context()
	svcPwd := newSvc(true)

	// No users yet — not a single-user install.
	_, _, err := svcPwd.PasswordlessLogin(ctx)
	require.ErrorIs(t, err, auth.ErrPasswordlessUnavailable)

	created, err := newSvc(false).Bootstrap(ctx, testUser, testPass)
	require.NoError(t, err)

	// Exactly one user + enabled — auto-login issues a working session.
	token, user, err := svcPwd.PasswordlessLogin(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, created.ID, user.ID)

	got, err := svcPwd.Authenticate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)

	// Disabled service never auto-logs-in, even with a single user.
	_, _, err = newSvc(false).PasswordlessLogin(ctx)
	require.ErrorIs(t, err, auth.ErrPasswordlessUnavailable)

	// A second user makes the install multi-user — no auto-login.
	_, err = newSvc(false).CreateUser(ctx, "second", "second-pass", false)
	require.NoError(t, err)

	_, _, err = svcPwd.PasswordlessLogin(ctx)
	require.ErrorIs(t, err, auth.ErrPasswordlessUnavailable)
}
