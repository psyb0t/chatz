//go:build real

package realtest

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	upstreamTimeout = 90 * time.Second

	// A static marker makes a real completion distinguishable from an empty or
	// truncated one without putting real-world data in a reusable prompt.
	//nolint:lll // one prompt line; splitting the literal is banned
	upstreamPrompt   = "Reply with exactly this marker and nothing else: STREAM_OK"
	upstreamExpected = "stream_ok"

	// upstreamMaxOutputTokens has to leave room for a REASONING model to think
	// before it answers. The default upstream model is one, and at 64 tokens it
	// intermittently spent the entire budget reasoning and returned empty text
	// with finish_reason=length — a flake that looks exactly like a broken
	// stream assembler and costs a bisect to tell apart. The cap still bounds
	// the call; it just stops bounding it below the model's own overhead.
	upstreamMaxOutputTokens = 512

	// Cheap models keep the transport, translation, and accounting probe
	// inexpensive. Override per upstream with CHATZ_REAL_MODEL_<UPSTREAM NAME>.
	upstreamOpenAIModel    = "groq-gpt-oss-20b"
	upstreamAnthropicModel = "glm-4.5-air"

	// A tool the model can only satisfy by CALLING it — the answer is not
	// derivable from the prompt, so a model that just replies in prose fails
	// the assertion rather than accidentally passing it.
	upstreamToolName   = "get_build_marker"
	upstreamToolPrompt = "Call get_build_marker for host prod-1. " +
		"Do not answer in prose."
	upstreamToolArgument = "prod-1"

	upstreamToolDescription = "Return the build marker for a host."

	// The argument schema for upstreamToolName. One required string keeps every
	// provider's strictness setting happy while still giving the arguments
	// something to get wrong.
	//
	//nolint:lll // one JSON schema literal; splitting it hurts more than it helps
	upstreamToolSchema = `{"type":"object","properties":{"host":{"type":"string"}},"required":["host"],"additionalProperties":false}`
)

// realUpstreams returns EVERY configured upstream.
//
// realLLM covers upstreams[0] only, which is the right scope for the agentic
// loop but means a second upstream can be broken without any test noticing —
// and a broken upstream is silently dropped from the registry at boot, so the
// app starts "fine" with its models missing.
func realUpstreams(t *testing.T) []config.Upstream {
	t.Helper()

	cfg := config.Config{UpstreamsJSON: os.Getenv("CHATZ_UPSTREAMS")}

	upstreams, err := cfg.Upstreams()
	require.NoError(t, err, "decode CHATZ_UPSTREAMS")

	if len(upstreams) == 0 {
		t.Skip("no upstreams configured — set CHATZ_UPSTREAMS (the same .env " +
			"`make run` uses) to run the real-LLM tests")
	}

	return upstreams
}

// upstreamTestModel picks the model this upstream answers with.
//
// An explicit per-upstream override wins; otherwise the cheap default for the
// provider, and only if the upstream does not serve it does this fall back to
// whatever it lists first. The fallback keeps the test useful against an
// upstream nobody anticipated, without billing a frontier model by default.
func upstreamTestModel(
	ctx context.Context,
	t *testing.T,
	up config.Upstream,
	driver elelem.Driver,
) string {
	t.Helper()

	if override := os.Getenv(
		"CHATZ_REAL_MODEL_" + strings.ToUpper(up.Name),
	); override != "" {
		return override
	}

	preferred := upstreamOpenAIModel
	if up.Provider == config.UpstreamProviderAnthropic {
		preferred = upstreamAnthropicModel
	}

	models, err := driver.ListModels(ctx)
	require.NoError(t, err, "%s: list models", up.Name)
	require.NotEmpty(t, models, "%s: no models", up.Name)

	if slices.Contains(models, preferred) {
		return preferred
	}

	t.Logf("%s does not serve %q; falling back to %q",
		up.Name, preferred, models[0])

	return models[0]
}

// Every configured upstream must be reachable and, when configured,
// authenticated. ListModels is the cheapest call proving both, and it is what
// upstreams.Discover calls at boot.
func TestRealUpstreamsListModels(t *testing.T) {
	for _, up := range realUpstreams(t) {
		t.Run(up.Name, func(t *testing.T) {
			if up.APIKeyEnv != "" && up.APIKey() == "" {
				t.Skipf("%s: %s unset", up.Name, up.APIKeyEnv)
			}

			ctx, cancel := context.WithTimeout(t.Context(), upstreamTimeout)
			defer cancel()

			models, err := realDriverFor(t, up).ListModels(ctx)
			require.NoError(t, err, "%s: list models", up.Name)
			require.NotEmpty(t, models, "%s: no models", up.Name)

			t.Logf("%s (%s) serves %d models, first: %s",
				up.Name, up.BaseURL, len(models), models[0])
		})
	}
}

// A full streamed completion against each upstream.
//
// ListModels only proves the endpoint answers. This exercises the driver's
// request translation, its SSE decoding, and its usage accounting against that
// provider's real wire format — which is where the provider-specific bugs are.
func TestRealUpstreamsCompleteAPrompt(t *testing.T) {
	for _, up := range realUpstreams(t) {
		t.Run(up.Name, func(t *testing.T) {
			if up.APIKeyEnv != "" && up.APIKey() == "" {
				t.Skipf("%s: %s unset", up.Name, up.APIKeyEnv)
			}

			ctx, cancel := context.WithTimeout(t.Context(), upstreamTimeout)
			defer cancel()

			driver := realDriverFor(t, up)

			modelID := upstreamTestModel(ctx, t, up, driver)

			t.Logf("%s: completing against %s", up.Name, modelID)

			client := elelem.New(
				driver,
				elelem.WithDefaultModel(elelem.Model{ID: modelID}),
			)

			var streamed strings.Builder

			response, err := elelem.NewRequest(client).
				WithPrompt(elelem.NewPrompt().UserText(upstreamPrompt)).
				WithMaxOutputTokens(upstreamMaxOutputTokens).
				OnDelta(func(_ context.Context, delta elelem.Delta) error {
					streamed.WriteString(delta.Text)

					return nil
				}).
				Run(ctx)
			require.NoError(t, err, "%s: stream", up.Name)
			require.NotNil(t, response)

			// A real completion came back, not an empty 200.
			assert.Contains(t,
				strings.ToLower(response.Text), upstreamExpected,
				"%s: unexpected answer %q", up.Name, response.Text)

			// The two channels are assembled separately, so a driver can
			// produce one without the other.
			assert.Equal(t, response.Text, streamed.String(),
				"%s: streamed text differs from the final text", up.Name)

			// Zero tokens means accounting silently failed, which makes cost
			// and budgeting blind.
			assert.Positive(t, response.Usage.Prompt,
				"%s: no prompt tokens reported", up.Name)
			assert.Positive(t, response.Usage.Completion,
				"%s: no completion tokens reported", up.Name)
			assert.True(t, response.FinishReason.IsTerminal(),
				"%s: non-terminal finish reason %q",
				up.Name, response.FinishReason)

			t.Logf(
				"%s: %q | model=%s prompt=%d completion=%d reason=%s",
				up.Name,
				strings.TrimSpace(response.Text),
				response.Model,
				response.Usage.Prompt,
				response.Usage.Completion,
				response.FinishReason,
			)
		})
	}
}

// The same prompt through BOTH transports, against the real provider.
//
// elelem.WithStreaming(false) exists for backends that cannot serve a token
// stream, and its whole promise is that it changes the TRANSPORT and nothing
// else: the driver feeds the finished response through the same delta callback,
// so callbacks still fire and the Response has the same shape. Unit tests prove
// that against a scripted driver, which by construction cannot disagree with
// itself — only a real provider can, because only a real provider has two
// genuinely different wire formats behind the two calls (an SSE event sequence
// vs a single JSON body).
//
// Both cases hit one model on one upstream, so any difference is attributable
// to the transport rather than to the model or the endpoint.
func TestRealUpstreamsStreamingOffMatchesStreamingOn(t *testing.T) {
	for _, up := range realUpstreams(t) {
		t.Run(up.Name, func(t *testing.T) {
			if up.APIKeyEnv != "" && up.APIKey() == "" {
				t.Skipf("%s: %s unset", up.Name, up.APIKeyEnv)
			}

			ctx, cancel := context.WithTimeout(t.Context(), upstreamTimeout)
			defer cancel()

			driver := realDriverFor(t, up)
			modelID := upstreamTestModel(ctx, t, up, driver)

			client := elelem.New(
				driver,
				elelem.WithDefaultModel(elelem.Model{ID: modelID}),
			)

			t.Logf("%s: comparing transports on %s", up.Name, modelID)

			testCases := []struct {
				name      string
				streaming bool
			}{
				{"streaming", true},
				{"not streaming", false},
			}

			// Keyed by streaming so the cross-transport comparison below reads
			// the same values the subtests asserted on.
			responses := map[bool]*elelem.Response{}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					// delivered is what a renderer would have drawn; the
					// Response is what the engine assembled. Different code
					// builds each, so a transport producing one without the
					// other looks healthy from either one alone.
					var delivered strings.Builder

					response, err := elelem.NewRequest(client).
						WithPrompt(elelem.NewPrompt().UserText(upstreamPrompt)).
						WithMaxOutputTokens(upstreamMaxOutputTokens).
						WithStreaming(tc.streaming).
						OnDelta(func(
							_ context.Context,
							delta elelem.Delta,
						) error {
							delivered.WriteString(delta.Text)

							return nil
						}).
						Run(ctx)
					require.NoError(t, err,
						"%s: streaming=%t", up.Name, tc.streaming)
					require.NotNil(t, response)

					responses[tc.streaming] = response

					// The model actually answered, on both transports.
					assert.Contains(t,
						strings.ToLower(response.Text), upstreamExpected,
						"%s: streaming=%t unexpected answer %q",
						up.Name, tc.streaming, response.Text)

					// The load-bearing assertion. OnDelta firing on the
					// non-streaming path is the entire reason Driver.Complete
					// takes a delta callback instead of returning a Message —
					// if that regressed, every renderer built on callbacks
					// would draw nothing while the Response looked correct.
					assert.Equal(t, response.Text, delivered.String(),
						"%s: streaming=%t delivered %q via OnDelta but "+
							"assembled %q",
						up.Name, tc.streaming,
						delivered.String(), response.Text)

					// Accounting is not a streaming-path feature.
					assert.Positive(t, response.Usage.Prompt,
						"%s: streaming=%t no prompt tokens",
						up.Name, tc.streaming)
					assert.Positive(t, response.Usage.Completion,
						"%s: streaming=%t no completion tokens",
						up.Name, tc.streaming)

					t.Logf("%s streaming=%t: %q | prompt=%d completion=%d "+
						"reason=%s",
						up.Name, tc.streaming,
						strings.TrimSpace(response.Text),
						response.Usage.Prompt, response.Usage.Completion,
						response.FinishReason)
				})
			}

			// Comparing the runs is only meaningful if both produced one; a
			// skipped or failed subtest must not let this pass by default.
			streamed, streamedOK := responses[true]
			completed, completedOK := responses[false]
			require.True(t, streamedOK && completedOK,
				"%s: both transports must have produced a response", up.Name)

			// A model is free to word two answers differently, so the text is
			// not comparable across runs — the finish reason is. It is read
			// from a different place in each wire format (a terminal SSE chunk
			// vs a top-level stop reason), and mapping one of them wrong is
			// exactly the bug that pairing the transports catches.
			assert.Equal(t, streamed.FinishReason, completed.FinishReason,
				"%s: finish reason differs by transport", up.Name)
		})
	}
}

// A real TOOL CALL through both transports.
//
// The text-parity test above proves the callback plumbing but exercises the
// easiest possible payload. Tool calls are where the two paths genuinely differ
// in construction: streaming assigns a call's index as each tool-call fragment
// opens, while the non-streaming path has no per-call index on the wire at all
// and has to derive one from array position. Get that wrong and results pair to
// calls that do not exist — a failure that appears one ROUND later, as a
// provider rejecting the next request, and is miserable to trace back.
//
// Manual driving on purpose: the assertion is about what came back from the
// provider, so letting the engine run the loop would bury it behind a tool
// execution this test does not need.
func TestRealUpstreamsToolCallMatchesAcrossTransports(t *testing.T) {
	for _, up := range realUpstreams(t) {
		t.Run(up.Name, func(t *testing.T) {
			if up.APIKeyEnv != "" && up.APIKey() == "" {
				t.Skipf("%s: %s unset", up.Name, up.APIKeyEnv)
			}

			ctx, cancel := context.WithTimeout(t.Context(), upstreamTimeout)
			defer cancel()

			driver := realDriverFor(t, up)
			modelID := upstreamTestModel(ctx, t, up, driver)

			client := elelem.New(
				driver,
				elelem.WithDefaultModel(elelem.Model{ID: modelID}),
			)

			testCases := []struct {
				name      string
				streaming bool
			}{
				{"streaming", true},
				{"not streaming", false},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					response, err := elelem.NewRequest(client).
						WithPrompt(
							elelem.NewPrompt().UserText(upstreamToolPrompt),
						).
						WithMaxOutputTokens(upstreamMaxOutputTokens).
						WithStreaming(tc.streaming).
						WithTool(elelem.Tool{
							Name:        upstreamToolName,
							Description: upstreamToolDescription,
							ArgumentsSchema: json.RawMessage(
								upstreamToolSchema,
							),
						}).
						Run(ctx)
					require.NoError(t, err,
						"%s: streaming=%t", up.Name, tc.streaming)
					require.NotNil(t, response)

					require.NotEmpty(t, response.ToolCalls,
						"%s: streaming=%t the model returned no tool call",
						up.Name, tc.streaming)

					call := response.ToolCalls[0]

					assert.Equal(t, upstreamToolName, call.Name,
						"%s: streaming=%t wrong tool called",
						up.Name, tc.streaming)

					// An id is what a tool RESULT is addressed to on the next
					// request. Empty here and the following turn is rejected by
					// the provider, not by this test.
					assert.NotEmpty(t, call.ID,
						"%s: streaming=%t tool call has no id",
						up.Name, tc.streaming)

					// Arguments assemble from fragments on the streaming path
					// and arrive whole on the other. Both must end up as valid
					// JSON carrying what was asked for — a truncated or
					// double-concatenated buffer fails right here.
					var args struct {
						Host string `json:"host"`
					}

					require.NoError(t,
						json.Unmarshal(call.Arguments, &args),
						"%s: streaming=%t arguments are not valid JSON: %s",
						up.Name, tc.streaming, call.Arguments)
					assert.Equal(t, upstreamToolArgument, args.Host,
						"%s: streaming=%t wrong argument",
						up.Name, tc.streaming)

					assert.Equal(t,
						elelem.FinishReasonToolCalls, response.FinishReason,
						"%s: streaming=%t finish reason should be tool_calls",
						up.Name, tc.streaming)

					t.Logf("%s streaming=%t: %s(%s) id=%s reason=%s",
						up.Name, tc.streaming, call.Name, call.Arguments,
						call.ID, response.FinishReason)
				})
			}
		})
	}
}
