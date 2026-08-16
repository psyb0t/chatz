//go:build real

// Proves the generated GenUI system prompt actually makes a real model emit a
// well-formed ```spec block (json-render JSONL) instead of markdown/HTML — the
// whole point of injecting the catalog. Opt-in: runs against the same .env
// `make run` uses (CHATZ_UPSTREAMS + the key env it names).
package realtest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	chatcore "github.com/psyb0t/chatz/internal/pkg/core/chat"
	"github.com/psyb0t/chatz/internal/pkg/core/chats/prompts"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/essessey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const genUITestPrompt = "Render these three KPIs as stat components inside a card: Occupancy 92%, Vacancy 8%, Revenue $1.2M. Use the UI spec format, not markdown." //nolint:lll // one-line user prompt

var errNoTools = errors.New("genui test uses no tools")

// emptyTools is a ToolExecutor with no tools — the genui turn is pure text, no
// MCP involved.
type emptyTools struct{}

func (emptyTools) Tools(context.Context) []mcp.Tool {
	return nil
}

func (emptyTools) Call(
	context.Context,
	string,
	map[string]any,
) (*mcp.ToolResult, error) {
	return nil, errNoTools
}

func TestReal_GenUISpecEmission(t *testing.T) {
	client, model := realLLM(t)

	ctx, cancel := context.WithTimeout(context.Background(), turnTimeout)
	defer cancel()

	sink := &essessey.InMemorySink{}
	pub := essessey.NewPublisher(ctx, sink)

	// The same system prompt the chat service builds: base + generated catalog.
	system := "You are chatz, a helpful assistant.\n\n" +
		prompts.GenUIInstructions()

	result, err := chatcore.Run(ctx, chatcore.Deps{
		Client:    client,
		Tools:     emptyTools{},
		Publisher: pub,
	}, chatcore.Request{
		MessageID:      "msg_real_genui",
		ConversationID: "conv_real_genui",
		Model:          model,
		Prompt: elelem.NewPrompt().
			WithSystem(system).
			UserText(genUITestPrompt),
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	out := result.FinalText
	require.Contains(t, out, "```spec",
		"model should emit a ```spec fence; got:\n%s", out)

	spec := extractSpecBlock(t, out)

	assert.Contains(t, spec, `"op":"add"`, "spec should use add patches")
	assert.Contains(t, spec, `"/root"`, "spec should set a root element")
	assert.Contains(t, spec, "/elements/", "spec should define elements")
	assert.Contains(t, spec, `"type":"Stat"`,
		"the requested KPIs should render as Stat components")

	assertSpecPatchesAreJSON(t, spec)

	t.Logf("rounds: %d, total tokens: %d", result.Rounds, result.Usage.Total)
}

// extractSpecBlock returns the body between the first ```spec fence and its
// closing ```.
func extractSpecBlock(t *testing.T, out string) string {
	t.Helper()

	const open = "```spec"

	start := strings.Index(out, open)
	require.GreaterOrEqual(t, start, 0, "no ```spec fence found")

	body := out[start+len(open):]
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		body = body[nl+1:]
	}

	end := strings.Index(body, "```")
	require.GreaterOrEqual(t, end, 0, "```spec fence not closed")

	return body[:end]
}

// assertSpecPatchesAreJSON requires at least the root + one element line to
// parse as JSON. Non-object lines are tolerated to avoid over-fitting a live
// model's exact formatting.
func assertSpecPatchesAreJSON(t *testing.T, spec string) {
	t.Helper()

	var valid int

	for _, line := range strings.Split(spec, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var patch map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &patch),
			"spec patch line is not valid JSON: %s", line)

		valid++
	}

	assert.GreaterOrEqual(t, valid, 2,
		"expected at least a /root patch plus one element patch")
}
