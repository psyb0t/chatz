//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/core/chats/fixedresponses"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/psyb0t/essessey"
	essesseysse "github.com/psyb0t/essessey/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_ShowcaseModeInterceptsExactMessagesOnly(t *testing.T) {
	resetDB(t)

	llm := &fakeLLM{}
	ts := newShowcaseTestServer(t, llm)
	client, _ := newAuthedClient(t, ts)

	var models []map[string]any

	status := requestJSON(
		t, client, http.MethodGet, ts.URL+pathModels, nil, &models,
	)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, models, 1)
	assert.Equal(t, testModel, models[0]["id"])

	status, contentType, body := requestStream(
		t,
		client,
		ts.URL+pathChats,
		map[string]string{
			"model":   testModel,
			"message": fixedresponses.ShowcasePromptOperations,
		},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, contentType, "text/event-stream")
	assert.Contains(t, body, "thinking_delta")
	assert.Contains(t, body, "operations__get_service_health")
	assert.Contains(t, body, "operations__list_recent_events")
	assert.Contains(t, body, "message_stop")
	assert.Empty(t, llm.Requests(),
		"exact showcase prompt must not call the LLM")

	// The dashboard title streams as chunked text_delta events, so it can be
	// split mid-phrase across raw SSE frames — reconstruct the assembled
	// answer text before asserting on it, same as a real client would.
	parsed := essessey.Reassemble(
		t.Context(), essesseysse.NewSource(strings.NewReader(body)),
	)
	assert.Contains(t, parsed.Text, "REQUEST VOLUME")

	var chatsEnv map[string]any

	status = requestJSON(
		t, client, http.MethodGet, ts.URL+pathChats, nil, &chatsEnv,
	)
	require.Equal(t, http.StatusOK, status)

	chatItems, ok := chatsEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, chatItems, 1)
	chatSummary, ok := chatItems[0].(map[string]any)
	require.True(t, ok)
	chatID, ok := chatSummary["id"].(string)
	require.True(t, ok)

	status, _, body = requestStream(
		t,
		client,
		ts.URL+pathChatsPrefix+chatID,
		map[string]string{
			"message": fixedresponses.ShowcasePromptOperations + " ",
		},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, fakeReply)
	require.Len(t, llm.Requests(), 1,
		"a non-exact showcase prompt must use the configured LLM")

	var messagesEnv map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChatsPrefix+chatID+"/messages",
		nil,
		&messagesEnv,
	)
	require.Equal(t, http.StatusOK, status)

	messages, ok := messagesEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 4)
	assert.Equal(t, fixedresponses.ShowcasePromptOperations,
		messageContent(t, messages[0]))
	assert.Contains(t, messageContent(t, messages[1]), "REQUEST VOLUME")
	assert.Equal(t, fixedresponses.ShowcasePromptOperations+" ",
		messageContent(t, messages[2]))
	assert.Equal(t, fakeReply, messageContent(t, messages[3]))
}

// messageContent pulls the content field off one decoded message envelope,
// failing the test rather than panicking when the shape is not what the API
// documents.
func messageContent(t *testing.T, raw any) string {
	t.Helper()

	message, ok := raw.(map[string]any)
	require.True(t, ok, "message envelope must be an object")

	content, ok := message["content"].(string)
	require.True(t, ok, "message content must be a string")

	return content
}

func TestServer_ShowcasePersistsOneCompletedTurn(t *testing.T) {
	resetDB(t)

	ts := newShowcaseTestServer(t, &fakeLLM{})

	client, adminID := newAuthedClient(t, ts)
	chat := &models.Chat{
		UserID:  uuid.MustParse(adminID),
		Title:   "tool demo",
		ModelID: testModel,
	}
	require.NoError(
		t,
		repositories.Chat.WithContext(t.Context()).Create(chat),
	)

	chatRepo := repositories.Chat
	before, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.ID.Eq(chat.ID)).
		First()
	require.NoError(t, err)

	status, _, body := requestStream(
		t,
		client,
		ts.URL+pathChatsPrefix+chat.ID.String(),
		map[string]string{
			"model":   testModel,
			"message": fixedresponses.ShowcasePromptOperations,
		},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "message_stop")

	messageRepo := repositories.Message
	rows, err := messageRepo.WithContext(t.Context()).
		Where(messageRepo.ChatID.Eq(chat.ID)).
		Order(messageRepo.Position).
		Find()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, models.MessageRoleUser, rows[0].Role)
	assert.Equal(t, fixedresponses.ShowcasePromptOperations, rows[0].Content)
	assert.Equal(t, models.MessageRoleAssistant, rows[1].Role)
	assert.NotEmpty(t, rows[1].Content)
	assert.Equal(t, rows[0].TurnID, rows[1].TurnID)
	assert.NotEqual(t, uuid.Nil, rows[0].TurnID)

	for _, row := range rows {
		require.NotNil(t, row.TurnComplete)
		assert.True(t, *row.TurnComplete)
		assert.Equal(t, testModel, row.ModelID)
	}

	after, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.ID.Eq(chat.ID)).
		First()
	require.NoError(t, err)
	assert.True(t, after.UpdatedAt.After(before.UpdatedAt))
	assert.Equal(t, testModel, after.ModelID)
}

func TestServer_ShowcaseCompletionFailureRetainsVisibleUserMessage(
	t *testing.T,
) {
	resetDB(t)

	ts := newShowcaseTestServer(t, &fakeLLM{})

	client, adminID := newAuthedClient(t, ts)
	chat := &models.Chat{
		UserID:  uuid.MustParse(adminID),
		Title:   "failed tool demo",
		ModelID: testModel,
	}
	require.NoError(
		t,
		repositories.Chat.WithContext(t.Context()).Create(chat),
	)

	chatRepo := repositories.Chat
	before, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.ID.Eq(chat.ID)).
		First()
	require.NoError(t, err)

	installRejectAssistantTrigger(t)

	status, _, body := requestStream(
		t,
		client,
		ts.URL+pathChatsPrefix+chat.ID.String(),
		map[string]string{
			"model":   testModel,
			"message": fixedresponses.ShowcasePromptOperations,
		},
	)
	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "message_stop")

	messageRepo := repositories.Message
	rows, err := messageRepo.WithContext(t.Context()).
		Where(messageRepo.ChatID.Eq(chat.ID)).
		Find()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, models.MessageRoleUser, rows[0].Role)
	assert.Equal(t, fixedresponses.ShowcasePromptOperations, rows[0].Content)
	require.NotNil(t, rows[0].TurnComplete)
	assert.False(t, *rows[0].TurnComplete)
	assert.NotEqual(t, uuid.Nil, rows[0].TurnID)

	after, err := chatRepo.WithContext(t.Context()).
		Where(chatRepo.ID.Eq(chat.ID)).
		First()
	require.NoError(t, err)
	assert.True(t, after.UpdatedAt.After(before.UpdatedAt))

	var messagesEnv map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChatsPrefix+chat.ID.String()+"/messages",
		nil,
		&messagesEnv,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(1), messagesEnv["total"], 0)

	items, ok := messagesEnv["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)

	message, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", message["role"])
	assert.Equal(t, fixedresponses.ShowcasePromptOperations, message["content"])
}
