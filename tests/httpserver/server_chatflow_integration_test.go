//go:build integration

package httpservertest

import (
	"net/http"
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestServer_ChatFlow drives the full stack: bootstrap admin -> create a chat
// (SSE stream, fake LLM) -> list -> load, asserting the SSE envelope and that
// the turn persisted a user + assistant message.
func TestServer_ChatFlow(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	// --- bootstrap admin (sets the session cookie) ---
	var user map[string]any

	status := requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		&user,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, adminUser, user["username"])
	assert.Equal(t, true, user["isAdmin"])

	// --- create a chat + stream the first turn ---
	status, ctype, body := requestStream(
		t, client,
		ts.URL+pathChats,
		map[string]string{"model": testModel, "message": "hi there"},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, ctype, "text/event-stream")
	assert.Contains(t, body, "message_start")
	assert.Contains(t, body, "stream_id")
	assert.Contains(t, body, fakeReply)
	assert.Contains(t, body, "message_stop")

	// --- list chats: paginated envelope ---
	var chatsEnv map[string]any

	requestJSON(t, client, http.MethodGet, ts.URL+pathChats, nil, &chatsEnv)

	chatItems, ok := chatsEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, chatItems, 1)
	assert.InDelta(t, float64(1), chatsEnv["total"], 0)
	assert.InDelta(t, float64(100), chatsEnv["limit"], 0)
	assert.InDelta(t, float64(0), chatsEnv["offset"], 0)

	chatSummary, ok := chatItems[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "hi there", chatSummary["title"])

	chatID, ok := chatSummary["id"].(string)
	require.True(t, ok)

	// --- get the chat: metadata only, no embedded messages ---
	var chat map[string]any

	requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+chatID, nil, &chat)

	assert.Equal(t, chatID, chat["id"])
	assert.Equal(t, "hi there", chat["title"])
	_, hasMessages := chat["messages"]
	assert.False(t, hasMessages, "getChat must not embed messages")

	// --- list the chat's messages: paginated sub-resource ---
	var msgsEnv map[string]any

	requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+chatID+"/messages", nil, &msgsEnv)

	msgs, ok := msgsEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, msgs, 2)
	assert.InDelta(t, float64(2), msgsEnv["total"], 0)
	assert.InDelta(t, float64(50), msgsEnv["limit"], 0)
	assert.InDelta(t, float64(0), msgsEnv["offset"], 0)

	userMsg, ok := msgs[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", userMsg["role"])
	assert.Equal(t, "hi there", userMsg["content"])

	asstMsg, ok := msgs[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "assistant", asstMsg["role"])
	assert.Equal(t, fakeReply, asstMsg["content"])
}

// TestServer_GetOrCreateEmptyChat covers the "New chat" dedup contract: a
// fresh empty chat is created and reused across repeated calls; it stays
// invisible to GET /chats until it gets its first message, at which point it
// both appears (titled from that message) AND unlocks a NEW empty chat for
// the next "New chat" call. Also proves sending in an OLDER chat bumps it to
// the top of the list (UpdatedAt-desc activity ordering).
func TestServer_GetOrCreateEmptyChat(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)
	bootstrapAdmin(t, client, ts)

	emptyURL := ts.URL + pathChatsPrefix + "empty"

	// --- repeated calls with nothing sent reuse the SAME empty chat ---
	var first, second map[string]any

	status := requestJSON(t, client, http.MethodPost, emptyURL, nil, &first)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "", first["title"])

	status = requestJSON(t, client, http.MethodPost, emptyURL, nil, &second)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, first["id"], second["id"],
		"repeated New chat must reuse the same empty chat")

	firstID, ok := first["id"].(string)
	require.True(t, ok)

	// --- the empty chat is invisible to history ---
	var chatsEnv map[string]any

	requestJSON(t, client, http.MethodGet, ts.URL+pathChats, nil, &chatsEnv)
	assert.InDelta(t, float64(0), chatsEnv["total"], 0,
		"an empty chat must not appear in history")

	// --- sending its first message titles it AND makes it visible ---
	status, _, body := requestStream(
		t, client,
		ts.URL+pathChatsPrefix+firstID,
		map[string]string{"model": testModel, "message": "hello there"},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, fakeReply)

	requestJSON(t, client, http.MethodGet, ts.URL+pathChats, nil, &chatsEnv)
	require.InDelta(t, float64(1), chatsEnv["total"], 0)

	items, ok := chatsEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)

	visible, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, firstID, visible["id"])
	assert.Equal(t, "hello there", visible["title"])

	// --- the now-non-empty chat no longer counts as "empty": a fresh New chat
	// call creates a DIFFERENT chat ---
	var third map[string]any

	status = requestJSON(t, client, http.MethodPost, emptyURL, nil, &third)
	require.Equal(t, http.StatusOK, status)
	assert.NotEqual(t, firstID, third["id"],
		"a non-empty chat must not be reused as the empty chat")

	thirdID, ok := third["id"].(string)
	require.True(t, ok)

	// --- sending a message in the OLDER (first) chat bumps it back to the
	// top ---
	status, _, _ = requestStream(
		t, client,
		ts.URL+pathChatsPrefix+thirdID,
		map[string]string{"model": testModel, "message": "second chat"},
	)
	require.Equal(t, http.StatusOK, status)

	status, _, _ = requestStream(
		t, client,
		ts.URL+pathChatsPrefix+firstID,
		map[string]string{
			"model": testModel, "message": "back to the first chat",
		},
	)
	require.Equal(t, http.StatusOK, status)

	requestJSON(t, client, http.MethodGet, ts.URL+pathChats, nil, &chatsEnv)
	items, ok = chatsEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)

	top, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, firstID, top["id"],
		"the chat just messaged in must sort first")
}

// TestServer_StreamedRoundRecordsUsage proves the usage recorder is wired into
// the chat loop: after a streamed turn, an llm_usage row lands attributed to
// the chat + user with the fake LLM's token counts.
func TestServer_StreamedRoundRecordsUsage(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	var user map[string]any

	status := requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		&user,
	)
	require.Equal(t, http.StatusOK, status)

	status, _, _ = requestStream(
		t, client,
		ts.URL+pathChats,
		map[string]string{"model": testModel, "message": "hi there"},
	)
	require.Equal(t, http.StatusOK, status)

	rows, err := repositories.LLMUsage.WithContext(t.Context()).Find()
	require.NoError(t, err)
	require.Len(t, rows, 1, "one streamed round -> one usage row")

	row := rows[0]
	assert.Equal(t, testModel, row.Model)
	assert.Equal(t, "chat", row.Stage)
	assert.Equal(t, int64(3), row.CompletionTokens)
	assert.Equal(t, int64(3), row.TotalTokens)
	require.NotNil(t, row.ChatID)
	require.NotNil(t, row.UserID)

	userID, ok := user["id"].(string)
	require.True(t, ok)
	assert.Equal(t, userID, row.UserID.String())

	var chatsEnv map[string]any
	requestJSON(t, client, http.MethodGet, ts.URL+pathChats, nil, &chatsEnv)

	chatItems, ok := chatsEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, chatItems, 1)

	chatSummary, ok := chatItems[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, chatSummary["id"], row.ChatID.String())
}
