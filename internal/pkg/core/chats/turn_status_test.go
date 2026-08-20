package chats

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/chatz/internal/pkg/core/chats/fixedresponses"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/mcp"
	"github.com/psyb0t/chatz/internal/pkg/upstreams"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/elelem/elelemtest"
	"github.com/psyb0t/essessey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishTurnStatus(t *testing.T) {
	t.Parallel()

	sink := essessey.NewInMemorySink()
	pub := essessey.NewPublisher(t.Context(), sink)

	require.NoError(t, publishTurnStatus(pub, turnStatusConnecting))

	events := sink.Events()
	require.Len(t, events, 1)
	assert.Equal(t, streamStatusEvent, events[0].Event)
	assert.JSONEq(
		t,
		`{"type":"chat_status","status":"connecting"}`,
		string(events[0].Data),
	)
}

func TestRunToolDemoStreamEmitsLifecycleStatuses(t *testing.T) {
	t.Parallel()

	var stream bytes.Buffer

	chat := &models.Chat{
		Base:    models.Base{ID: uuid.New()},
		ModelID: "showcase-model",
	}
	demo := fixedresponses.Response{
		Kind: fixedresponses.KindTools,
		Steps: []fixedresponses.Step{
			{Kind: fixedresponses.StepThinking, Text: "Check the data."},
			{
				Kind: fixedresponses.StepTool,
				Tool: &fixedresponses.ToolStep{
					ToolUseID:   "call-1",
					Name:        "analytics__summary",
					ArgsJSON:    `{}`,
					ResultText:  `{"count":1}`,
					ResultDelay: time.Nanosecond,
				},
			},
			{Kind: fixedresponses.StepText, Text: "Here is the summary."},
		},
	}

	require.NoError(t, (&Service{}).runToolDemoStream(
		t.Context(),
		&stream,
		chat,
		demo,
	))

	assertTurnStatusesInOrder(t, stream.String(), []string{
		turnStatusConnecting,
		turnStatusWaitingForFirstToken,
		turnStatusStreaming,
		turnStatusRunningTool,
		turnStatusWaitingForFirstToken,
		turnStatusStreaming,
	})
}

// TestRunDemoStreamEmitsTokens covers the canned-text demo path (runDemoStream
// -> streamTokens): it tokenizes the text, emits the connecting/waiting/
// streaming lifecycle, streams every token, and closes cleanly. This is the
// KindText branch the tool-heavy showcase catalog never exercises.
func TestRunDemoStreamEmitsTokens(t *testing.T) {
	t.Parallel()

	var stream bytes.Buffer

	chat := &models.Chat{
		Base:    models.Base{ID: uuid.New()},
		ModelID: "showcase-model",
	}

	require.NoError(t, (&Service{}).runDemoStream(
		t.Context(),
		&stream,
		chat,
		"one two three",
	))

	assertTurnStatusesInOrder(t, stream.String(), []string{
		turnStatusConnecting,
		turnStatusWaitingForFirstToken,
		turnStatusStreaming,
	})
	assert.NotEmpty(t, stream.String())
}

// failAfterSink is an io.Writer that succeeds for the first okWrites emits,
// then fails every write after. okWrites == 0 fails the first status publish; a
// larger value lets the preamble through so the failure lands inside the token
// or block streaming loop, driving the deeper publish-error branches a healthy
// sink never reaches.
type failAfterSink struct {
	okWrites int
}

func (s *failAfterSink) Write(p []byte) (int, error) {
	if s.okWrites <= 0 {
		return 0, io.ErrClosedPipe
	}

	s.okWrites--

	return len(p), nil
}

// TestDemoStreamsPropagatePublishError proves both demo entry points surface a
// sink write failure instead of swallowing it, whether it fails on the opening
// status publish or partway through the stream (the mid-stream token/block
// emit branches).
func TestDemoStreamsPropagatePublishError(t *testing.T) {
	t.Parallel()

	chat := &models.Chat{
		Base:    models.Base{ID: uuid.New()},
		ModelID: "showcase-model",
	}

	demo := fixedresponses.Response{
		Kind: fixedresponses.KindTools,
		Steps: []fixedresponses.Step{
			{Kind: fixedresponses.StepThinking, Text: "thinking"},
			{
				Kind: fixedresponses.StepTool,
				Tool: &fixedresponses.ToolStep{
					ToolUseID:  "call-1",
					Name:       "lookup",
					ArgsJSON:   `{}`,
					ResultText: `{"ok":true}`,
				},
			},
			{Kind: fixedresponses.StepText, Text: "the answer"},
		},
	}

	// okWrites walks the failure from the opening publish (0) to partway
	// through the stream, so both the early-return and mid-loop error branches
	// of each demo path run.
	for _, okWrites := range []int{0, 3, 6} {
		require.Error(t, (&Service{}).runDemoStream(
			t.Context(), &failAfterSink{okWrites: okWrites},
			chat, "one two three four",
		))
		require.Error(t, (&Service{}).runToolDemoStream(
			t.Context(), &failAfterSink{okWrites: okWrites}, chat, demo,
		))
	}
}

// TestWatchDemoPipeCancellation proves the cancellation watcher closes the demo
// pipe with the context error when the turn is cancelled, so the SSE reader
// unblocks with a failure instead of hanging.
func TestWatchDemoPipeCancellation(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())

	stop := watchDemoPipeCancellation(ctx, pw, uuid.New())
	defer stop()

	cancel()

	_, err := pr.Read(make([]byte, 1))
	require.Error(t, err)
}

func TestRunStreamCandidatesEmitsRetryBeforeFallback(t *testing.T) {
	t.Parallel()

	primary := elelemtest.NewScriptedDriver(
		elelemtest.Turn{Err: upstreams.ErrFirstTokenTimeout},
	).WithModels("primary")
	fallback := elelemtest.NewScriptedDriver(
		elelemtest.Text("fallback answer"),
	).WithModels("fallback")
	configured := []config.Upstream{
		{
			Name: "primary",
			Models: []config.Model{{
				ID:             "primary",
				FallbackModels: []string{"fallback"},
			}},
		},
		{Name: "fallback"},
	}
	drivers := map[string]*elelemtest.ScriptedDriver{
		"primary":  primary,
		"fallback": fallback,
	}
	registry := upstreams.Discover(
		t.Context(),
		configured,
		func(upstream config.Upstream) *elelem.Client {
			return elelem.New(drivers[upstream.Name])
		},
		time.Second,
		upstreams.NewHealthTracker([]string{"primary", "fallback"}),
	)
	require.NoError(t, registry.SetFallbacks(configured))

	sink := essessey.NewInMemorySink()
	pub := essessey.NewPublisher(t.Context(), sink)
	service := &Service{
		models: registry,
		mcp:    mcp.NewManager(nil),
	}
	chat := &models.Chat{
		Base:    models.Base{ID: uuid.New()},
		ModelID: "primary",
	}

	result, err := service.runStreamCandidates(
		context.Background(),
		chat,
		nil,
		elelem.NewPrompt().UserText("hello"),
		pub,
		nil,
	)
	require.NoError(t, err)
	assert.Equal(t, "fallback", result.ModelID)
	assert.Equal(t, "fallback answer", result.FinalText)
	assert.Equal(t, []string{
		turnStatusConnecting,
		turnStatusWaitingForFirstToken,
		turnStatusRetrying,
		turnStatusConnecting,
		turnStatusWaitingForFirstToken,
		turnStatusStreaming,
	}, turnStatuses(t, sink.Events()))
}

func turnStatuses(t *testing.T, events []essessey.Event) []string {
	t.Helper()

	statuses := make([]string, 0)

	for _, event := range events {
		if event.Event != streamStatusEvent {
			continue
		}

		var data turnStatusData
		require.NoError(t, json.Unmarshal(event.Data, &data))
		statuses = append(statuses, data.Status)
	}

	return statuses
}

func assertTurnStatusesInOrder(
	t *testing.T,
	stream string,
	statuses []string,
) {
	t.Helper()

	position := 0

	for _, status := range statuses {
		frame := "event: " + streamStatusEvent + "\ndata: {\"type\":\"" +
			streamStatusEvent + "\",\"status\":\"" + status + "\"}"
		next := strings.Index(stream[position:], frame)
		require.NotEqualf(t, -1, next, "missing status %q", status)
		position += next + len(frame)
	}
}
