package mcp

import (
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

const toolNameRead = "read"

func TestTool_QualifiedName(t *testing.T) {
	t.Parallel()

	tool := Tool{Server: "fs", Name: toolNameRead}
	assert.Equal(t, "fs__read", tool.QualifiedName())
}

func TestSplitToolName(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		in         string
		wantServer string
		wantTool   string
		wantOK     bool
	}{
		{"valid", "fs__read", "fs", toolNameRead, true},
		{"tool has sep", "fs__a__b", "fs", "a__b", true},
		{"no sep", toolNameRead, "", "", false},
		{"empty server", "__read", "", "", false},
		{"empty tool", "fs__", "", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server, tool, ok := splitToolName(tc.in)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantServer, server)
			assert.Equal(t, tc.wantTool, tool)
		})
	}
}

func TestEnvSlice(t *testing.T) {
	t.Parallel()

	assert.Nil(t, envSlice(nil))
	assert.Nil(t, envSlice(map[string]string{}))

	// Sorted by key for determinism.
	got := envSlice(map[string]string{"B": "2", "A": "1"})
	assert.Equal(t, []string{"A=1", "B=2"}, got)
}

func TestTextOf(t *testing.T) {
	t.Parallel()

	content := []mcpsdk.Content{
		&mcpsdk.TextContent{Text: "line one"},
		&mcpsdk.ImageContent{}, // non-text, ignored
		&mcpsdk.TextContent{Text: "line two"},
	}
	assert.Equal(t, "line one\nline two", textOf(content))
	assert.Empty(t, textOf(nil))
}

func TestSchemaMap(t *testing.T) {
	t.Parallel()

	m := map[string]any{"type": "object"}
	assert.Equal(t, m, schemaMap(any(m)))
	assert.Nil(t, schemaMap("not a map"))
	assert.Nil(t, schemaMap(nil))
}

func TestConnect_UnsupportedTransport(t *testing.T) {
	t.Parallel()

	srv := &models.MCPServer{Transport: "carrier-pigeon"}

	_, err := Connect(t.Context(), srv, testBox(t))
	require.ErrorIs(t, err, ErrUnsupportedTransport)
}

func TestStdioTransport_InjectsEnv(t *testing.T) {
	t.Parallel()

	box := testBox(t)

	envEnc, err := box.SealMap(map[string]string{"API_TOKEN": "sekret"})
	require.NoError(t, err)

	srv := &models.MCPServer{
		Transport: models.MCPTransportStdio,
		Command:   "run-server",
		Args:      datatypes.JSON(`["--flag","x"]`),
		EnvEnc:    envEnc,
	}

	transport, err := stdioTransport(srv, box)
	require.NoError(t, err)

	assert.Contains(t, transport.Command.Args, "--flag")
	// Env decrypted from EnvEnc and injected into the child process.
	assert.Contains(t, transport.Command.Env, "API_TOKEN=sekret")
}

func TestHTTPTransport_InjectsHeaders(t *testing.T) {
	t.Parallel()

	box := testBox(t)

	headers := map[string]string{"Authorization": "Bearer z"}

	headersEnc, err := box.SealMap(headers)
	require.NoError(t, err)

	srv := &models.MCPServer{
		Transport:  models.MCPTransportHTTP,
		URL:        "https://mcp.example.com/mcp",
		HeadersEnc: headersEnc,
	}

	transport, err := httpTransport(srv, box)
	require.NoError(t, err)

	assert.Equal(t, "https://mcp.example.com/mcp", transport.Endpoint)

	_, ok := transport.HTTPClient.Transport.(*headerRoundTripper)
	assert.True(t, ok) // custom header injector wired
}
