package chats

import (
	"context"
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeToolExecutor is a scriptable ToolExecutor for exercising the per-chat
// filtering wrapper without a live MCP manager.
type fakeToolExecutor struct {
	tools      []mcp.Tool
	callResult *mcp.ToolResult
	callErr    error
	gotName    string
}

func (f *fakeToolExecutor) Tools(context.Context) []mcp.Tool {
	return f.tools
}

func (f *fakeToolExecutor) Call(
	_ context.Context,
	qualifiedName string,
	_ map[string]any,
) (*mcp.ToolResult, error) {
	f.gotName = qualifiedName

	return f.callResult, f.callErr
}

// TestFilteredToolExecutorHidesBlockedServers proves the per-chat filter drops
// every tool whose server the chat disabled while leaving the rest intact.
func TestFilteredToolExecutorHidesBlockedServers(t *testing.T) {
	t.Parallel()

	inner := &fakeToolExecutor{
		tools: []mcp.Tool{
			{Server: "allowed", Name: "search"},
			{Server: "blocked", Name: "delete"},
			{Server: "allowed", Name: "summarize"},
		},
	}
	filtered := filteredToolExecutor{
		inner:          inner,
		blockedServers: map[string]bool{"blocked": true},
	}

	got := filtered.Tools(t.Context())
	require.Len(t, got, 2)

	for _, tool := range got {
		assert.Equal(t, "allowed", tool.Server)
	}
}

// TestFilteredToolExecutorCallPassesThrough proves Call is a transparent
// passthrough: the filter hides tools from the model's view but never rewrites
// an actual invocation.
func TestFilteredToolExecutorCallPassesThrough(t *testing.T) {
	t.Parallel()

	inner := &fakeToolExecutor{
		callResult: &mcp.ToolResult{Text: "done"},
	}
	filtered := filteredToolExecutor{
		inner:          inner,
		blockedServers: map[string]bool{"blocked": true},
	}

	result, err := filtered.Call(t.Context(), "allowed__search", nil)
	require.NoError(t, err)
	assert.Equal(t, "done", result.Text)
	assert.Equal(t, "allowed__search", inner.gotName)
}

// TestServerStatusDisabled proves a disabled server reports "disabled" without
// consulting the live manager (so a nil manager is safe on that branch).
func TestServerStatusDisabled(t *testing.T) {
	t.Parallel()

	got := (&Service{}).serverStatus(&models.MCPServer{Enabled: false})
	assert.Equal(t, string(mcp.StateDisabled), got)
}

// TestClientForModelWithoutRegistry proves clientForModel reports "not found"
// when no model registry is wired, rather than panicking on a nil registry.
func TestClientForModelWithoutRegistry(t *testing.T) {
	t.Parallel()

	client, ok := (&Service{}).clientForModel("any-model")
	assert.Nil(t, client)
	assert.False(t, ok)
}

// TestToolExecutorForWithoutDisabledServers proves a chat with no disabled
// servers runs against the bare manager, not a filtering wrapper.
func TestToolExecutorForWithoutDisabledServers(t *testing.T) {
	t.Parallel()

	svc := &Service{}

	got := svc.toolExecutorFor(t.Context(), &models.Chat{})
	_, wrapped := got.(filteredToolExecutor)
	assert.False(t, wrapped, "no disabled servers must not wrap the executor")
}
