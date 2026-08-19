//go:build integration

package httpservertest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_ChatWorkflow walks the chat surface end to end against the real
// stack: literal title search narrows the list, the full list reports every
// seeded chat, then delete removes one and a follow-up GET 404s to prove the
// row is gone.
func TestServer_ChatWorkflow(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)
	chatIDs := seedChats(t, uuid.MustParse(adminID), 2)
	chatID := chatIDs[1]

	var searched map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChats+"?search=chat-00",
		nil,
		&searched,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(1), searched["total"], 0)

	var listed map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChats,
		nil,
		&listed,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(2), listed["total"], 0)

	status = requestJSON(
		t,
		client,
		http.MethodDelete,
		ts.URL+pathChatsPrefix+chatID,
		nil,
		nil,
	)
	assert.Equal(t, http.StatusNoContent, status)

	var missing map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChatsPrefix+chatID,
		nil,
		&missing,
	)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, errCodeNotFound, missing["code"])
}
