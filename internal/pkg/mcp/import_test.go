package mcp

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testBox(t *testing.T) *secrets.Box {
	t.Helper()

	key := make([]byte, secrets.KeySize)
	for i := range key {
		key[i] = byte(i)
	}

	box, err := secrets.New(key)
	require.NoError(t, err)

	return box
}

const mixedMCPJSON = `{
  "mcpServers": {
    "remote-search": {
      "type": "http",
      "url": "https://mcp.example.com/v1",
      "headers": { "Authorization": "Bearer tok-abc" }
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/data"],
      "env": { "ROOT_TOKEN": "env-xyz" }
    }
  }
}`

func TestParseMCPJSON_MixedTransports(t *testing.T) {
	t.Parallel()

	box := testBox(t)
	admin := uuid.New()

	servers, err := ParseMCPJSON([]byte(mixedMCPJSON), box, &admin)
	require.NoError(t, err)
	require.Len(t, servers, 2)

	// Sorted by name: "filesystem" (stdio) before "remote-search" (http).
	stdio, http := servers[0], servers[1]

	assert.Equal(t, "filesystem", stdio.Name)
	assert.Equal(t, models.MCPTransportStdio, stdio.Transport)
	assert.Equal(t, "npx", stdio.Command)
	assert.True(t, stdio.Enabled)
	require.NotNil(t, stdio.CreatedBy)
	assert.Equal(t, admin, *stdio.CreatedBy)
	assert.Nil(t, stdio.HeadersEnc)

	var gotArgs []string
	require.NoError(t, json.Unmarshal(stdio.Args, &gotArgs))
	assert.Equal(t,
		[]string{"-y", "@modelcontextprotocol/server-filesystem", "/data"},
		gotArgs,
	)

	env, err := box.OpenMap(stdio.EnvEnc)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"ROOT_TOKEN": "env-xyz"}, env)
	assert.NotContains(t, string(stdio.EnvEnc), "env-xyz") // sealed, opaque

	assert.Equal(t, "remote-search", http.Name)
	assert.Equal(t, models.MCPTransportHTTP, http.Transport)
	assert.Equal(t, "https://mcp.example.com/v1", http.URL)
	assert.Empty(t, http.Command)
	assert.Nil(t, http.EnvEnc)

	headers, err := box.OpenMap(http.HeadersEnc)
	require.NoError(t, err)
	assert.Equal(t,
		map[string]string{"Authorization": "Bearer tok-abc"},
		headers,
	)
}

func TestParseMCPJSON_StdioNoEnv(t *testing.T) {
	t.Parallel()

	raw := `{"mcpServers":{"x":{"command":"run","args":["a"]}}}`

	servers, err := ParseMCPJSON([]byte(raw), testBox(t), nil)
	require.NoError(t, err)
	require.Len(t, servers, 1)
	assert.Nil(t, servers[0].EnvEnc) // empty env -> nil blob
	assert.Nil(t, servers[0].CreatedBy)
}

func TestParseMCPJSON_Errors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{"empty object", `{}`, ErrNoServers},
		{"empty servers", `{"mcpServers":{}}`, ErrNoServers},
		{
			"neither command nor url",
			`{"mcpServers":{"bad":{"type":"http"}}}`,
			ErrInvalidServer,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseMCPJSON([]byte(tc.raw), testBox(t), nil)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestParseMCPJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseMCPJSON([]byte("{not json"), testBox(t), nil)
	require.Error(t, err)
}
