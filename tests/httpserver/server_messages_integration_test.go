//go:build integration

package httpservertest

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// TestServer_ListChatMessages_Ownership proves a missing chat and a
// wrong-owner chat both 404 (never leaking existence via a different status).
func TestServer_ListChatMessages_Ownership(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	// A chat owned by the caller: 200.
	ownChat := &models.Chat{
		UserID: uuid.MustParse(adminID), Title: "mine", ModelID: testModel,
	}
	require.NoError(
		t, repositories.Chat.WithContext(t.Context()).Create(ownChat),
	)

	var env map[string]any

	status := requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+ownChat.ID.String()+"/messages", nil, &env)
	assert.Equal(t, http.StatusOK, status)

	// A chat owned by a different user: 404, never leaking existence.
	otherUser := &models.User{Username: "other", IsAdmin: false}
	require.NoError(
		t, repositories.User.WithContext(t.Context()).Create(otherUser),
	)

	otherChat := &models.Chat{
		UserID: otherUser.ID, Title: "not yours", ModelID: testModel,
	}
	require.NoError(
		t, repositories.Chat.WithContext(t.Context()).Create(otherChat),
	)

	status = requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+otherChat.ID.String()+"/messages", nil, &env)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, errCodeNotFound, env["code"])

	// A chat id that does not exist at all: 404.
	missingID := uuid.New().String()

	status = requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+missingID+"/messages", nil, &env)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, errCodeNotFound, env["code"])
}

// TestServer_ListChatMessages_ReconstructsTurnStructure seeds a realistic
// two-round tool-calling turn directly at the repo layer (mirroring exactly
// what persistTurn writes) and proves GET /messages returns enough structure
// to reconstruct the same thinking/text/tool blocks the live SSE stream
// built: a reasoning trace on the assistant round, tool calls surviving on a
// round whose only output WAS the tool call (empty content — previously
// dropped entirely by the content-only visibility filter), and the matching
// tool-result's error flag. A stray role=system row (the shape of a
// tool-steering injection that could have been persisted before persistTurn
// started skipping them) must never appear — it isn't durable transcript
// content and would render as a fake user bubble.
func TestServer_ListChatMessages_ReconstructsTurnStructure(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	chat := &models.Chat{
		UserID: uuid.MustParse(adminID), Title: "t", ModelID: testModel,
	}
	require.NoError(t, repositories.Chat.WithContext(t.Context()).Create(chat))

	base := time.Now().Add(-time.Hour)
	msgRepo := repositories.Message
	turnID := uuid.New()
	turnComplete := true

	seed := func(m *models.Message, offset time.Duration) {
		t.Helper()

		m.ChatID = chat.ID
		require.NoError(t, msgRepo.WithContext(t.Context()).Create(m))
		_, err := msgRepo.WithContext(t.Context()).
			Where(msgRepo.ID.Eq(m.ID)).
			UpdateColumn(msgRepo.CreatedAt, base.Add(offset))
		require.NoError(t, err)
	}

	userMsg := &models.Message{
		TurnID:       turnID,
		TurnComplete: &turnComplete,
		Role:         models.MessageRoleUser,
		Content:      "search for X",
	}
	seed(userMsg, 0)

	toolCallRound := &models.Message{
		TurnID:       turnID,
		TurnComplete: &turnComplete,
		Role:         models.MessageRoleAssistant,
		Content:      "",
		Reasoning:    "Let me search first",
		ToolCalls: datatypes.JSON(
			[]byte(
				`[{"id":"call_1","name":"search","arguments":"{\"q\":\"X\"}"}]`,
			),
		),
	}
	seed(toolCallRound, time.Second)

	toolResult := &models.Message{
		TurnID:       turnID,
		TurnComplete: &turnComplete,
		Role:         models.MessageRoleTool,
		Content:      "search failed: timeout",
		ToolCallID:   "call_1",
		IsError:      true,
	}
	seed(toolResult, 2*time.Second)

	finalRound := &models.Message{
		TurnID:       turnID,
		TurnComplete: &turnComplete,
		Role:         models.MessageRoleAssistant,
		Content:      "Here's what I found",
	}
	seed(finalRound, 3*time.Second)

	// A stray tool-steering injection row — must never surface.
	injection := &models.Message{
		TurnID:       uuid.New(),
		TurnComplete: &turnComplete,
		Role:         elelem.RoleSystem,
		Content:      "internal reminder",
	}
	seed(injection, 4*time.Second)

	var env map[string]any

	status := requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+chat.ID.String()+"/messages", nil, &env)
	require.Equal(t, http.StatusOK, status)

	items, ok := env["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 4, "the system-role injection must be excluded")
	assert.InDelta(t, float64(4), env["total"], 0)

	for _, raw := range items {
		m, ok := raw.(map[string]any)
		require.True(t, ok)
		assert.NotEqual(t, "system", m["role"])
	}

	got, ok := items[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "assistant", got["role"])
	assert.Equal(t, "", got["content"])
	assert.Equal(t, "Let me search first", got["thinking"])
	calls, ok := got["toolCalls"].([]any)
	require.True(t, ok, "empty-content tool-call round must still be visible")
	require.Len(t, calls, 1)
	call, ok := calls[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "call_1", call["id"])
	assert.Equal(t, "search", call["name"])
	assert.Equal(t, `{"q":"X"}`, call["arguments"])

	got, ok = items[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "tool", got["role"])
	assert.Equal(t, "search failed: timeout", got["content"])
	assert.Equal(t, "call_1", got["toolCallId"])
	assert.Equal(t, true, got["isError"])

	got, ok = items[3].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "assistant", got["role"])
	assert.Equal(t, "Here's what I found", got["content"])
	assert.Nil(t, got["thinking"])
	assert.Nil(t, got["toolCalls"])
}

// TestServer_ListChatMessages_ReasoningOnlyRoundIsVisible proves the third,
// otherwise-untested leg of ListMessages' visibility filter against real
// Postgres: a round whose ONLY output was a reasoning trace (empty content,
// no tool calls — the model thought but decided neither to answer nor call a
// tool) must still survive the WHERE clause and come back, not get silently
// dropped like the pre-fix content-only filter would have done.
func TestServer_ListChatMessages_ReasoningOnlyRoundIsVisible(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	chat := &models.Chat{
		UserID: uuid.MustParse(adminID), Title: "t", ModelID: testModel,
	}
	require.NoError(t, repositories.Chat.WithContext(t.Context()).Create(chat))

	msg := &models.Message{
		ChatID:       chat.ID,
		TurnID:       uuid.New(),
		TurnComplete: new(true),
		Role:         models.MessageRoleAssistant,
		Content:      "",
		Reasoning:    "just pondering, nothing to say or call",
	}
	require.NoError(
		t, repositories.Message.WithContext(t.Context()).Create(msg),
	)

	var env map[string]any

	status := requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+chat.ID.String()+"/messages", nil, &env)
	require.Equal(t, http.StatusOK, status)

	items, ok := env["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1, "reasoning-only row must not be filtered out")
	assert.InDelta(t, float64(1), env["total"], 0)

	got, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "", got["content"])
	assert.Equal(t, "just pondering, nothing to say or call", got["thinking"])
}
