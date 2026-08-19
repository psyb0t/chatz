//go:build integration

package httpservertest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_ChatWorkflow walks the chat-organization surface end to end
// against the real stack: literal title search, pin/unpin (including the
// pinned-first ordering the sidebar depends on), then delete and the 404 that
// proves the row is gone.
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

	var pinned map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPut,
		ts.URL+pathChatsPrefix+chatID+"/pin",
		nil,
		&pinned,
	)
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, pinned["pinnedAt"])

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

	// The pinned chat leads regardless of which one saw activity last.
	items, ok := listed["items"].([]any)
	require.True(t, ok)
	firstChat, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, chatID, firstChat["id"])

	var unpinned map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodDelete,
		ts.URL+pathChatsPrefix+chatID+"/pin",
		nil,
		&unpinned,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Nil(t, unpinned["pinnedAt"])

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
