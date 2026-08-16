//go:build integration

package httpservertest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/aichteeteapee"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pathProjects = "/api/v1/projects"

func TestServer_ChatWorkflow(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)
	chatIDs := seedChats(t, uuid.MustParse(adminID), 2)
	chatID := chatIDs[1]

	var project map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+pathProjects,
		map[string]string{"name": "Operations"},
		&project,
	)
	require.Equal(t, http.StatusCreated, status)
	require.Equal(t, "Operations", project["name"])
	projectID, ok := project["id"].(string)
	require.True(t, ok)

	var projects []map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathProjects,
		nil,
		&projects,
	)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, projects, 1)
	assert.Equal(t, projectID, projects[0]["id"])

	var renamed map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPatch,
		ts.URL+pathProjects+"/"+projectID,
		map[string]string{"name": "Production Operations"},
		&renamed,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "Production Operations", renamed["name"])

	var duplicate map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+pathProjects,
		map[string]string{"name": "Production Operations"},
		&duplicate,
	)
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, aichteeteapee.ErrorCodeConflict, duplicate["code"])

	var assigned map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPut,
		ts.URL+pathChatsPrefix+chatID+"/project",
		map[string]string{"projectId": projectID},
		&assigned,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, projectID, assigned["projectId"])

	var filtered map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChats+"?projectId="+projectID,
		nil,
		&filtered,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(1), filtered["total"], 0)

	var searched map[string]any

	status = requestJSON(
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

	var activeChats map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChats,
		nil,
		&activeChats,
	)
	require.Equal(t, http.StatusOK, status)

	activeItems, ok := activeChats["items"].([]any)
	require.True(t, ok)
	firstChat, ok := activeItems[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, chatID, firstChat["id"])

	var archived map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPut,
		ts.URL+pathChatsPrefix+chatID+"/archive",
		nil,
		&archived,
	)
	require.Equal(t, http.StatusOK, status)
	require.NotEmpty(t, archived["archivedAt"])

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChats,
		nil,
		&activeChats,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(1), activeChats["total"], 0)

	var archivedChats map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChats+"?archived=true",
		nil,
		&archivedChats,
	)
	require.Equal(t, http.StatusOK, status)
	assert.InDelta(t, float64(1), archivedChats["total"], 0)

	var unarchived map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodDelete,
		ts.URL+pathChatsPrefix+chatID+"/archive",
		nil,
		&unarchived,
	)
	require.Equal(t, http.StatusOK, status)

	_, hasArchivedAt := unarchived["archivedAt"]
	assert.False(t, hasArchivedAt)

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

	var cleared map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodDelete,
		ts.URL+pathChatsPrefix+chatID+"/project",
		nil,
		&cleared,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Nil(t, cleared["projectId"])

	status = requestJSON(
		t,
		client,
		http.MethodPut,
		ts.URL+pathChatsPrefix+chatID+"/project",
		map[string]string{"projectId": projectID},
		&assigned,
	)
	require.Equal(t, http.StatusOK, status)

	status = requestJSON(
		t,
		client,
		http.MethodDelete,
		ts.URL+pathProjects+"/"+projectID,
		nil,
		nil,
	)
	assert.Equal(t, http.StatusNoContent, status)

	var unassignedAfterDelete map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+pathChatsPrefix+chatID,
		nil,
		&unassignedAfterDelete,
	)
	require.Equal(t, http.StatusOK, status)
	assert.Nil(t, unassignedAfterDelete["projectId"])

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
