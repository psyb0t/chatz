//go:build real

// Package realtest drives chatz's agentic chat loop against a configured REAL
// OpenAI-compatible or Anthropic upstream plus a REAL MCP tool server. It
// proves the whole tool-call chain and remains opt-in through the real build
// tag and the same CHATZ_UPSTREAMS environment used by `make run`.
package realtest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/config"
	chatcore "github.com/psyb0t/chatz/internal/pkg/core/chat"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/chatz/internal/pkg/secrets"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/elelem/drivers/anthropic"
	"github.com/psyb0t/elelem/drivers/openai"
	"github.com/psyb0t/essessey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

const (
	defaultRealModel  = "groq-gpt-oss-120b"
	realTestAPIKeyEnv = "CHATZ_REAL_TEST_API_KEY" //nolint:gosec // env name
	realTestAPIKey    = "fake-test-key"
	echoTool          = "stdio__echo"
	echoMarker        = "banana-42"
	turnTimeout       = 120 * time.Second
	toolRequestPrompt = `Call the %s tool with the exact text %q. Then tell me exactly what it returned.` //nolint:lll // one-line user prompt

	// echoResultPrefix is how the echo MCP tool prefixes its tool-result
	// content; both the stdio and HTTP round-trip tests look for it.
	echoResultPrefix = "echo: "

	// realSystemPrompt is the system prompt shared by every real-LLM tool
	// round-trip test: it forces the model to call a tool instead of
	// guessing at the answer.
	realSystemPrompt = "You are a helpful assistant. When a tool can " +
		"answer the request, you MUST call it rather than guessing."

	// on-wire assertion messages shared by every real-LLM tool test.
	msgToolUseOnWire    = "a tool_use block should be on the wire"
	msgToolResultOnWire = "a tool_result block should be on the wire"

	// stdioTransportName is the MCP server name/transport-arg used to wire the
	// stdio-transport test server.
	stdioTransportName = "stdio"
)

// realLLM resolves and builds the first configured upstream the same way the
// app does. It skips before constructing a request when no live key is present.
func realLLM(t *testing.T) (*elelem.Client, elelem.Model) {
	t.Helper()

	cfg := config.Config{UpstreamsJSON: os.Getenv("CHATZ_UPSTREAMS")}
	upstreams, err := cfg.Upstreams()
	require.NoError(t, err)

	if len(upstreams) == 0 {
		t.Skip("no upstreams configured — set CHATZ_UPSTREAMS (the same .env " +
			"`make run` uses) to run the real-LLM tests")
	}

	up := upstreams[0]
	apiKey := up.APIKey()

	if apiKey == "" {
		t.Skip("no LLM key configured — set CHATZ_UPSTREAMS + its api key " +
			"env (the same .env `make run` uses) to run the real-LLM tests")
	}

	configuredModelID := os.Getenv("CHATZ_REAL_MODEL")
	driver := realDriverFor(t, up)

	switch up.Provider {
	case config.UpstreamProviderOpenAI:
		if configuredModelID == "" {
			configuredModelID = defaultRealModel
		}

		return elelem.New(driver), openai.LookupModel(configuredModelID)
	case config.UpstreamProviderAnthropic:
		if configuredModelID != "" {
			return elelem.New(driver),
				anthropic.LookupModel(configuredModelID)
		}

		knownModels := anthropic.KnownModels()
		require.NotEmpty(t, knownModels)

		return elelem.New(driver), knownModels[0]
	default:
		require.FailNow(t, "unsupported parsed upstream provider", up.Provider)

		return nil, elelem.Model{}
	}
}

// realDriverFor builds ONE upstream's driver exactly as
// internal/pkg/services/http-server does.
//
// Shared by realLLM (which drives the first upstream through the agentic loop)
// and the per-upstream reachability tests, so the two cannot drift into
// constructing drivers differently — a test that builds its own driver proves
// the SDK works, not that this app's configuration does.
func realDriverFor(t *testing.T, up config.Upstream) elelem.Driver {
	t.Helper()

	apiKey := up.APIKey()

	switch up.Provider {
	case config.UpstreamProviderOpenAI:
		options := []openai.DriverOption{}
		if apiKey != "" {
			options = append(options, openai.WithAPIKey(apiKey))
		}

		if up.BaseURL != "" {
			options = append(options, openai.WithBaseURL(up.BaseURL))
		}

		return openai.NewDriver(options...)
	case config.UpstreamProviderAnthropic:
		options := []anthropic.DriverOption{}
		if apiKey != "" {
			options = append(options, anthropic.WithAPIKey(apiKey))
		}

		if up.BaseURL != "" {
			options = append(options, anthropic.WithBaseURL(up.BaseURL))
		}

		return anthropic.NewDriver(options...)
	default:
		require.FailNow(t, "unsupported upstream provider", up.Provider)

		return nil
	}
}

func TestRealLLMProviderDefaults(t *testing.T) {
	knownAnthropicModels := anthropic.KnownModels()
	require.NotEmpty(t, knownAnthropicModels)

	testCases := []struct {
		name       string
		provider   config.UpstreamProvider
		wantDriver elelem.Driver
		wantModel  string
	}{
		{
			name:       "OpenAI-compatible",
			provider:   config.UpstreamProviderOpenAI,
			wantDriver: &openai.Driver{},
			wantModel:  defaultRealModel,
		},
		{
			name:       "Anthropic",
			provider:   config.UpstreamProviderAnthropic,
			wantDriver: &anthropic.Driver{},
			wantModel:  knownAnthropicModels[0].ID,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("CHATZ_UPSTREAMS", testUpstreamJSON(t, testCase.provider))
			t.Setenv(realTestAPIKeyEnv, realTestAPIKey)
			t.Setenv("CHATZ_REAL_MODEL", "")

			client, model := realLLM(t)

			assert.IsType(t, testCase.wantDriver, client.Driver())
			assert.Equal(t, testCase.wantModel, model.ID)
		})
	}
}

func testUpstreamJSON(
	t *testing.T,
	provider config.UpstreamProvider,
) string {
	t.Helper()

	payload, err := json.Marshal([]config.Upstream{{
		Name:      "real-test",
		Provider:  provider,
		APIKeyEnv: realTestAPIKeyEnv,
	}})
	require.NoError(t, err)

	return string(payload)
}

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

// serverScript resolves tests/mcpserver/server.py relative to this test file.
func serverScript(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)

	script := filepath.Join(
		filepath.Dir(thisFile), "..", "mcpserver", "server.py",
	)
	require.FileExists(t, script)

	return script
}

// stdioManager wires an MCP manager to the Python test server over stdio.
func stdioManager(ctx context.Context, t *testing.T) *mcp.Manager {
	t.Helper()

	mgr := mcp.NewManager(testBox(t))
	t.Cleanup(func() { _ = mgr.Close() })

	argsJSON, err := json.Marshal(
		[]string{serverScript(t), stdioTransportName},
	)
	require.NoError(t, err)

	require.NoError(t, mgr.Add(ctx, &models.MCPServer{
		Name:      stdioTransportName,
		Transport: models.MCPTransportStdio,
		Command:   "python3",
		Args:      datatypes.JSON(argsJSON),
	}))

	return mgr
}

// toolRoundTripCase parameterizes the "model calls tool → MCP executes → SSE
// carries tool_use/tool_result → answer includes the result" scenario across
// transports (stdio, HTTP). setupFn runs BEFORE ctx exists (e.g. grabbing a
// free port + booting the Python HTTP server); managerFn then wires the MCP
// manager once ctx is available.
type toolRoundTripCase struct {
	name           string
	toolName       string
	messageID      string
	conversationID string
	setupFn        func(t *testing.T)
	managerFn      func(t *testing.T, ctx context.Context) *mcp.Manager
}

// TestReal_ToolCallRoundTrip forces the model to call the echo MCP tool over
// each supported transport and verifies the full chain: an assistant tool_use
// for the echo tool, a matching tool_result carrying the echoed marker,
// tool_use + tool_result SSE blocks on the wire, and the marker surfacing in
// the final answer.
func TestReal_ToolCallRoundTrip(t *testing.T) {
	// httpPort is shared between the "http" case's setupFn (which boots the
	// Python server on it) and managerFn (which points the MCP manager at it).
	var httpPort int

	testCases := []toolRoundTripCase{
		{
			name:           "stdio",
			toolName:       echoTool,
			messageID:      "msg_real_tool",
			conversationID: "conv_real_tool",
			managerFn: func(t *testing.T, ctx context.Context) *mcp.Manager {
				t.Helper()

				return stdioManager(ctx, t)
			},
		},
		{
			name:           "http",
			toolName:       httpEchoTool,
			messageID:      "msg_real_http",
			conversationID: "conv_real_http",
			setupFn: func(t *testing.T) {
				t.Helper()

				httpPort = freePort(t)
				startPythonHTTP(t, serverScript(t), httpPort)
			},
			managerFn: func(t *testing.T, ctx context.Context) *mcp.Manager {
				t.Helper()

				return httpManager(ctx, t, httpPort)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runToolCallRoundTrip(t, tc)
		})
	}
}

// runToolCallRoundTrip runs a single transport's tool round-trip scenario:
// build the deps + request, run the turn, and assert the tool chain both in
// the returned messages and on the SSE wire.
func runToolCallRoundTrip(t *testing.T, tc toolRoundTripCase) {
	t.Helper()

	client, model := realLLM(t)

	if tc.setupFn != nil {
		tc.setupFn(t)
	}

	ctx, cancel := context.WithTimeout(context.Background(), turnTimeout)
	defer cancel()

	mgr := tc.managerFn(t, ctx)

	// Sanity: the tool the model is asked to call is actually offered.
	require.Contains(t, toolNames(mgr.Tools(ctx)), tc.toolName)

	sink := &essessey.InMemorySink{}
	pub := essessey.NewPublisher(ctx, sink)

	result, err := chatcore.Run(ctx, chatcore.Deps{
		Client:    client,
		Tools:     mgr,
		Publisher: pub,
	}, chatcore.Request{
		MessageID:      tc.messageID,
		ConversationID: tc.conversationID,
		Model:          model,
		Prompt: elelem.NewPrompt().
			WithSystem(realSystemPrompt).
			UserText(fmt.Sprintf(
				toolRequestPrompt,
				tc.toolName,
				echoMarker,
			)),
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assertToolChain(t, result, tc.toolName)
	assertToolBlocksOnWire(t, sink, tc.toolName)

	assert.Contains(
		t,
		strings.ToLower(result.FinalText),
		echoMarker,
		"final answer should carry the echoed marker; got: %s",
		result.FinalText,
	)

	// gpt-oss-class models usually stream reasoning; the gateway may strip it,
	// so this is observed (logged), not asserted, to avoid gateway-dependent
	// flake.
	t.Logf("thinking blocks on wire: %d",
		countBlocks(sink, essessey.ContentBlockTypeThinking))
	t.Logf("rounds: %d, total tokens: %d", result.Rounds, result.Usage.Total)
}

// assertToolChain checks the conversation state: an assistant message
// requested toolName, and a tool-role message fed back the echoed marker.
func assertToolChain(
	t *testing.T,
	result *chatcore.Result,
	toolName string,
) {
	t.Helper()

	var calledEcho, gotResult bool

	for _, m := range result.Messages {
		for _, tc := range m.ToolCalls {
			if tc.Name == toolName {
				calledEcho = true

				assert.Contains(t, string(tc.Arguments), echoMarker,
					"echo args should contain the marker")
			}
		}

		if m.Role == elelem.RoleTool &&
			strings.Contains(m.Text(), echoResultPrefix+echoMarker) {
			gotResult = true
		}
	}

	assert.True(t, calledEcho, "model should have called %s", toolName)
	assert.True(t, gotResult, "tool result should echo the marker back")
}

// assertToolBlocksOnWire scans the emitted SSE events for a tool_use block
// naming toolName and a tool_result block.
func assertToolBlocksOnWire(
	t *testing.T,
	sink *essessey.InMemorySink,
	toolName string,
) {
	t.Helper()

	assert.Positive(t, countBlocks(sink, essessey.ContentBlockTypeToolUse),
		msgToolUseOnWire)
	assert.Positive(t, countBlocks(sink, essessey.ContentBlockTypeToolResult),
		msgToolResultOnWire)

	var sawEchoName bool

	for _, ev := range sink.Events() {
		data := string(ev.Data)
		if strings.Contains(data, essessey.ContentBlockTypeToolUse) &&
			strings.Contains(data, toolName) {
			sawEchoName = true
		}
	}

	assert.True(t, sawEchoName, "tool_use block should name %s", toolName)
}

// countBlocks counts content blocks of a given type by matching the block's
// type tag in the event data.
func countBlocks(sink *essessey.InMemorySink, blockType string) int {
	needle := `"type":"` + blockType + `"`

	var n int

	for _, ev := range sink.Events() {
		if strings.Contains(string(ev.Data), needle) {
			n++
		}
	}

	return n
}

func toolNames(tools []mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.QualifiedName())
	}

	return names
}
