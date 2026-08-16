package chats

import (
	"context"
	"errors"

	"github.com/psyb0t/chatz/internal/pkg/upstreams"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/essessey"
)

const (
	streamErrorEvent = "error"

	streamErrorTypeTimeout          = "upstream_timeout"
	streamErrorTypeRateLimited      = "rate_limited"
	streamErrorTypeModelUnavailable = "model_unavailable"
	streamErrorTypeContextLimit     = "context_limit"
	streamErrorTypeRequestFailed    = "request_failed"

	streamErrorMessageTimeout = "The model did not respond in time. Try again."
	//nolint:lll // user-facing terminal message must remain one line
	streamErrorMessageRateLimited = "The model provider is busy. Try again shortly."
	//nolint:lll // user-facing terminal message must remain one line
	streamErrorMessageModelUnavailable = "The selected model is unavailable. Contact an administrator."
	//nolint:lll // user-facing terminal message must remain one line
	streamErrorMessageContextLimit  = "This conversation is too large for the selected model."
	streamErrorMessageRequestFailed = "The model request failed. Try again."
)

type streamErrorData struct {
	Type  string            `json:"type"`
	Error streamErrorDetail `json:"error"`
}

type streamErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// streamFailure returns an intentionally small, user-safe terminal event. Raw
// provider errors can carry response bodies and endpoint details, so they must
// stay in server-side diagnostics rather than cross the SSE boundary.
func streamFailure(err error) (streamErrorData, bool) {
	if err == nil || errors.Is(err, context.Canceled) {
		return streamErrorData{}, false
	}

	return classifyStreamFailure(err), true
}

func classifyStreamFailure(err error) streamErrorData {
	if matchesAny(
		err,
		context.DeadlineExceeded,
		upstreams.ErrFirstTokenTimeout,
		upstreams.ErrTurnTimeout,
	) {
		return newStreamFailure(
			streamErrorTypeTimeout,
			streamErrorMessageTimeout,
		)
	}

	if matchesAny(err, commerr.ErrRateLimited) {
		return newStreamFailure(
			streamErrorTypeRateLimited,
			streamErrorMessageRateLimited,
		)
	}

	if matchesAny(
		err,
		commerr.ErrNotAuthenticated,
		commerr.ErrPermissionDenied,
		commerr.ErrUnavailable,
	) {
		return newStreamFailure(
			streamErrorTypeModelUnavailable,
			streamErrorMessageModelUnavailable,
		)
	}

	if matchesAny(
		err,
		elelem.ErrContextExceeded,
		elelem.ErrMaxOutputExceedsContext,
	) {
		return newStreamFailure(
			streamErrorTypeContextLimit,
			streamErrorMessageContextLimit,
		)
	}

	return newStreamFailure(
		streamErrorTypeRequestFailed,
		streamErrorMessageRequestFailed,
	)
}

// isFallbackEligible limits model fallback to failures that prove no provider
// response was available. The caller separately requires chatcore.StreamStarted
// to be false, which means no client-visible content or tool-capable assistant
// message has been emitted.
func isFallbackEligible(err error) bool {
	return matchesAny(
		err,
		upstreams.ErrFirstTokenTimeout,
		upstreams.ErrTurnTimeout,
		commerr.ErrRateLimited,
		commerr.ErrUnavailable,
	)
}

func matchesAny(err error, candidates ...error) bool {
	for _, candidate := range candidates {
		if errors.Is(err, candidate) {
			return true
		}
	}

	return false
}

func newStreamFailure(kind, message string) streamErrorData {
	return streamErrorData{
		Type: streamErrorEvent,
		Error: streamErrorDetail{
			Type:    kind,
			Message: message,
		},
	}
}

func publishStreamFailure(
	pub *essessey.Publisher,
	failure streamErrorData,
) error {
	if err := pub.Publish(streamErrorEvent, failure); err != nil {
		return ctxerrors.Wrap(err, "publish terminal stream failure")
	}

	return nil
}
