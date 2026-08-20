package chats

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/upstreams"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/essessey"
	essesseysse "github.com/psyb0t/essessey/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const expectedGenericStreamFailure = `{
	"type": "error",
	"error": {
		"type": "request_failed",
		"message": "The model request failed. Try again."
	}
}`

func TestStreamFailure(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		err     error
		want    streamErrorData
		publish bool
	}{
		{
			name:    "nil does not publish",
			publish: false,
		},
		{
			name:    "cancelled turn does not publish",
			err:     context.Canceled,
			publish: false,
		},
		{
			name: "first-token timeout is safe",
			err: ctxerrors.Wrap(
				upstreams.ErrFirstTokenTimeout,
				"provider response",
			),
			want: newStreamFailure(
				streamErrorTypeTimeout,
				streamErrorMessageTimeout,
			),
			publish: true,
		},
		{
			name: "rate limit is safe",
			err: ctxerrors.Wrap(
				commerr.ErrRateLimited,
				"provider said too many requests",
			),
			want: newStreamFailure(
				streamErrorTypeRateLimited,
				streamErrorMessageRateLimited,
			),
			publish: true,
		},
		{
			name: "unknown provider failure is generic",
			err:  ctxerrors.New("provider response contains secret"),
			want: newStreamFailure(
				streamErrorTypeRequestFailed,
				streamErrorMessageRequestFailed,
			),
			publish: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, publishable := streamFailure(tc.err)

			assert.Equal(t, tc.publish, publishable)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestHandleStreamTurnFailure covers the terminal-failure dispatcher: a
// publishable error emits exactly one redacted error event, while a cancelled
// turn is a quiet stop that publishes nothing.
func TestHandleStreamTurnFailure(t *testing.T) {
	t.Parallel()

	t.Run("publishable error emits a terminal event", func(t *testing.T) {
		t.Parallel()

		sink := essessey.NewInMemorySink()
		pub := essessey.NewPublisher(t.Context(), sink)

		handleStreamTurnFailure(
			t.Context(),
			uuid.New(),
			pub,
			ctxerrors.Wrap(
				upstreams.ErrFirstTokenTimeout,
				"provider response",
			),
		)

		events := sink.Events()
		require.Len(t, events, 1)
		assert.Equal(t, streamErrorEvent, events[0].Event)
	})

	t.Run("cancelled turn publishes nothing", func(t *testing.T) {
		t.Parallel()

		sink := essessey.NewInMemorySink()
		pub := essessey.NewPublisher(t.Context(), sink)

		handleStreamTurnFailure(t.Context(), uuid.New(), pub, context.Canceled)

		assert.Empty(t, sink.Events())
	})
}

// TestPublishStreamFailurePropagatesSinkError proves a terminal-failure publish
// surfaces a sink write error rather than swallowing it.
func TestPublishStreamFailurePropagatesSinkError(t *testing.T) {
	t.Parallel()

	pub := essessey.NewPublisher(
		t.Context(), essesseysse.NewWriterSink(&failAfterSink{}),
	)

	err := publishStreamFailure(pub, newStreamFailure(
		streamErrorTypeRequestFailed, streamErrorMessageRequestFailed,
	))
	require.Error(t, err)
}

func TestPublishStreamFailureRedactsProviderError(t *testing.T) {
	t.Parallel()

	sink := essessey.NewInMemorySink()
	pub := essessey.NewPublisher(t.Context(), sink)
	failure, publishable := streamFailure(
		ctxerrors.New("provider response contains secret"),
	)
	require.True(t, publishable)

	err := publishStreamFailure(pub, failure)
	require.NoError(t, err)

	events := sink.Events()
	require.Len(t, events, 1)
	assert.Equal(t, streamErrorEvent, events[0].Event)
	assert.NotContains(t, string(events[0].Data), "secret")
	assert.JSONEq(t, expectedGenericStreamFailure, string(events[0].Data))
}
