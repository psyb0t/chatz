package upstreams

import (
	"context"
	"sync"
	"time"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/elelem"
)

// RuntimeLimits bound one upstream's live provider traffic. The queue is
// caller-context-bound; first-token and turn timers start only after a slot is
// acquired, so queue time is not mislabeled as provider lag.
type RuntimeLimits struct {
	MaxConcurrent     int
	FirstTokenTimeout time.Duration
	TurnTimeout       time.Duration
}

// Validate ensures limits fail closed before any provider request is made.
func (l RuntimeLimits) Validate() error {
	if l.MaxConcurrent <= 0 ||
		l.FirstTokenTimeout <= 0 ||
		l.TurnTimeout <= 0 ||
		l.FirstTokenTimeout > l.TurnTimeout {
		return ctxerrors.Wrap(
			ErrInvalidLimits,
			"validate upstream runtime limits",
		)
	}

	return nil
}

// WrapDriver adds queue limits, time-to-first-token and total-turn deadlines
// around a concurrency-safe Elelem driver. It never retries a request: Elelem
// owns provider-aware retries, and MCP tool execution is outside this wrapper.
func WrapDriver(
	driver elelem.Driver,
	upstream string,
	limits RuntimeLimits,
	health *HealthTracker,
) (elelem.Driver, error) {
	if driver == nil {
		return nil, ctxerrors.Wrap(ErrNilDriver, "wrap upstream driver")
	}

	if upstream == "" {
		return nil, ctxerrors.Wrap(
			ErrInvalidUpstreamName,
			"wrap upstream driver",
		)
	}

	if health == nil {
		return nil, ctxerrors.Wrap(ErrNilHealthTracker, "wrap upstream driver")
	}

	if err := limits.Validate(); err != nil {
		return nil, ctxerrors.Wrap(err, "wrap upstream driver")
	}

	return &runtimeDriver{
		driver:   driver,
		upstream: upstream,
		limits:   limits,
		health:   health,
		slots:    make(chan struct{}, limits.MaxConcurrent),
	}, nil
}

type runtimeDriver struct {
	driver   elelem.Driver
	upstream string
	limits   RuntimeLimits
	health   *HealthTracker
	slots    chan struct{}
}

type driverCall func(
	context.Context,
	elelem.DriverRequest,
	func(elelem.Delta) error,
) (elelem.Usage, error)

type driverResult struct {
	usage elelem.Usage
	err   error
}

func (d *runtimeDriver) Stream(
	ctx context.Context,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return d.call(ctx, "stream", d.driver.Stream, request, onDelta)
}

func (d *runtimeDriver) Complete(
	ctx context.Context,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return d.call(ctx, "complete", d.driver.Complete, request, onDelta)
}

func (d *runtimeDriver) ListModels(ctx context.Context) ([]string, error) {
	models, err := d.driver.ListModels(ctx)
	if err != nil {
		return nil, ctxerrors.Wrap(err, "list upstream models")
	}

	return models, nil
}

func (d *runtimeDriver) Capabilities(model elelem.Model) elelem.Capabilities {
	return d.driver.Capabilities(model)
}

func (d *runtimeDriver) TokenCounter() elelem.TokenCounter {
	return d.driver.TokenCounter()
}

func (d *runtimeDriver) call(
	ctx context.Context,
	operation string,
	call driverCall,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	queuedAt := time.Now()
	if err := d.acquire(ctx); err != nil {
		d.health.RecordFailure(d.upstream, "queue", time.Since(queuedAt), err)

		return elelem.Usage{}, ctxerrors.Wrap(
			err,
			"wait for upstream request slot",
		)
	}

	turnCtx, cancel := context.WithTimeout(ctx, d.limits.TurnTimeout)
	firstToken := make(chan struct{})
	results := make(chan driverResult, 1)
	startedAt := time.Now()

	go d.run(
		turnCtx,
		call,
		request,
		onDelta,
		firstToken,
		results,
	)

	return d.await(
		ctx,
		turnCtx,
		cancel,
		operation,
		startedAt,
		firstToken,
		results,
	)
}

func (d *runtimeDriver) acquire(ctx context.Context) error {
	select {
	case d.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctxerrors.Wrap(ctx.Err(), "wait for upstream request slot")
	}
}

func (d *runtimeDriver) release() {
	<-d.slots
}

func (d *runtimeDriver) run(
	ctx context.Context,
	call driverCall,
	request elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
	firstToken chan<- struct{},
	results chan<- driverResult,
) {
	var firstTokenOnce sync.Once

	usage, err := call(ctx, request, func(delta elelem.Delta) error {
		if err := ctx.Err(); err != nil {
			return ctxerrors.Wrap(err, "stream upstream delta")
		}

		firstTokenOnce.Do(func() {
			close(firstToken)
		})

		if onDelta == nil {
			return nil
		}

		return onDelta(delta)
	})

	results <- driverResult{usage: usage, err: err}
}

func (d *runtimeDriver) await(
	parent context.Context,
	turnCtx context.Context,
	cancel context.CancelFunc,
	operation string,
	startedAt time.Time,
	firstToken <-chan struct{},
	results <-chan driverResult,
) (elelem.Usage, error) {
	defer cancel()

	firstTokenTimer := time.NewTimer(d.limits.FirstTokenTimeout)
	defer firstTokenTimer.Stop()

	for {
		select {
		case result := <-results:
			return d.finish(operation, startedAt, result)
		case <-firstToken:
			firstToken = nil

			if !firstTokenTimer.Stop() {
				select {
				case <-firstTokenTimer.C:
				default:
				}
			}
		case <-firstTokenTimer.C:
			return elelem.Usage{}, d.timeout(
				cancel,
				operation,
				startedAt,
				results,
				ErrFirstTokenTimeout,
			)
		case <-turnCtx.Done():
			if err := parent.Err(); err != nil {
				return elelem.Usage{}, d.timeout(
					cancel,
					operation,
					startedAt,
					results,
					err,
				)
			}

			return elelem.Usage{}, d.timeout(
				cancel,
				operation,
				startedAt,
				results,
				ErrTurnTimeout,
			)
		}
	}
}

func (d *runtimeDriver) finish(
	operation string,
	startedAt time.Time,
	result driverResult,
) (elelem.Usage, error) {
	d.release()

	latency := time.Since(startedAt)
	if result.err != nil {
		d.health.RecordFailure(
			d.upstream,
			operation,
			latency,
			result.err,
		)

		return result.usage, ctxerrors.Wrap(
			result.err,
			"run upstream request",
		)
	}

	d.health.RecordSuccess(d.upstream, operation, latency)

	return result.usage, nil
}

func (d *runtimeDriver) timeout(
	cancel context.CancelFunc,
	operation string,
	startedAt time.Time,
	results <-chan driverResult,
	err error,
) error {
	cancel()
	d.health.RecordFailure(d.upstream, operation, time.Since(startedAt), err)

	go func() {
		<-results
		d.release()
	}()

	return ctxerrors.Wrap(err, "run upstream request")
}
