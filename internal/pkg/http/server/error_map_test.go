package server

import (
	"errors"
	"testing"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommerrMigration_WrappedAndJoinedFailuresRemainClassifiable(
	t *testing.T,
) {
	t.Parallel()

	additionalCause := commerr.ErrUnavailable
	testCases := []struct {
		name        string
		sentinel    error
		notSentinel error
	}{
		{
			name:        "rate limited",
			sentinel:    commerr.ErrRateLimited,
			notSentinel: commerr.ErrNotAuthenticated,
		},
		{
			name:        "not authenticated",
			sentinel:    commerr.ErrNotAuthenticated,
			notSentinel: commerr.ErrNotFound,
		},
		{
			name:        "not found",
			sentinel:    commerr.ErrNotFound,
			notSentinel: commerr.ErrInvalidArgument,
		},
		{
			name:        "invalid argument",
			sentinel:    commerr.ErrInvalidArgument,
			notSentinel: commerr.ErrExhausted,
		},
		{
			name:        "exhausted",
			sentinel:    commerr.ErrExhausted,
			notSentinel: commerr.ErrRateLimited,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := errors.Join(
				additionalCause,
				ctxerrors.Wrap(tc.sentinel, "classify shared failure"),
			)

			require.ErrorIs(t, err, tc.sentinel)
			assert.NotErrorIs(t, err, tc.notSentinel)
			assert.ErrorIs(t, err, additionalCause)
		})
	}
}
