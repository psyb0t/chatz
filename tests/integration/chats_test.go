//go:build integration

package integration

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
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

// TestServer_ChatOwnershipNotFound proves a chat owned by another user is
// indistinguishable from a missing one across the workflow endpoints: get,
// rename, and delete each return 404 rather than leaking that the chat exists.
func TestServer_ChatOwnershipNotFound(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, _ := newAuthedClient(t, ts)

	otherUser := &models.User{Username: "stranger", IsAdmin: false}
	require.NoError(
		t, repositories.User.WithContext(t.Context()).Create(otherUser),
	)

	foreignChat := &models.Chat{
		UserID: otherUser.ID, Title: "not yours", ModelID: testModel,
	}
	require.NoError(
		t, repositories.Chat.WithContext(t.Context()).Create(foreignChat),
	)

	fID := foreignChat.ID.String()

	status := requestJSON(
		t, client, http.MethodGet, ts.URL+pathChatsPrefix+fID, nil, nil,
	)
	assert.Equal(t, http.StatusNotFound, status)

	status = requestJSON(
		t, client, http.MethodPatch, ts.URL+pathChatsPrefix+fID,
		map[string]string{"title": "hijack"}, nil,
	)
	assert.Equal(t, http.StatusNotFound, status)

	status = requestJSON(
		t, client, http.MethodDelete, ts.URL+pathChatsPrefix+fID, nil, nil,
	)
	assert.Equal(t, http.StatusNotFound, status)

	// A wholly unknown id is likewise a 404 on delete.
	status = requestJSON(
		t, client, http.MethodDelete,
		ts.URL+pathChatsPrefix+uuid.New().String(), nil, nil,
	)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestServer_CreateChatErrors covers the create-chat validation branches: a
// blank model/message is a 400, and an unknown model is a 400 before any turn
// streams.
func TestServer_CreateChatErrors(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, _ := newAuthedClient(t, ts)

	var missing map[string]any

	status := requestJSON(
		t, client, http.MethodPost, ts.URL+pathChats,
		map[string]string{"message": "", "model": ""}, &missing,
	)
	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, errCodeBadRequest, missing["code"])

	var unknown map[string]any

	status = requestJSON(
		t, client, http.MethodPost, ts.URL+pathChats,
		map[string]string{"message": "hi", "model": "ghost-model"}, &unknown,
	)
	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, errCodeBadRequest, unknown["code"])
}

// TestServer_ContinueChatErrors covers the continue-chat failure branches: a
// blank message is a 400 and an unknown chat is a 404, both resolved before any
// turn streams.
func TestServer_ContinueChatErrors(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)
	chatID := seedChats(t, uuid.MustParse(adminID), 1)[0]

	var blank map[string]any

	status := requestJSON(
		t, client, http.MethodPost, ts.URL+pathChatsPrefix+chatID,
		map[string]string{"message": ""}, &blank,
	)
	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, errCodeBadRequest, blank["code"])

	var missing map[string]any

	status = requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathChatsPrefix+uuid.New().String(),
		map[string]string{"message": "hello"}, &missing,
	)
	require.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, errCodeNotFound, missing["code"])
}

// TestServer_UpdateChatSettings drives PATCH /chats/{id}/settings: a valid full
// settings body persists and echoes the projected settings, an out-of-range
// field fails validation (422), and an unknown chat 404s.
func TestServer_UpdateChatSettings(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)
	chatID := seedChats(t, uuid.MustParse(adminID), 1)[0]
	settingsURL := ts.URL + pathChatsPrefix + chatID + "/settings"

	var applied map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodPatch,
		settingsURL,
		map[string]any{
			"temperature":      0.7,
			"topP":             0.9,
			"reasoningEffort":  "high",
			"maxOutputTokens":  2048,
			"maxHistoryTokens": 8000,
		},
		&applied,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, 0.7, applied["temperature"], 0.0001)
	assert.Equal(t, "high", applied["reasoningEffort"])

	var invalid map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPatch,
		settingsURL,
		map[string]any{"temperature": 5.0},
		&invalid,
	)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, errCodeValidationFailed, invalid["code"])

	var missing map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPatch,
		ts.URL+pathChatsPrefix+uuid.New().String()+"/settings",
		map[string]any{"temperature": 0.5},
		&missing,
	)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, errCodeNotFound, missing["code"])
}

// TestServer_RenameChat drives the PATCH /chats/{id} rename endpoint through
// the real stack: a valid title renames and echoes the updated summary, a blank
// title is rejected before touching the store, and an unknown chat 404s.
func TestServer_RenameChat(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)
	chatID := seedChats(t, uuid.MustParse(adminID), 1)[0]

	var renamed map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodPatch,
		ts.URL+pathChatsPrefix+chatID,
		map[string]string{"title": "Renamed Chat"},
		&renamed,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "Renamed Chat", renamed["title"])

	var blank map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPatch,
		ts.URL+pathChatsPrefix+chatID,
		map[string]string{"title": "   "},
		&blank,
	)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, errCodeBadRequest, blank["code"])

	var missing map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPatch,
		ts.URL+pathChatsPrefix+uuid.New().String(),
		map[string]string{"title": "does not matter"},
		&missing,
	)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, errCodeNotFound, missing["code"])
}
