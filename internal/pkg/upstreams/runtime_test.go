package upstreams

import (
	"context"
	"testing"
	"time"

	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const upstreamRuntimeTestName = "runtime-test"

type runtimeTestDriver struct {
	stream func(
		context.Context,
		elelem.DriverRequest,
		func(elelem.Delta) error,
	) (elelem.Usage, error)
}

func (d runtimeTestDriver) Stream(
	ctx context.Context,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return d.stream(ctx, request, onDelta)
}

func (d runtimeTestDriver) Complete(
	ctx context.Context,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return d.stream(ctx, request, onDelta)
}

func (runtimeTestDriver) ListModels(context.Context) ([]string, error) {
	return nil, nil
}

func (runtimeTestDriver) Capabilities(elelem.Model) elelem.Capabilities {
	return elelem.Capabilities{}
}

func (runtimeTestDriver) TokenCounter() elelem.TokenCounter {
	return elelem.DefaultTokenCounter()
}

func runtimeLimits() RuntimeLimits {
	return RuntimeLimits{
		MaxConcurrent:     1,
		FirstTokenTimeout: 25 * time.Millisecond,
		TurnTimeout:       100 * time.Millisecond,
	}
}

func wrappedRuntimeDriver(
	t *testing.T,
	driver elelem.Driver,
	limits RuntimeLimits,
) (*runtimeDriver, *HealthTracker) {
	t.Helper()

	health := NewHealthTracker([]string{upstreamRuntimeTestName})
	wrapped, err := WrapDriver(
		driver,
		upstreamRuntimeTestName,
		limits,
		health,
	)
	require.NoError(t, err)

	runtime, ok := wrapped.(*runtimeDriver)
	require.True(t, ok)

	return runtime, health
}

func TestRuntimeLimits_Validate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		limits  RuntimeLimits
		wantErr error
	}{
		{
			name: "valid",
			limits: RuntimeLimits{
				MaxConcurrent:     1,
				FirstTokenTimeout: time.Second,
				TurnTimeout:       time.Second,
			},
		},
		{
			name: "zero concurrency",
			limits: RuntimeLimits{
				FirstTokenTimeout: time.Second,
				TurnTimeout:       time.Second,
			},
			wantErr: ErrInvalidLimits,
		},
		{
			name: "first token exceeds turn budget",
			limits: RuntimeLimits{
				MaxConcurrent:     1,
				FirstTokenTimeout: 2 * time.Second,
				TurnTimeout:       time.Second,
			},
			wantErr: ErrInvalidLimits,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.limits.Validate()
			if tc.wantErr == nil {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestRuntimeDriver_FirstTokenTimeout(t *testing.T) {
	t.Parallel()

	driver, health := wrappedRuntimeDriver(t, runtimeTestDriver{
		stream: func(
			ctx context.Context,
			_ elelem.DriverRequest,
			_ func(elelem.Delta) error,
		) (elelem.Usage, error) {
			<-ctx.Done()

			return elelem.Usage{}, ctx.Err()
		},
	}, runtimeLimits())

	_, err := driver.Stream(t.Context(), elelem.DriverRequest{}, nil)
	require.ErrorIs(t, err, ErrFirstTokenTimeout)

	status, ok := health.Snapshot(upstreamRuntimeTestName)
	require.True(t, ok)
	assert.Equal(t, HealthStateDegraded, status.State)
	assert.Equal(t, "first_token_timeout", status.LastFailureClass)
	assert.Equal(t, 1, status.ConsecutiveFailure)
}

func TestRuntimeDriver_TurnTimeoutAfterFirstToken(t *testing.T) {
	t.Parallel()

	driver, health := wrappedRuntimeDriver(t, runtimeTestDriver{
		stream: func(
			ctx context.Context,
			_ elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			if err := onDelta(elelem.Delta{}); err != nil {
				return elelem.Usage{}, err
			}

			<-ctx.Done()

			return elelem.Usage{}, ctx.Err()
		},
	}, runtimeLimits())

	_, err := driver.Stream(t.Context(), elelem.DriverRequest{}, nil)
	require.ErrorIs(t, err, ErrTurnTimeout)

	status, ok := health.Snapshot(upstreamRuntimeTestName)
	require.True(t, ok)
	assert.Equal(t, HealthStateDegraded, status.State)
	assert.Equal(t, "turn_timeout", status.LastFailureClass)
}

func TestRuntimeDriver_StopsFirstTokenTimerAfterDelta(t *testing.T) {
	t.Parallel()

	limits := RuntimeLimits{
		MaxConcurrent:     1,
		FirstTokenTimeout: 25 * time.Millisecond,
		TurnTimeout:       250 * time.Millisecond,
	}
	driver, health := wrappedRuntimeDriver(t, runtimeTestDriver{
		stream: func(
			ctx context.Context,
			_ elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			if err := onDelta(elelem.Delta{}); err != nil {
				return elelem.Usage{}, err
			}

			select {
			case <-time.After(75 * time.Millisecond):
				return elelem.Usage{}, nil
			case <-ctx.Done():
				return elelem.Usage{}, ctx.Err()
			}
		},
	}, limits)

	_, err := driver.Stream(t.Context(), elelem.DriverRequest{}, nil)
	require.NoError(t, err)

	status, ok := health.Snapshot(upstreamRuntimeTestName)
	require.True(t, ok)
	assert.Equal(t, HealthStateHealthy, status.State)
	assert.Empty(t, status.LastFailureClass)
}

func TestRuntimeDriver_QueueRespectsCallerCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})
	firstFinished := make(chan error, 1)

	driver, health := wrappedRuntimeDriver(t, runtimeTestDriver{
		stream: func(
			ctx context.Context,
			_ elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			if err := onDelta(elelem.Delta{}); err != nil {
				return elelem.Usage{}, err
			}

			select {
			case started <- struct{}{}:
			case <-ctx.Done():
				return elelem.Usage{}, ctx.Err()
			}

			select {
			case <-release:
				return elelem.Usage{}, nil
			case <-ctx.Done():
				return elelem.Usage{}, ctx.Err()
			}
		},
	}, runtimeLimits())

	go func() {
		_, err := driver.Stream(t.Context(), elelem.DriverRequest{}, nil)
		firstFinished <- err
	}()

	<-started

	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	t.Cleanup(cancel)

	_, err := driver.Stream(ctx, elelem.DriverRequest{}, nil)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	status, ok := health.Snapshot(upstreamRuntimeTestName)
	require.True(t, ok)
	assert.Equal(t, "queue", status.LastOperation)
	assert.Equal(t, "deadline_exceeded", status.LastFailureClass)

	close(release)
	require.NoError(t, <-firstFinished)
}

func TestRuntimeDriver_HealthyCompletionRestoresHealth(t *testing.T) {
	t.Parallel()

	driver, health := wrappedRuntimeDriver(t, runtimeTestDriver{
		stream: func(
			_ context.Context,
			_ elelem.DriverRequest,
			onDelta func(elelem.Delta) error,
		) (elelem.Usage, error) {
			if err := onDelta(elelem.Delta{}); err != nil {
				return elelem.Usage{}, err
			}

			return elelem.Usage{}, nil
		},
	}, runtimeLimits())

	health.RecordFailure(
		upstreamRuntimeTestName,
		"stream",
		0,
		ErrTurnTimeout,
	)

	_, err := driver.Stream(t.Context(), elelem.DriverRequest{}, nil)
	require.NoError(t, err)

	status, ok := health.Snapshot(upstreamRuntimeTestName)
	require.True(t, ok)
	assert.Equal(t, HealthStateHealthy, status.State)
	assert.Zero(t, status.ConsecutiveFailure)
	assert.NotZero(t, status.LastSuccessAt)
}
