package rebound

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errOperation = errors.New("operation failed")

type retryHarness struct {
	now    time.Time
	delays []time.Duration
}

func newRetryHarness() *retryHarness {
	return &retryHarness{now: time.Unix(0, 0)}
}

func (h *retryHarness) options() []Option {
	return []Option{
		withClock(func() time.Time {
			return h.now
		}),
		withWait(func(_ context.Context, delay time.Duration) error {
			h.delays = append(h.delays, delay)
			h.now = h.now.Add(delay)

			return nil
		}),
		withRandom(func() float64 {
			return 1
		}),
	}
}

func TestDo_Defaults(t *testing.T) {
	t.Parallel()

	cfg, err := newConfig(nil)
	require.NoError(t, err)
	assert.Equal(t, defaultMaxAttempts, cfg.maxAttempts)
	assert.Equal(t, defaultInitialDelay, cfg.initialDelay)
	assert.Equal(t, defaultDelayMultiplier, cfg.delayMultiplier)
	assert.Equal(t, defaultMaximumDelay, cfg.maximumDelay)
	assert.Equal(t, defaultMaximumAge, cfg.maximumElapsed)
	assert.False(t, cfg.jitter)
}

func TestDo_InvalidInput(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		newContext  func() context.Context
		operation   func(context.Context) error
		options     []Option
		wantErr     error
		wantMessage string
	}{
		{
			name:        "nil context",
			operation:   func(context.Context) error { return nil },
			wantErr:     commerr.ErrInvalidArgument,
			wantMessage: "rebound context is nil",
		},
		{
			name:        "nil operation",
			newContext:  context.Background,
			wantErr:     commerr.ErrInvalidArgument,
			wantMessage: "rebound operation is nil",
		},
		{
			name:        "zero attempts",
			newContext:  context.Background,
			operation:   func(context.Context) error { return nil },
			options:     []Option{WithMaxAttempts(0)},
			wantErr:     commerr.ErrInvalidArgument,
			wantMessage: "rebound WithMaxAttempts(0): must be positive",
		},
		{
			name:        "negative initial delay",
			newContext:  context.Background,
			operation:   func(context.Context) error { return nil },
			options:     []Option{WithInitialDelay(-time.Nanosecond)},
			wantErr:     commerr.ErrInvalidArgument,
			wantMessage: "rebound WithInitialDelay(-1ns): must be positive",
		},
		{
			name:       "zero delay multiplier",
			newContext: context.Background,
			operation:  func(context.Context) error { return nil },
			options:    []Option{WithDelayMultiplier(0)},
			wantErr:    commerr.ErrInvalidArgument,
			wantMessage: "rebound WithDelayMultiplier(0): " +
				"must be positive and finite",
		},
		{
			name:       "negative delay multiplier",
			newContext: context.Background,
			operation:  func(context.Context) error { return nil },
			options:    []Option{WithDelayMultiplier(-0.5)},
			wantErr:    commerr.ErrInvalidArgument,
			wantMessage: "rebound WithDelayMultiplier(-0.5): " +
				"must be positive and finite",
		},
		{
			name:       "NaN delay multiplier",
			newContext: context.Background,
			operation:  func(context.Context) error { return nil },
			options:    []Option{WithDelayMultiplier(math.NaN())},
			wantErr:    commerr.ErrInvalidArgument,
			wantMessage: "rebound WithDelayMultiplier(NaN): " +
				"must be positive and finite",
		},
		{
			name:       "infinite delay multiplier",
			newContext: context.Background,
			operation:  func(context.Context) error { return nil },
			options:    []Option{WithDelayMultiplier(math.Inf(1))},
			wantErr:    commerr.ErrInvalidArgument,
			wantMessage: "rebound WithDelayMultiplier(+Inf): " +
				"must be positive and finite",
		},
		{
			name:       "inverted delays",
			newContext: context.Background,
			operation:  func(context.Context) error { return nil },
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(time.Millisecond),
			},
			wantErr: commerr.ErrInvalidArgument,
			wantMessage: "rebound WithMaxDelay(1ms): must be greater than or " +
				"equal to WithInitialDelay(1s)",
		},
		{
			name:        "zero maximum elapsed",
			newContext:  context.Background,
			operation:   func(context.Context) error { return nil },
			options:     []Option{WithMaxElapsed(0)},
			wantErr:     commerr.ErrInvalidArgument,
			wantMessage: "rebound WithMaxElapsed(0s): must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var ctx context.Context
			if tc.newContext != nil {
				ctx = tc.newContext()
			}

			var operationCalls atomic.Int32

			operation := tc.operation
			if operation != nil {
				originalOperation := operation
				operation = func(ctx context.Context) error {
					operationCalls.Add(1)

					return originalOperation(ctx)
				}
			}

			err := Do(ctx, operation, tc.options...)
			require.ErrorIs(t, err, tc.wantErr)
			assert.ErrorContains(t, err, tc.wantMessage)
			assert.Zero(t, operationCalls.Load())
		})
	}
}

func TestDo_SucceedsWithoutRetry(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32

	err := Do(t.Context(), func(context.Context) error {
		attempts.Add(1)

		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, int32(1), attempts.Load())
}

func TestDo_RetriesThenSucceeds(t *testing.T) {
	t.Parallel()

	harness := newRetryHarness()

	var attempts atomic.Int32

	err := Do(t.Context(), func(context.Context) error {
		if attempts.Add(1) < 3 {
			return errOperation
		}

		return nil
	}, harness.options()...)

	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts.Load())
	assert.Equal(t, []time.Duration{
		defaultInitialDelay,
		defaultInitialDelay * 2,
	}, harness.delays)
}

func TestDo_NonRetryableWrappedSentinel(t *testing.T) {
	t.Parallel()

	wrapped := ctxerrors.Wrap(
		commerr.ErrInvalidArgument,
		"provider rejected input",
	)

	var attempts atomic.Int32

	err := Do(t.Context(), func(context.Context) error {
		attempts.Add(1)

		return errors.Join(errOperation, wrapped)
	}, WithNonRetryables(commerr.ErrInvalidArgument))

	require.ErrorIs(t, err, commerr.ErrInvalidArgument)
	assert.Equal(t, int32(1), attempts.Load())
}

func TestDo_ExhaustionMatchesSharedAndOperationErrors(t *testing.T) {
	t.Parallel()

	harness := newRetryHarness()

	var attempts atomic.Int32

	err := Do(t.Context(), func(context.Context) error {
		attempts.Add(1)

		return errOperation
	}, append(harness.options(), WithMaxAttempts(2))...)

	require.ErrorIs(t, err, commerr.ErrExhausted)
	require.ErrorIs(t, err, errOperation)
	assert.Equal(t, int32(2), attempts.Load())

	var exhausted *ExhaustedError
	require.ErrorAs(t, err, &exhausted)
	assert.Equal(t, 2, exhausted.Attempts)
	assert.ErrorIs(t, exhausted.Cause, errOperation)
}

func TestDo_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("before first attempt", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		var attempts atomic.Int32

		err := Do(ctx, func(context.Context) error {
			attempts.Add(1)

			return nil
		})

		require.ErrorIs(t, err, context.Canceled)
		assert.Zero(t, attempts.Load())
	})

	t.Run("during operation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		var attempts atomic.Int32

		err := Do(ctx, func(ctx context.Context) error {
			attempts.Add(1)
			cancel()

			return ctx.Err()
		})

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(1), attempts.Load())
	})

	t.Run("during backoff", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)

		var attempts atomic.Int32

		err := Do(ctx, func(context.Context) error {
			attempts.Add(1)

			return errOperation
		}, withWait(func(context.Context, time.Duration) error {
			cancel()

			return ctx.Err()
		}))

		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, int32(1), attempts.Load())
	})
}

func TestDo_OptionsAndCallbacksArePerInvocation(t *testing.T) {
	t.Parallel()

	harness := newRetryHarness()
	preAttempts := make([]Attempt, 0, 2)
	postAttempts := make([]Attempt, 0, 2)

	err := Do(t.Context(), func(context.Context) error {
		return errOperation
	}, append(harness.options(),
		WithMaxAttempts(2),
		WithRetryAfter(func(error) (time.Duration, bool) {
			return time.Second, true
		}),
		WithPreAttemptHandler(func(attempt Attempt) {
			preAttempts = append(preAttempts, attempt)
		}),
		WithPostAttemptHandler(func(attempt Attempt) {
			postAttempts = append(postAttempts, attempt)
		}),
	)...)

	require.ErrorIs(t, err, commerr.ErrExhausted)
	require.Len(t, preAttempts, 2)
	require.Len(t, postAttempts, 2)
	assert.Equal(t, []Attempt{{Number: 1}, {Number: 2}}, preAttempts)
	assert.Equal(t, time.Second, postAttempts[0].RetryDelay)
	assert.Zero(t, postAttempts[1].RetryDelay)
	assert.Equal(t, []time.Duration{time.Second}, harness.delays)

	var secondAttempts atomic.Int32

	err = Do(t.Context(), func(context.Context) error {
		if secondAttempts.Add(1) == 1 {
			return errOperation
		}

		return nil
	}, WithInitialDelay(time.Nanosecond))

	require.NoError(t, err)
	assert.Equal(t, int32(2), secondAttempts.Load())
}

func TestConfig_Delay(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		attempt int
		random  float64
		jitter  bool
		want    time.Duration
		options []Option
	}{
		{
			name:    "exponential backoff caps at maximum",
			attempt: 4,
			want:    4 * time.Second,
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(4 * time.Second),
			},
		},
		{
			name:    "fractional multiplier decreases the delay",
			attempt: 2,
			want:    500 * time.Millisecond,
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(4 * time.Second),
				WithDelayMultiplier(0.5),
			},
		},
		{
			name:    "decimal multiplier is applied repeatedly",
			attempt: 3,
			want:    1960 * time.Millisecond,
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(4 * time.Second),
				WithDelayMultiplier(1.4),
			},
		},
		{
			name:    "large decimal multiplier remains capped",
			attempt: 3,
			want:    4 * time.Second,
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(4 * time.Second),
				WithDelayMultiplier(3.6),
			},
		},
		{
			name:    "retry after is bounded",
			attempt: 1,
			want:    4 * time.Second,
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(4 * time.Second),
				WithRetryAfter(func(error) (time.Duration, bool) {
					return 10 * time.Second, true
				}),
			},
		},
		{
			name:    "equal jitter lower bound",
			attempt: 1,
			jitter:  true,
			want:    500 * time.Millisecond,
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(2 * time.Second),
			},
		},
		{
			name:    "equal jitter upper bound clamps random input",
			attempt: 1,
			random:  2,
			jitter:  true,
			want:    time.Second,
			options: []Option{
				WithInitialDelay(time.Second),
				WithMaxDelay(2 * time.Second),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := newConfig(append(tc.options,
				WithJitter(tc.jitter),
				withRandom(func() float64 {
					return tc.random
				}),
			))
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.delay(tc.attempt, errOperation))
		})
	}
}

func TestDo_MaxElapsedStopsBeforeAnotherAttempt(t *testing.T) {
	t.Parallel()

	harness := newRetryHarness()

	var attempts atomic.Int32

	err := Do(t.Context(), func(context.Context) error {
		attempts.Add(1)

		return errOperation
	}, append(harness.options(),
		WithInitialDelay(time.Second),
		WithMaxElapsed(500*time.Millisecond),
	)...)

	require.ErrorIs(t, err, commerr.ErrExhausted)
	assert.Equal(t, int32(1), attempts.Load())
	assert.Empty(t, harness.delays)
}

func TestDo_Concurrent(t *testing.T) {
	t.Parallel()

	const workers = 32

	var calls atomic.Int32

	errs := make(chan error, workers)

	var group sync.WaitGroup
	group.Add(workers)

	for range workers {
		go func() {
			defer group.Done()

			attempt := 0
			errs <- Do(t.Context(), func(context.Context) error {
				attempt++

				calls.Add(1)

				if attempt == 1 {
					return errOperation
				}

				return nil
			}, WithInitialDelay(time.Nanosecond))
		}()
	}

	group.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	assert.Equal(t, int32(workers*2), calls.Load())
}
