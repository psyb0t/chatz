//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/db"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_AdminReadiness(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	unauthenticated := newClient(t)
	status := requestJSON(
		t,
		unauthenticated,
		http.MethodGet,
		ts.URL+pathReadiness,
		nil,
		nil,
	)
	require.Equal(t, http.StatusUnauthorized, status)

	admin, _ := newAuthedClient(t, ts)
	member := newClient(t)
	status = requestJSON(
		t,
		admin,
		http.MethodPost,
		ts.URL+pathUsers,
		api.CreateUserJSONRequestBody{
			Password: "member-password",
			Username: "member",
		},
		nil,
	)
	require.Equal(t, http.StatusCreated, status)

	status = requestJSON(
		t,
		member,
		http.MethodPost,
		ts.URL+pathAuthLogin,
		map[string]string{
			"password": "member-password",
			"username": "member",
		},
		nil,
	)
	require.Equal(t, http.StatusOK, status)
	status = requestJSON(
		t,
		member,
		http.MethodGet,
		ts.URL+pathReadiness,
		nil,
		nil,
	)
	require.Equal(t, http.StatusForbidden, status)

	var readiness map[string]any

	status = requestJSON(
		t,
		admin,
		http.MethodGet,
		ts.URL+pathReadiness,
		nil,
		&readiness,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "test", readiness["appVersion"])
	assert.Equal(t, string(db.DriverPostgres), readiness["databaseDriver"])
	assert.Equal(t, false, readiness["migrationDirty"])
	backup, ok := readiness["backup"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "not_recorded", backup["state"])

	upstreams, ok := readiness["upstreams"].([]any)
	require.True(t, ok)
	require.Len(t, upstreams, 1)
	upstream, ok := upstreams[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "fake", upstream["upstream"])
	assert.NotContains(t, readiness, "backupStatusPath")
}
