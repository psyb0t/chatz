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

// TestServer_MCPServerCRUD drives the admin MCP-server lifecycle over the real
// DB — create, list, tools, update, delete — through the handlers now backed
// by mcp.ServerStore (no direct repo access). It also pins every not-found
// path to a 404 (the store's commerr.ErrNotFound mapping). An http
// transport with an unreachable URL is used so the background connect fails
// harmlessly.
func TestServer_MCPServerCRUD(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	status := requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		nil,
	)
	require.Equal(t, http.StatusOK, status)

	// Create -> 201 with the persisted row echoed back.
	var created map[string]any

	status = requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathMCPServers,
		map[string]any{
			"name":      "test-mcp",
			"transport": "http",
			"url":       "http://127.0.0.1:1/mcp",
		},
		&created,
	)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "test-mcp", created["name"])

	serverID, ok := created["id"].(string)
	require.True(t, ok)

	// List -> the one server we created.
	var list []map[string]any

	requestJSON(t, client, http.MethodGet,
		ts.URL+pathMCPServers, nil, &list)
	require.Len(t, list, 1)
	assert.Equal(t, "test-mcp", list[0]["name"])
	attemptedAt, ok := list[0]["lastConnectionAttemptAt"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, attemptedAt)

	// An authenticated non-admin receives no MCP health/configuration data.
	// The server must reject before serializing any admin-only response fields.
	status = requestJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+pathUsers,
		map[string]any{
			"username": "mcp-viewer",
			"password": "pw-mcp-viewer",
		},
		nil,
	)
	require.Equal(t, http.StatusCreated, status)

	nonAdminClient := newClient(t)
	status = requestJSON(
		t,
		nonAdminClient,
		http.MethodPost,
		ts.URL+pathAuthLogin,
		map[string]string{
			"username": "mcp-viewer",
			"password": "pw-mcp-viewer",
		},
		nil,
	)
	require.Equal(t, http.StatusOK, status)

	status = requestJSON(
		t,
		nonAdminClient,
		http.MethodGet,
		ts.URL+pathMCPServers,
		nil,
		nil,
	)
	assert.Equal(t, http.StatusForbidden, status)

	// Tools on a live-but-unconnected server -> 200 (empty; tools exist only
	// while connected).
	status = requestJSON(t, client, http.MethodGet,
		ts.URL+pathMCPServers+"/"+serverID+"/tools", nil, nil)
	assert.Equal(t, http.StatusOK, status)

	// Update (PATCH) the name -> 200 with the new name.
	var updated map[string]any

	status = requestJSON(t, client, http.MethodPatch,
		ts.URL+pathMCPServers+"/"+serverID,
		map[string]any{"name": "renamed-mcp"}, &updated)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "renamed-mcp", updated["name"])

	// Delete -> 204, then the list is empty.
	status = requestJSON(t, client, http.MethodDelete,
		ts.URL+pathMCPServers+"/"+serverID, nil, nil)
	require.Equal(t, http.StatusNoContent, status)

	var afterDelete []map[string]any

	requestJSON(t, client, http.MethodGet,
		ts.URL+pathMCPServers, nil, &afterDelete)
	assert.Empty(t, afterDelete)

	// Every not-found path maps to 404 via mcp.ServerStore.Get ->
	// commerr.ErrNotFound.
	missing := uuid.New().String()

	status = requestJSON(t, client, http.MethodDelete,
		ts.URL+pathMCPServers+"/"+missing, nil, nil)
	assert.Equal(t, http.StatusNotFound, status)

	status = requestJSON(t, client, http.MethodPatch,
		ts.URL+pathMCPServers+"/"+missing,
		map[string]any{"name": "nope"}, nil)
	assert.Equal(t, http.StatusNotFound, status)

	status = requestJSON(t, client, http.MethodGet,
		ts.URL+pathMCPServers+"/"+missing+"/tools", nil, nil)
	assert.Equal(t, http.StatusNotFound, status)

	status = requestJSON(t, client, http.MethodPost,
		ts.URL+pathMCPServers+"/"+missing+"/reconnect", nil, nil)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestServer_CreateStdioMCPServer drives the stdio branch of the create
// handler (applyStdio): command, args, and env are persisted, with env sealed
// at rest and returned decrypted in the admin response.
func TestServer_CreateStdioMCPServer(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	status := requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		nil,
	)
	require.Equal(t, http.StatusOK, status)

	var created map[string]any

	status = requestJSON(
		t, client, http.MethodPost,
		ts.URL+pathMCPServers,
		map[string]any{
			"name":      "stdio-mcp",
			"transport": "stdio",
			"command":   "my-mcp-binary",
			"args":      []string{"--serve", "--quiet"},
			"env":       map[string]string{"API_TOKEN": "sekret-value"},
		},
		&created,
	)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "stdio-mcp", created["name"])
	assert.Equal(t, "my-mcp-binary", created["command"])

	args, ok := created["args"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"--serve", "--quiet"}, args)

	env, ok := created["env"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "sekret-value", env["API_TOKEN"])
}

// TestServer_CreateHTTPMCPWithHeaders drives the http branch of the create
// handler with headers, so applyHTTP seals the header credential at rest. The
// URL is unreachable, so the async connect fails harmlessly.
func TestServer_CreateHTTPMCPWithHeaders(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	status := requestJSON(
		t, client, http.MethodPost, ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		nil,
	)
	require.Equal(t, http.StatusOK, status)

	var created map[string]any

	status = requestJSON(
		t, client, http.MethodPost, ts.URL+pathMCPServers,
		map[string]any{
			"name":      "hdr-mcp",
			"transport": "http",
			"url":       "http://127.0.0.1:1/mcp",
			"headers":   map[string]string{"Authorization": "Bearer tok"},
		}, &created,
	)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "hdr-mcp", created["name"])
}

// TestServer_MCPReconnectAndUpdateSecrets covers the reconnect endpoint on an
// existing server and the update path that reseals secrets (env + headers) and
// rewires the live connection, including the disable branch. The URLs are
// unreachable, so the background connects fail harmlessly.
func TestServer_MCPReconnectAndUpdateSecrets(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client := newClient(t)

	status := requestJSON(
		t, client, http.MethodPost, ts.URL+pathAuthSetup,
		map[string]string{"username": adminUser, "password": adminPassword},
		nil,
	)
	require.Equal(t, http.StatusOK, status)

	var created map[string]any

	status = requestJSON(
		t, client, http.MethodPost, ts.URL+pathMCPServers,
		map[string]any{
			"name":      "secret-mcp",
			"transport": "http",
			"url":       "http://127.0.0.1:1/mcp",
		}, &created,
	)
	require.Equal(t, http.StatusCreated, status)

	serverID, ok := created["id"].(string)
	require.True(t, ok)

	// Reconnect an existing server.
	status = requestJSON(t, client, http.MethodPost,
		ts.URL+pathMCPServers+"/"+serverID+"/reconnect", nil, nil)
	require.Equal(t, http.StatusOK, status)

	// Update reseals env + headers and rewires the live connection.
	status = requestJSON(t, client, http.MethodPatch,
		ts.URL+pathMCPServers+"/"+serverID,
		map[string]any{
			"url":     "http://127.0.0.1:2/mcp",
			"env":     map[string]string{"TOKEN": "sekret"},
			"headers": map[string]string{"Authorization": "Bearer abc"},
		}, nil)
	require.Equal(t, http.StatusOK, status)

	// Disabling rewires through the disable branch instead of a reconnect.
	var disabled map[string]any

	status = requestJSON(t, client, http.MethodPatch,
		ts.URL+pathMCPServers+"/"+serverID,
		map[string]any{"enabled": false}, &disabled)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, disabled["enabled"])
}

// TestServer_ChatMCPServers drives the per-chat MCP enable/disable endpoints:
// the list reflects every globally-known server with its live status + this
// chat's enabled flag; PATCH toggles a server off then back on for THIS chat
// only; and both endpoints are ownership-checked (a chat the caller doesn't
// own is indistinguishable from a missing one -> 404).
func TestServer_ChatMCPServers(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, adminID := newAuthedClient(t, ts)

	// A globally-enabled MCP server. Its URL is unreachable, so the live status
	// resolves to connecting/failed async — but per-chat enablement is a
	// separate axis that doesn't depend on the connection outcome.
	var created map[string]any

	status := requestJSON(t, client, http.MethodPost,
		ts.URL+pathMCPServers,
		map[string]any{
			"name":      "chat-scoped-mcp",
			"transport": "http",
			"url":       "http://127.0.0.1:1/mcp",
		}, &created)
	require.Equal(t, http.StatusCreated, status)

	serverID, ok := created["id"].(string)
	require.True(t, ok)

	chatID := seedChats(t, uuid.MustParse(adminID), 1)[0]
	base := ts.URL + pathChatsPrefix + chatID + "/mcp-servers"

	validStatus := map[string]bool{
		"connecting": true,
		"connected":  true,
		"failed":     true,
		"disabled":   true,
	}

	// List -> the one server, enabled for this chat by default, live status
	// set.
	var list []map[string]any

	status = requestJSON(t, client, http.MethodGet, base, nil, &list)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, list, 1)
	assert.Equal(t, serverID, list[0]["id"])
	assert.Equal(t, "chat-scoped-mcp", list[0]["name"])
	assert.Equal(t, true, list[0]["enabled"])

	st, ok := list[0]["status"].(string)
	require.True(t, ok)
	assert.True(t, validStatus[st], "unexpected status %q", st)

	// Disable it for this chat -> 200, enabled=false echoed back.
	var patched map[string]any

	status = requestJSON(t, client, http.MethodPatch,
		base+"/"+serverID, map[string]any{"enabled": false}, &patched)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, patched["enabled"])

	// The disable persisted.
	status = requestJSON(t, client, http.MethodGet, base, nil, &list)
	require.Equal(t, http.StatusOK, status)
	require.Len(t, list, 1)
	assert.Equal(t, false, list[0]["enabled"])

	// Re-enable -> the disabled set empties back to '[]' (the Select-column
	// persist path that a plain Updates would skip on a zero-value field).
	status = requestJSON(t, client, http.MethodPatch,
		base+"/"+serverID, map[string]any{"enabled": true}, &patched)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, patched["enabled"])

	status = requestJSON(t, client, http.MethodGet, base, nil, &list)
	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, list[0]["enabled"])

	// Ownership: a chat owned by a different real user is a 404 on both
	// endpoints, never leaking that the chat exists.
	otherUser := &models.User{Username: "other-mcp", IsAdmin: false}
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

	status = requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+fID+"/mcp-servers", nil, nil)
	assert.Equal(t, http.StatusNotFound, status)

	status = requestJSON(t, client, http.MethodPatch,
		ts.URL+pathChatsPrefix+fID+"/mcp-servers/"+serverID,
		map[string]any{"enabled": false}, nil)
	assert.Equal(t, http.StatusNotFound, status)

	// A missing server id under an owned chat is also a 404.
	status = requestJSON(t, client, http.MethodPatch,
		base+"/"+uuid.New().String(), map[string]any{"enabled": false}, nil)
	assert.Equal(t, http.StatusNotFound, status)

	// A missing chat id is a 404.
	status = requestJSON(t, client, http.MethodGet,
		ts.URL+pathChatsPrefix+uuid.New().String()+"/mcp-servers", nil, nil)
	assert.Equal(t, http.StatusNotFound, status)
}

// TestServer_ImportMCP drives the POST /mcp/servers/import endpoint: a valid
// .mcp.json persists its servers and returns them, an empty file is a 400
// (ErrNoServers), and a server with neither command nor url is a 400
// (ErrInvalidServer). The imported server points at an unreachable url so the
// async connect fails harmlessly, matching the other MCP tests here.
func TestServer_ImportMCP(t *testing.T) {
	resetDB(t)

	ts := newTestServer(t)
	client, _ := newAuthedClient(t, ts)
	importURL := ts.URL + "/api/v1/mcp/import"

	var imported map[string]any

	status := requestJSON(
		t,
		client,
		http.MethodPost,
		importURL,
		map[string]string{
			"content": `{"mcpServers":{"demo":` +
				`{"url":"http://127.0.0.1:1/mcp"}}}`,
		},
		&imported,
	)
	require.Equal(t, http.StatusOK, status)

	servers, ok := imported["imported"].([]any)
	require.True(t, ok)
	assert.Len(t, servers, 1)

	var empty map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPost,
		importURL,
		map[string]string{"content": `{"mcpServers":{}}`},
		&empty,
	)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, errCodeBadRequest, empty["code"])

	var invalid map[string]any

	status = requestJSON(
		t,
		client,
		http.MethodPost,
		importURL,
		map[string]string{"content": `{"mcpServers":{"broken":{}}}`},
		&invalid,
	)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, errCodeBadRequest, invalid["code"])
}
