//go:build integration

package httpservertest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_PreviewChatContext(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)
	budget := 100000
	chat := &models.Chat{
		UserID:           uuid.MustParse(adminID),
		Title:            "context preview",
		ModelID:          testModel,
		MaxHistoryTokens: &budget,
	}
	require.NoError(t, repositories.Chat.WithContext(t.Context()).Create(chat))

	turnComplete := true
	seedTurns(t, []*models.Message{
		{
			ChatID:       chat.ID,
			TurnID:       uuid.New(),
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleUser,
			Content:      "earlier question",
		},
		{
			ChatID:       chat.ID,
			TurnID:       uuid.New(),
			TurnComplete: &turnComplete,
			Role:         models.MessageRoleAssistant,
			Content:      "earlier answer",
		},
	})

	var preview map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+pathChatsPrefix+chat.ID.String()+"/context-preview",
		map[string]string{"message": "new draft"},
		&preview,
	)

	assert.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(budget), preview["budgetTokens"], 0)
	assert.Greater(t, preview["systemTokens"], float64(0))
	assert.Greater(t, preview["currentMessageTokens"], float64(0))
	assert.Greater(t, preview["totalTokens"], float64(0))
	assert.InDelta(t, float64(2), preview["retainedMessages"], 0)
	assert.InDelta(t, float64(2), preview["retainedTurns"], 0)
	assert.InDelta(t, float64(0), preview["omittedMessages"], 0)
	assert.InDelta(t, float64(0), preview["omittedTurns"], 0)
}

func TestServer_PreviewChatContext_OwnershipAndAuthentication(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, _ := newAuthedClient(t, ts)
	otherUser := &models.User{Username: "other", IsAdmin: false}
	require.NoError(
		t,
		repositories.User.WithContext(t.Context()).Create(otherUser),
	)
	chat := &models.Chat{
		UserID:  otherUser.ID,
		Title:   "not yours",
		ModelID: testModel,
	}
	require.NoError(t, repositories.Chat.WithContext(t.Context()).Create(chat))

	url := ts.URL + pathChatsPrefix + chat.ID.String() + "/context-preview"

	var env map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodPost,
		url,
		map[string]string{"message": "draft"},
		&env,
	)
	assert.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, errCodeNotFound, env["code"])

	status = requestJSON(
		t,
		newClient(t),
		http.MethodPost,
		url,
		map[string]string{"message": "draft"},
		&env,
	)
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, errCodeUnauthorized, env["code"])
}
