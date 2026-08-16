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

// TestServer_ListChatsPagination walks every page of a 13-chat dataset with
// limit=5 (partial last page), asserting per-page item ids AND that total
// stays 13 at every offset — the canonical multi-page pagination test.
func TestServer_ListChatsPagination(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	wantIDs := seedChats(t, uuid.MustParse(adminID), 13)

	const limit = 5

	assertPaginates(t, client, ts.URL+pathChats, wantIDs, limit)
}

// TestServer_ListChatMessagesPagination walks every page of a 13-message
// chat with limit=5 (partial last page), asserting per-page item ids AND
// that total stays 13 at every offset.
func TestServer_ListChatMessagesPagination(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	chat := &models.Chat{
		UserID: uuid.MustParse(adminID), Title: "t", ModelID: testModel,
	}
	require.NoError(t, repositories.Chat.WithContext(t.Context()).Create(chat))

	wantIDs := seedMessages(t, chat.ID, 13)

	const limit = 5

	base := ts.URL + pathChatsPrefix + chat.ID.String() + "/messages"
	assertPaginates(t, client, base, wantIDs, limit)
}

// TestServer_ListChats_ValidationBoundary proves garbage limit/offset 400s,
// for both the chats list and the messages sub-resource list. A well-formed
// but out-of-range value (negative/zero/over-max limit, negative offset)
// carries the VALIDATION_FAILED envelope; a structurally malformed value
// (non-numeric) never reaches the range check — it's rejected by the
// generated query-param binder and carries the generic BAD_REQUEST envelope.
func TestServer_ListChats_ValidationBoundary(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	chat := &models.Chat{
		UserID: uuid.MustParse(adminID), Title: "t", ModelID: testModel,
	}
	require.NoError(t, repositories.Chat.WithContext(t.Context()).Create(chat))

	testCases := []struct {
		name string
		url  string
		code string
	}{
		{
			"chats: negative limit",
			ts.URL + pathChats + "?limit=-1", errCodeValidationFailed,
		},
		{
			"chats: zero limit",
			ts.URL + pathChats + "?limit=0", errCodeValidationFailed,
		},
		{
			"chats: over-max limit",
			ts.URL + pathChats + "?limit=101", errCodeValidationFailed,
		},
		{
			"chats: non-numeric limit",
			ts.URL + pathChats + "?limit=abc", errCodeBadRequest,
		},
		{
			"chats: negative offset",
			ts.URL + pathChats + "?offset=-1", errCodeValidationFailed,
		},
		{
			"messages: negative limit",
			ts.URL + pathChatsPrefix + chat.ID.String() + "/messages?limit=-1",
			errCodeValidationFailed,
		},
		{
			"messages: zero limit",
			ts.URL + pathChatsPrefix + chat.ID.String() + "/messages?limit=0",
			errCodeValidationFailed,
		},
		{
			"messages: over-max limit",
			ts.URL + pathChatsPrefix + chat.ID.String() + "/messages?limit=201",
			errCodeValidationFailed,
		},
		{
			"messages: non-numeric offset",
			ts.URL + pathChatsPrefix + chat.ID.String() +
				"/messages?offset=xyz",
			errCodeBadRequest,
		},
		{
			"messages: negative offset",
			ts.URL + pathChatsPrefix + chat.ID.String() + "/messages?offset=-1",
			errCodeValidationFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var env map[string]any

			status := requestJSON(t, client, http.MethodGet, tc.url, nil, &env)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, tc.code, env["code"])
		})
	}
}

// TestServer_UpdateChatSettings_ValidationBoundary proves a well-formed
// settings field with an out-of-range value 400s with the VALIDATION_FAILED
// envelope.
func TestServer_UpdateChatSettings_ValidationBoundary(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	chat := &models.Chat{
		UserID: uuid.MustParse(adminID), Title: "t", ModelID: testModel,
	}
	require.NoError(t, repositories.Chat.WithContext(t.Context()).Create(chat))

	settingsURL := ts.URL + pathChatsPrefix + chat.ID.String() + "/settings"

	temp := 3.0
	topP := 1.5
	maxOut := 0
	maxHist := -1

	testCases := []struct {
		name    string
		payload any
		code    string
	}{
		{
			"temperature over max",
			map[string]any{"temperature": temp},
			errCodeValidationFailed,
		},
		{
			"topP over max",
			map[string]any{"topP": topP},
			errCodeValidationFailed,
		},
		{
			"maxOutputTokens not positive",
			map[string]any{"maxOutputTokens": maxOut},
			errCodeValidationFailed,
		},
		{
			"maxHistoryTokens not positive",
			map[string]any{"maxHistoryTokens": maxHist},
			errCodeValidationFailed,
		},
		{
			"invalid reasoningEffort",
			map[string]any{"reasoningEffort": "extreme"},
			errCodeValidationFailed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var env map[string]any

			status := requestJSON(
				t, client, http.MethodPatch, settingsURL, tc.payload, &env,
			)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, tc.code, env["code"])
		})
	}
}

// TestServer_ListChats_EmptyDataset proves an empty result set is a 200 with
// empty items and zero total — never a 404.
func TestServer_ListChats_EmptyDataset(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)
	bootstrapAdmin(t, client, ts)

	var env map[string]any

	status := requestJSON(t, client, http.MethodGet,
		ts.URL+pathChats, nil, &env)
	require.Equal(t, http.StatusOK, status)

	items, ok := env["items"].([]any)
	require.True(t, ok)
	assert.Empty(t, items)
	assert.InDelta(t, float64(0), env["total"], 0)
}
