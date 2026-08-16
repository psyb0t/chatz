//go:build integration

package httpservertest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_UpstreamHealth returns only the configured runtime health fields
// after the registry's model-discovery probe succeeds.
func TestServer_UpstreamHealth(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	status := requestJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		nil,
	)
	require.Equal(t, http.StatusOK, status)

	var health []map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathUpstreams,
		nil,
		&health,
	)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, health, 1)
	assert.Equal(t, "fake", health[0]["upstream"])
	assert.Equal(t, "healthy", health[0]["state"])
	assert.Equal(t, "model_discovery", health[0]["lastOperation"])
	assert.NotContains(t, health[0], "error")
}
