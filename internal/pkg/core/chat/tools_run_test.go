package chat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// panicToolExecutor panics on every Call, exercising runMCPTool's recover
// guard: a misbehaving tool must not take down the turn.
type panicToolExecutor struct{}

func (panicToolExecutor) Tools(context.Context) []mcp.Tool { return nil }

func (panicToolExecutor) Call(
	context.Context,
	string,
	map[string]any,
) (*mcp.ToolResult, error) {
	panic("tool blew up")
}

// TestRunMCPTool covers every branch of the tool-execution wrapper: a valid
// call, malformed arguments (decode fails), an executor error, and a panicking
// executor. Each non-happy branch must feed an is-error result back to the
// model rather than returning a Go error, so the turn continues.
func TestRunMCPTool(t *testing.T) {
	t.Parallel()

	t.Run("valid args returns the tool result", func(t *testing.T) {
		t.Parallel()

		tools := newFakeTools()
		tools.results["search"] = &mcp.ToolResult{Text: "found it"}

		out, err := runMCPTool(t.Context(), tools, elelem.ToolInput{
			Name:      "search",
			Arguments: json.RawMessage(`{"q":"hi"}`),
		})
		require.NoError(t, err)
		assert.False(t, out.IsError)
		assert.Equal(t, "found it", out.Content)

		args, ok := tools.wasCalled("search")
		require.True(t, ok)
		assert.Equal(t, "hi", args["q"])
	})

	t.Run("malformed args feed an error to the model", func(t *testing.T) {
		t.Parallel()

		out, err := runMCPTool(t.Context(), newFakeTools(), elelem.ToolInput{
			Name:      "search",
			Arguments: json.RawMessage(`{not json`),
		})
		require.NoError(t, err)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "error")
	})

	t.Run("executor error feeds an error to the model", func(t *testing.T) {
		t.Parallel()

		tools := newFakeTools()
		tools.errs["search"] = assert.AnError

		out, err := runMCPTool(t.Context(), tools, elelem.ToolInput{
			Name:      "search",
			Arguments: json.RawMessage(`{}`),
		})
		require.NoError(t, err)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "error")
	})

	t.Run("panic is recovered as an error result", func(t *testing.T) {
		t.Parallel()

		out, err := runMCPTool(
			t.Context(),
			panicToolExecutor{},
			elelem.ToolInput{
				Name:      "search",
				Arguments: json.RawMessage(`{}`),
			},
		)
		require.NoError(t, err)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "panicked")
	})
}
