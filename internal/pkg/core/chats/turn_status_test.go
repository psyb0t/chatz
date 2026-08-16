package chats

import (
	"bytes"
	"context"
	"encoding/json"
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
