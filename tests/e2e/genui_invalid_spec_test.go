//go:build e2e

package e2e

import (
	"strings"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	invalidSpecPrompt     = "Show me the service status as a UI."
	invalidSpecSendSel    = "[data-testid=send]"
	invalidSpecBlockSel   = "[data-testid=spec-block]"
	invalidSpecErrorSel   = "[data-testid=spec-error]"
	invalidSpecWaitSec    = 60
	invalidSpecSettleMS   = 1500
	invalidSpecFenceOpen  = "```spec"
	invalidSpecFenceClose = "```"
)

// The card element is keyed "cardMain" in both payloads. Only the /root value
// differs: the broken one names "main", an element that is never defined, which
// is precisely what a real model emitted in production. json-render resolves
// the tree from /root, finds nothing, and draws nothing — so before the
// renderer validated the spec this failure was completely invisible, on first
// paint and on every reload of the persisted message.
const (
	invalidSpecRootLine = `{"op":"add","path":"/root","value":"main"}`
	validSpecRootLine   = `{"op":"add","path":"/root","value":"cardMain"}`
	//nolint:lll // one JSONL patch = one physical line, by the spec's contract
	specCardLine = `{"op":"add","path":"/elements/cardMain","value":{"type":"Card","props":{"id":null,"title":"Service Status"},"children":[]}}`
)

func genUISpecResponse(rootLine string) string {
	return strings.Join([]string{
		"Here is the current status.",
		"",
		invalidSpecFenceOpen,
		rootLine,
		specCardLine,
		invalidSpecFenceClose,
	}, "\n")
}

// Counts what actually reached the DOM: rendered json-render components, and
// whether the invalid-spec notice is present. A silently-dropped spec scores
// zero components AND zero errors — the exact state this test exists to catch.
const genUISpecProbeJS = `(() => {
	const blocks = document.querySelectorAll('[data-testid=spec-block]');
	const spec = [...blocks].at(-1);
	return JSON.stringify({
		components: spec
			? spec.querySelectorAll('[data-jr-type]').length
			: 0,
		errors: document.querySelectorAll('[data-testid=spec-error]').length,
	});
})()`

type genUISpecProbe struct {
	Components int `json:"components"`
	Errors     int `json:"errors"`
}

func genUISpecProbeAfterSend(
	t *testing.T,
	responseText string,
) genUISpecProbe {
	t.Helper()

	ctx := t.Context()
	stack, client := newStack(
		t,
		testinfra.E2EOptions{UpstreamResponseText: responseText},
		testinfra.BrowserOptions{},
	)
	setupAdmin(ctx, t, stack, client)

	require.NoError(t, client.Fill(ctx, composerInput, invalidSpecPrompt))
	require.NoError(t, client.Eval(ctx, submitJS(invalidSpecSendSel)))

	err := client.WaitForElement(
		ctx,
		invalidSpecBlockSel,
		stateVisible,
		invalidSpecWaitSec,
	)
	if err != nil {
		logContainerOutput(ctx, t, "app", stack.App)
		logContainerOutput(ctx, t, "fake upstream", stack.Upstream)

		require.NoError(t, err)
	}

	// The spec block appears as soon as the fence opens; give the patches and
	// the post-close validation a beat to land before probing.
	require.NoError(t, client.Eval(ctx, settleJS(invalidSpecSettleMS)))

	var probe genUISpecProbe
	require.NoError(t, client.EvalJSON(ctx, genUISpecProbeJS, &probe))

	return probe
}

// TestGenUIValidSpecRendersComponents is the positive half: a well-formed spec
// must put real components in the DOM. Without it the negative test below could
// pass on a renderer that draws nothing for everything.
func TestGenUIValidSpecRendersComponents(t *testing.T) {
	t.Parallel()

	probe := genUISpecProbeAfterSend(t, genUISpecResponse(validSpecRootLine))

	assert.Positive(t, probe.Components,
		"a valid spec must render at least one json-render component")
	assert.Equal(t, 0, probe.Errors,
		"a valid spec must not surface the invalid-spec notice")
}

// TestGenUIInvalidSpecSurfacesError is the regression: a spec whose /root names
// an undefined element must tell the reader, not render an empty block.
func TestGenUIInvalidSpecSurfacesError(t *testing.T) {
	t.Parallel()

	probe := genUISpecProbeAfterSend(t, genUISpecResponse(invalidSpecRootLine))

	assert.Positive(t, probe.Errors,
		"an unresolvable /root must surface the invalid-spec notice")
	assert.Equal(t, 0, probe.Components,
		"an unresolvable /root resolves to no components at all")
}
