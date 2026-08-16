package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

const (
	testMCPVersion = "2025-06-18"
	maskedSecret   = "[SECRET]"

	headerXMCPVersion = "X-MCP-Version"
)

func testSecretsBox(t *testing.T) *secrets.Box {
	t.Helper()

	key := make([]byte, secrets.KeySize)
	for i := range key {
		key[i] = byte(i)
	}

	box, err := secrets.New(key)
	require.NoError(t, err)

	return box
}

func TestMCPServerResponse_HTTP_MasksAuthorizationHeader(t *testing.T) {
	box := testSecretsBox(t)
	srv := New(Deps{Secrets: box})
	ctx := context.Background()

	headersEnc, err := box.SealMap(map[string]string{
		aichteeteapee.HeaderNameAuthorization: "Bearer tok",
		headerXMCPVersion:                     testMCPVersion,
	})
	require.NoError(t, err)

	httpOut := srv.mcpServerResponse(ctx, &models.MCPServer{
		Name:       "brain",
		Transport:  models.MCPTransportHTTP,
		Enabled:    true,
		URL:        "https://mcp.example/mcp",
		HeadersEnc: headersEnc,
	})

	require.NotNil(t, httpOut.Headers)
	assert.Equal(t, "Bearer "+maskedSecret,
		(*httpOut.Headers)[aichteeteapee.HeaderNameAuthorization])
	assert.Equal(t, testMCPVersion, (*httpOut.Headers)[headerXMCPVersion])
	assert.Nil(t, httpOut.Env)
}

func TestMCPServerResponse_Stdio_RendersEnvAndArgs(t *testing.T) {
	box := testSecretsBox(t)
	srv := New(Deps{Secrets: box})
	ctx := context.Background()

	envEnc, err := box.SealMap(map[string]string{"API_TOKEN": "sek"})
	require.NoError(t, err)

	argsJSON, err := json.Marshal([]string{"server.py", "http"})
	require.NoError(t, err)

	stdioOut := srv.mcpServerResponse(ctx, &models.MCPServer{
		Name:      "local",
		Transport: models.MCPTransportStdio,
		Enabled:   true,
		Command:   "python3",
		Args:      datatypes.JSON(argsJSON),
		EnvEnc:    envEnc,
	})

	require.NotNil(t, stdioOut.Env)
	assert.Equal(t, "sek", (*stdioOut.Env)["API_TOKEN"])
	require.NotNil(t, stdioOut.Args)
	assert.Equal(t, []string{"server.py", "http"}, *stdioOut.Args)
}

func TestMaskedAuthorizationHeaderValue(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "bearer", value: "Bearer token", want: "Bearer " + maskedSecret},
		{
			name:  "basic",
			value: "Basic credentials",
			want:  "Basic " + maskedSecret,
		},
		{
			name:  "custom scheme",
			value: "Custom value",
			want:  "Custom " + maskedSecret,
		},
		{name: "blank", value: "", want: maskedSecret},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, maskedAuthorizationHeaderValue(tc.value))
		})
	}
}

// A nil secrets box (sealing disabled) yields empty maps, never an error, so
// the list still renders.
func TestMCPSecrets_NilBoxYieldsEmpty(t *testing.T) {
	srv := New(Deps{})

	env, headers := srv.mcpSecrets(
		context.Background(), &models.MCPServer{Name: "x"},
	)
	assert.Nil(t, env)
	assert.Nil(t, headers)
}

func TestParseMCPArgs(t *testing.T) {
	ctx := context.Background()

	good, err := json.Marshal([]string{"a", "b"})
	require.NoError(t, err)

	assert.Equal(t, []string{"a", "b"}, parseMCPArgs(
		ctx, &models.MCPServer{Name: "s", Args: datatypes.JSON(good)},
	))
	assert.Nil(t, parseMCPArgs(ctx, &models.MCPServer{Name: "s"}))
	assert.Nil(t, parseMCPArgs(
		ctx, &models.MCPServer{Name: "s", Args: datatypes.JSON(`{bad`)},
	))
}

// PATCH secret semantics: an absent map keeps the stored value, a present map
// replaces it, and a present-empty map clears it.
func TestApplyMCPUpdateSecrets_PresentReplacesAbsentKeeps(t *testing.T) {
	box := testSecretsBox(t)
	srv := New(Deps{Secrets: box})

	orig, err := box.SealMap(map[string]string{
		aichteeteapee.HeaderNameAuthorization: "Bearer old",
	})
	require.NoError(t, err)

	m := &models.MCPServer{
		Name:       "h",
		Transport:  models.MCPTransportHTTP,
		HeadersEnc: orig,
	}

	require.NoError(t, srv.applyMCPUpdateSecrets(
		m, &api.UpdateMCPServerRequest{},
	))
	assert.Equal(t, orig, m.HeadersEnc, "absent headers keep the stored value")

	newHeaders := map[string]string{
		aichteeteapee.HeaderNameAuthorization: "Bearer new",
	}
	require.NoError(t, srv.applyMCPUpdateSecrets(
		m, &api.UpdateMCPServerRequest{Headers: &newHeaders},
	))

	got, err := box.OpenMap(m.HeadersEnc)
	require.NoError(t, err)
	assert.Equal(t, "Bearer new", got[aichteeteapee.HeaderNameAuthorization])

	empty := map[string]string{}
	require.NoError(t, srv.applyMCPUpdateSecrets(
		m, &api.UpdateMCPServerRequest{Headers: &empty},
	))
	assert.Empty(t, m.HeadersEnc, "present-empty headers clear the value")
}

func TestApplyMCPUpdateSecrets_PreservesMaskedAuthorization(t *testing.T) {
	box := testSecretsBox(t)
	srv := New(Deps{Secrets: box})

	orig, err := box.SealMap(map[string]string{
		"authorization":   "Basic old-credentials",
		headerXMCPVersion: testMCPVersion,
	})
	require.NoError(t, err)

	m := &models.MCPServer{HeadersEnc: orig}
	updatedHeaders := map[string]string{
		aichteeteapee.HeaderNameAuthorization: "Basic " + maskedSecret,
		headerXMCPVersion:                     testMCPVersion,
	}
	require.NoError(t, srv.applyMCPUpdateSecrets(
		m, &api.UpdateMCPServerRequest{Headers: &updatedHeaders},
	))

	got, err := box.OpenMap(m.HeadersEnc)
	require.NoError(t, err)
	assert.Equal(t, "Basic old-credentials",
		got[aichteeteapee.HeaderNameAuthorization])
	assert.Equal(t, testMCPVersion, got[headerXMCPVersion])
}
