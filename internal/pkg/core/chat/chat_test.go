package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/heartbeat"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/elelem/elelemtest"
	"github.com/psyb0t/essessey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	echoToolQName = "srv__echo"
	toolServer    = "srv"
	toolNameEcho  = "echo"
	toolCallID    = "call-1"
)

type fakeTools struct {
	tools      []mcp.Tool
	mutex      sync.Mutex
	called     map[string]map[string]any
	results    map[string]*mcp.ToolResult
	errs       map[string]error
	discovered int
	active     int
	maxActive  int
	delay      time.Duration
}

func newFakeTools(tools ...mcp.Tool) *fakeTools {
	return &fakeTools{
		tools:   tools,
		called:  make(map[string]map[string]any),
		results: make(map[string]*mcp.ToolResult),
		errs:    make(map[string]error),
	}
}

func (tools *fakeTools) Tools(_ context.Context) []mcp.Tool {
	tools.mutex.Lock()
	defer tools.mutex.Unlock()

	tools.discovered++

	return tools.tools
}

func (tools *fakeTools) Call(
	_ context.Context,
	name string,
	args map[string]any,
) (*mcp.ToolResult, error) {
	tools.mutex.Lock()
	tools.called[name] = args

	tools.active++
	if tools.active > tools.maxActive {
		tools.maxActive = tools.active
	}

	delay := tools.delay
	result := tools.results[name]
	err := tools.errs[name]
	tools.mutex.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	tools.mutex.Lock()
	tools.active--
	tools.mutex.Unlock()

	if err != nil {
		return nil, err
	}

	if result != nil {
		return result, nil
	}

	return &mcp.ToolResult{Text: "ok:" + name}, nil
}

func (tools *fakeTools) wasCalled(name string) (map[string]any, bool) {
	tools.mutex.Lock()
	defer tools.mutex.Unlock()

	args, ok := tools.called[name]

	return args, ok
}

func run(
	t *testing.T,
	driver *elelemtest.ScriptedDriver,
	tools ToolExecutor,
	request Request,
) (*Result, *essessey.InMemorySink, error) {
	t.Helper()

	sink := &essessey.InMemorySink{}
	deps := Deps{
		Client:    elelem.New(driver),
		Tools:     tools,
		Publisher: essessey.NewPublisher(t.Context(), sink),
	}
	result, err := Run(t.Context(), deps, request)

	return result, sink, err
}

func userRequest() Request {
	return Request{
		MessageID:      "message-1",
		ConversationID: "chat-1",
		Model:          elelem.Model{ID: "mock-model"},
		Prompt:         elelem.NewPrompt().UserText("hello"),
	}
}

func textTurn(text string, reason elelem.FinishReason) elelemtest.Turn {
	return elelemtest.Turn{
		Deltas: []elelem.Delta{{Text: text, FinishReason: reason}},
		Usage: elelem.Usage{
			TokenCounts: elelem.TokenCounts{
				Completion: int64(len(text)),
				Total:      int64(len(text)),
			},
			FinishReason: reason,
		},
	}
}

func toolTurn(calls ...elelem.ToolCall) elelemtest.Turn {
	deltas := make([]elelem.Delta, 0, len(calls))
	for index, call := range calls {
		deltas = append(deltas, elelem.Delta{ToolCall: &elelem.ToolCallDelta{
			Index:     index,
			ID:        call.ID,
			Name:      call.Name,
			Arguments: string(call.Arguments),
		}})
	}

	return elelemtest.Turn{
		Deltas: deltas,
		Usage:  elelem.Usage{FinishReason: elelem.FinishReasonToolCalls},
	}
}

func blockIndices(t *testing.T, events []essessey.Event) []int {
	t.Helper()

	indices := make([]int, 0)

	for _, event := range events {
		if !strings.HasPrefix(event.Event, "content_block_") {
			continue
		}

		var data struct {
			Index int `json:"index"`
		}
		require.NoError(t, json.Unmarshal([]byte(event.Data), &data))
		indices = append(indices, data.Index)
	}

	return indices
}

func startBlockTypes(t *testing.T, events []essessey.Event) []string {
	t.Helper()

	types := make([]string, 0)

	for _, event := range events {
		if event.Event != essessey.EventTypeContentBlockStart {
			continue
		}

		var data struct {
			//nolint:tagliatelle // The SSE protocol uses snake_case.
			ContentBlock struct {
				Type string `json:"type"`
			} `json:"content_block"`
		}
		require.NoError(t, json.Unmarshal([]byte(event.Data), &data))
		types = append(types, data.ContentBlock.Type)
	}

	return types
}

func messageDeltaStopReason(t *testing.T, events []essessey.Event) string {
	t.Helper()

	for _, event := range events {
		if event.Event != essessey.EventTypeMessageDelta {
			continue
		}

		var data struct {
			Delta struct {
				//nolint:tagliatelle // The SSE protocol uses snake_case.
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
		}
		require.NoError(t, json.Unmarshal([]byte(event.Data), &data))

		return data.Delta.StopReason
	}

	t.Fatal("message_delta was not emitted")

	return ""
}

func lastMessageWithRole(
	messages []elelem.Message,
	role elelem.Role,
) elelem.Message {
	var found elelem.Message

	for _, message := range messages {
		if message.Role == role {
			found = message
		}
	}

	return found
}

func TestRunPreservesSSEOrderingAcrossReasoningToolsAndText(t *testing.T) {
	t.Parallel()

	first := toolTurn(elelem.ToolCall{
		ID:        toolCallID,
		Name:      echoToolQName,
		Arguments: json.RawMessage(`{"text":"hello"}`),
	})
	first.Deltas = append(
		[]elelem.Delta{{Reasoning: "inspect"}},
		first.Deltas...,
	)
	driver := elelemtest.NewScriptedDriver(
		first, textTurn("done", elelem.FinishReasonStop),
	)
	tools := newFakeTools(mcp.Tool{Server: toolServer, Name: toolNameEcho})
	tools.results[echoToolQName] = &mcp.ToolResult{Text: "echoed"}

	result, sink, err := run(t, driver, tools, userRequest())
	require.NoError(t, err)
	assert.Equal(t, "done", result.FinalText)
	assert.Equal(t, 2, result.Rounds)
	assert.Equal(t, []string{
		essessey.ContentBlockTypeThinking,
		essessey.ContentBlockTypeToolUse,
		essessey.ContentBlockTypeToolResult,
		essessey.ContentBlockTypeText,
	}, startBlockTypes(t, sink.Events()))
	assert.Equal(t, []int{
		0, 0, 0,
		1, 1, 1,
		2, 2, 2,
		3, 3, 3,
	}, blockIndices(t, sink.Events()))
	assert.Equal(
		t,
		essessey.StopReasonEndTurn,
		messageDeltaStopReason(t, sink.Events()),
	)

	args, called := tools.wasCalled(echoToolQName)
	require.True(t, called)
	assert.Equal(t, map[string]any{"text": "hello"}, args)
}

func TestRunReportsFirstDeltaAndToolDispatchLifecycle(t *testing.T) {
	t.Parallel()

	driver := elelemtest.NewScriptedDriver(
		toolTurn(elelem.ToolCall{
			ID:        toolCallID,
			Name:      echoToolQName,
			Arguments: json.RawMessage(`{"text":"hello"}`),
		}),
		textTurn("done", elelem.FinishReasonStop),
	)
	tools := newFakeTools(mcp.Tool{Server: toolServer, Name: toolNameEcho})
	request := userRequest()
	stages := make([]string, 0, 4)
	request.OnRoundStart = func(
		_ context.Context,
		_ *elelem.RoundEvent,
	) error {
		stages = append(stages, "round")

		return nil
	}
	request.OnFirstDelta = func(_ context.Context) error {
		stages = append(stages, "first_delta")

		return nil
	}
	request.OnAssistantDelta = func(_ context.Context, delta elelem.Delta) {
		if delta.Text != "" || delta.Reasoning != "" {
			stages = append(stages, "assistant_delta")
		}
	}
	request.OnToolCallStart = func(
		_ context.Context,
		_ elelem.ToolCallEvent,
	) error {
		stages = append(stages, "tool_dispatch")

		return nil
	}

	_, _, err := run(t, driver, tools, request)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"round",
		"first_delta",
		"tool_dispatch",
		"round",
		"first_delta",
		"assistant_delta",
	}, stages)
}

func TestRunRefreshesToolsAndForcesFinalRound(t *testing.T) {
	t.Parallel()

	driver := elelemtest.NewScriptedDriver(
		toolTurn(elelem.ToolCall{
			ID:        toolCallID,
			Name:      echoToolQName,
			Arguments: json.RawMessage(`{}`),
		}),
		toolTurn(elelem.ToolCall{
			ID: "call-2", Name: echoToolQName, Arguments: json.RawMessage(`{}`),
		}),
		textTurn("final", elelem.FinishReasonStop),
	)
	tools := newFakeTools(mcp.Tool{Server: toolServer, Name: toolNameEcho})
	request := userRequest()
	request.MaxRounds = 3

	result, _, err := run(t, driver, tools, request)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Rounds)
	assert.Equal(t, 2, tools.discovered)

	requests := driver.Requests()
	require.Len(t, requests, 3)
	assert.Len(t, requests[0].Tools, 1)
	assert.Len(t, requests[1].Tools, 1)
	assert.Empty(t, requests[2].Tools)
}

func TestRunToolErrorsDegradeAndBrainEnvelopeInjects(t *testing.T) {
	t.Parallel()

	t.Run("tool error", func(t *testing.T) {
		t.Parallel()

		driver := elelemtest.NewScriptedDriver(
			toolTurn(elelem.ToolCall{
				ID:        toolCallID,
				Name:      "srv__fail",
				Arguments: json.RawMessage(`{}`),
			}),
			textTurn("recovered", elelem.FinishReasonStop),
		)
		tools := newFakeTools(mcp.Tool{Server: toolServer, Name: "fail"})
		tools.errs["srv__fail"] = assert.AnError

		result, _, err := run(t, driver, tools, userRequest())
		require.NoError(t, err)

		toolResult := lastMessageWithRole(result.Messages, elelem.RoleTool)
		assert.True(t, toolResult.ToolResultIsError)
		assert.Contains(t, toolResult.Text(), "error")

		assert.Equal(t, "recovered", result.FinalText)
	})

	t.Run("brain envelope", func(t *testing.T) {
		t.Parallel()

		driver := elelemtest.NewScriptedDriver(
			toolTurn(elelem.ToolCall{
				ID:        toolCallID,
				Name:      "srv__weather",
				Arguments: json.RawMessage(`{}`),
			}),
			textTurn("presented", elelem.FinishReasonStop),
		)
		tools := newFakeTools(mcp.Tool{Server: toolServer, Name: "weather"})
		tools.results["srv__weather"] = &mcp.ToolResult{
			//nolint:lll // One JSON fixture; splitting literals is prohibited.
			Text: `{"result":"sunny","systemMessageInjection":"render a weather card"}`,
		}

		result, _, err := run(t, driver, tools, userRequest())
		require.NoError(t, err)

		toolResult := lastMessageWithRole(result.Messages, elelem.RoleTool)
		assert.Equal(t, "sunny", toolResult.Text())

		injection := lastMessageWithRole(result.Messages, elelem.RoleSystem)
		assert.Equal(t, "render a weather card", injection.Text())
		assert.NotNil(t, injection.Injection)

		requests := driver.Requests()
		require.Len(t, requests, 2)
		requestInjection := lastMessageWithRole(
			requests[1].Messages,
			elelem.RoleSystem,
		)
		assert.Equal(t, "render a weather card", requestInjection.Text())
	})
}

func TestRunBoundsParallelToolExecutionAndPreservesWireOrder(t *testing.T) {
	t.Parallel()

	const callCount = 12

	calls := make([]elelem.ToolCall, 0, callCount)

	mcpTools := make([]mcp.Tool, 0, callCount)
	for index := range callCount {
		name := "tool-" + string(rune('a'+index))
		calls = append(calls, elelem.ToolCall{
			ID:        "call-" + name,
			Name:      toolServer + "__" + name,
			Arguments: json.RawMessage(`{}`),
		})
		mcpTools = append(mcpTools, mcp.Tool{Server: toolServer, Name: name})
	}

	driver := elelemtest.NewScriptedDriver(
		toolTurn(calls...),
		textTurn("done", elelem.FinishReasonStop),
	)
	tools := newFakeTools(mcpTools...)
	tools.delay = 15 * time.Millisecond

	_, sink, err := run(t, driver, tools, userRequest())
	require.NoError(t, err)
	assert.Greater(t, tools.maxActive, 1)
	assert.LessOrEqual(t, tools.maxActive, maxConcurrentTools)

	indices := blockIndices(t, sink.Events())
	for index := 1; index < len(indices); index++ {
		assert.GreaterOrEqual(t, indices[index], indices[index-1])
	}
}

func TestRunRepairsTranscriptAndPropagatesParams(t *testing.T) {
	t.Parallel()

	temperature := 0.4
	maxOutput := int64(1000)
	params := elelem.GenerationParams{
		Temperature:     &temperature,
		ReasoningEffort: elelem.ReasoningEffortHigh,
		MaxOutputTokens: &maxOutput,
	}
	driver := elelemtest.NewScriptedDriver(
		textTurn("done", elelem.FinishReasonStop),
	).
		WithCapabilities(elelem.Capabilities{
			SupportsSamplingParams:  true,
			SupportsReasoningEffort: true,
		})
	request := userRequest()
	request.Model.SupportsReasoning = true
	request.Params = params
	request.Prompt = elelem.NewPrompt().
		WithSystem("sticky").
		Add(elelem.Message{
			Role:       elelem.RoleTool,
			ToolCallID: "orphan",
			Content:    elelem.Text("bad"),
		}).
		UserText("hello")

	_, _, err := run(t, driver, newFakeTools(), request)
	require.NoError(t, err)

	recorded := driver.Requests()
	require.Len(t, recorded, 1)
	assert.Equal(t, params, recorded[0].Params)

	for _, message := range recorded[0].Messages {
		assert.NotEqual(t, "orphan", message.ToolCallID)
	}

	assert.Equal(t, "sticky", recorded[0].Messages[0].Text())
}

func TestRunRoundHookSeesExactTranscriptAndFinalTextIsNotStale(t *testing.T) {
	t.Parallel()

	first := toolTurn(elelem.ToolCall{
		ID:        toolCallID,
		Name:      echoToolQName,
		Arguments: json.RawMessage(`{}`),
	})
	first.Deltas = append(
		[]elelem.Delta{{Text: "intermediate"}},
		first.Deltas...,
	)
	driver := elelemtest.NewScriptedDriver(
		first,
		textTurn("", elelem.FinishReasonStop),
	)
	tools := newFakeTools(mcp.Tool{Server: toolServer, Name: toolNameEcho})
	request := userRequest()
	roundMessages := make([][]elelem.Message, 0)
	request.OnRoundStart = func(
		_ context.Context,
		event *elelem.RoundEvent,
	) error {
		roundMessages = append(roundMessages, event.Messages)

		return nil
	}

	result, _, err := run(t, driver, tools, request)
	require.NoError(t, err)
	assert.Empty(t, result.FinalText)
	require.Len(t, roundMessages, 2)
	assert.Len(t, roundMessages[0], 1)
	assert.Greater(t, len(roundMessages[1]), len(roundMessages[0]))
	assert.Equal(t, driver.Requests()[1].Messages, roundMessages[1])
}

func TestMapStopReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason elelem.FinishReason
		calls  []elelem.ToolCall
		want   essessey.StopReason
	}{
		{
			name:   "stop",
			reason: elelem.FinishReasonStop,
			want:   essessey.StopReasonEndTurn,
		},
		{name: "unset", want: essessey.StopReasonEndTurn},
		{
			name:  "tools",
			calls: []elelem.ToolCall{{ID: toolCallID}},
			want:  essessey.StopReasonToolUse,
		},
		{
			name:   "length wins",
			reason: elelem.FinishReasonLength,
			calls:  []elelem.ToolCall{{ID: toolCallID}},
			want:   essessey.StopReasonMaxTokens,
		},
		{
			name:   "context exceeded",
			reason: elelem.FinishReasonContextExceeded,
			want:   essessey.StopReasonMaxTokens,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, mapStopReason(test.reason, test.calls))
		})
	}
}

func TestRunTruncatedFinishReasonReachesEpilogue(t *testing.T) {
	t.Parallel()

	driver := elelemtest.NewScriptedDriver(
		textTurn("partial", elelem.FinishReasonLength),
	)
	_, sink, err := run(t, driver, newFakeTools(), userRequest())
	require.NoError(t, err)
	assert.Equal(
		t,
		essessey.StopReasonMaxTokens,
		messageDeltaStopReason(t, sink.Events()),
	)
}

func TestSplitEnvelope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		raw           string
		wantContent   string
		wantInjection string
	}{
		{name: "plain", raw: "hello", wantContent: "hello"},
		{
			name:        "without injection",
			raw:         `{"result":"x"}`,
			wantContent: `{"result":"x"}`,
		},
		{
			name:          "string result",
			raw:           `{"result":"x","systemMessageInjection":"steer"}`,
			wantContent:   "x",
			wantInjection: "steer",
		},
		{
			name: "object result",
			//nolint:lll // One JSON fixture; splitting literals is prohibited.
			raw:           `{"result":{"a":1},"systemMessageInjection":"steer"}`,
			wantContent:   `{"a":1}`,
			wantInjection: "steer",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			content, injection := splitEnvelope(test.raw)
			assert.Equal(t, test.wantContent, content)
			assert.Equal(t, test.wantInjection, injection)
		})
	}
}

func toolRequestNames(tools []elelem.Tool) []string {
	names := make([]string, len(tools))
	for index, tool := range tools {
		names[index] = tool.Name
	}

	return names
}

func TestRunParamsToolsAndSystemPromptReachEveryModelRound(t *testing.T) {
	t.Parallel()

	temperature := 0.4
	maxOutput := int64(1000)
	params := elelem.GenerationParams{
		Temperature:     &temperature,
		ReasoningEffort: elelem.ReasoningEffortHigh,
		MaxOutputTokens: &maxOutput,
	}
	driver := elelemtest.NewScriptedDriver(
		toolTurn(elelem.ToolCall{
			ID:        toolCallID,
			Name:      echoToolQName,
			Arguments: json.RawMessage(`{"text":"hi"}`),
		}),
		textTurn("done", elelem.FinishReasonStop),
	).WithCapabilities(elelem.Capabilities{
		SupportsSamplingParams:  true,
		SupportsReasoningEffort: true,
	})
	tools := newFakeTools(mcp.Tool{Server: toolServer, Name: toolNameEcho})
	request := userRequest()
	request.Model.SupportsReasoning = true
	request.MaxRounds = 2
	request.Params = params
	request.Prompt = elelem.NewPrompt().
		WithSystem("SYSTEM-PROMPT-MARKER").
		UserText("hello")

	result, _, err := run(t, driver, tools, request)
	require.NoError(t, err)
	assert.Equal(t, "done", result.FinalText)
	assert.Equal(t, 2, result.Rounds)

	requests := driver.Requests()
	require.Len(t, requests, 2)
	assert.Equal(t, params, requests[0].Params)
	assert.Equal(t, params, requests[1].Params)
	assert.Equal(t, "SYSTEM-PROMPT-MARKER", requests[0].Messages[0].Text())
	assert.Equal(t, "SYSTEM-PROMPT-MARKER", requests[1].Messages[0].Text())
	assert.Greater(t, len(requests[1].Messages), len(requests[0].Messages))
	assert.Equal(
		t,
		[]string{echoToolQName},
		toolRequestNames(requests[0].Tools),
	)
	assert.Empty(t, requests[1].Tools)

	args, called := tools.wasCalled(echoToolQName)
	require.True(t, called)
	assert.Equal(t, map[string]any{"text": "hi"}, args)
}

// delayedDriver paces the scripted deltas so the run stalls long enough for
// the idle heartbeat to fire.
type delayedDriver struct {
	*elelemtest.ScriptedDriver
	delay time.Duration
}

func (driver *delayedDriver) Stream(
	ctx context.Context,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return driver.ScriptedDriver.Stream(ctx, request, driver.pace(onDelta))
}

// Complete overrides the embedded one rather than inheriting it. Embedding
// satisfies the interface either way, so the compiler would not have said
// anything — and a run with streaming off would then have taken the UNPACED
// path, making this test quietly stop exercising the delay it exists for.
func (driver *delayedDriver) Complete(
	ctx context.Context,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return driver.ScriptedDriver.Complete(ctx, request, driver.pace(onDelta))
}

// pace sleeps between deltas so the run stalls long enough for the idle
// heartbeat to fire. The first delta goes straight through — the stall has to
// land mid-turn, not before it starts.
func (driver *delayedDriver) pace(
	onDelta func(elelem.Delta) error,
) func(elelem.Delta) error {
	seen := 0

	return func(delta elelem.Delta) error {
		if seen > 0 {
			time.Sleep(driver.delay)
		}

		seen++

		return onDelta(delta)
	}
}

func countLogLines(t *testing.T, buffer *bytes.Buffer, message string) int {
	t.Helper()

	count := 0

	for line := range strings.SplitSeq(buffer.String(), "\n") {
		if line == "" {
			continue
		}

		var record map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &record))

		if record["msg"] == message {
			count++
		}
	}

	return count
}

func TestRunIdleHeartbeatFiresDuringSlowLLMStream(t *testing.T) {
	originalInterval := heartbeat.Interval
	heartbeat.Interval = 20 * time.Millisecond

	t.Cleanup(func() {
		heartbeat.Interval = originalInterval
	})

	// scope.GetLogger builds from slog.Default(), so capturing what the run
	// emits means swapping the process default and restoring it after. Not
	// parallel-safe, which is why this test does not call t.Parallel.
	buffer := &bytes.Buffer{}

	previousLogger := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(buffer, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	ctx := context.Background()
	mock := elelemtest.NewScriptedDriver(elelemtest.Turn{
		Deltas: []elelem.Delta{{Text: "thinking..."}, {Text: " done"}},
		Usage:  elelem.Usage{FinishReason: elelem.FinishReasonStop},
	})
	driver := &delayedDriver{ScriptedDriver: mock, delay: 90 * time.Millisecond}
	sink := &essessey.InMemorySink{}
	result, err := Run(ctx, Deps{
		Client:    elelem.New(driver),
		Tools:     newFakeTools(),
		Publisher: essessey.NewPublisher(ctx, sink),
	}, userRequest())
	require.NoError(t, err)
	assert.Equal(t, "thinking... done", result.FinalText)
	assert.GreaterOrEqual(
		t,
		countLogLines(t, buffer, "still waiting on LLM stream"),
		2,
	)
}
