// Package rebound retries context-aware operations with bounded exponential
// backoff. It is deliberately stateless: each Do call owns its configuration
// and retry state.
package rebound

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/psyb0t/ctxerrors"
)

// Do invokes operation until it succeeds, becomes non-retryable, the caller's
// context ends, or Rebound exhausts its per-call retry budget.
func Do(
	ctx context.Context,
	operation func(context.Context) error,
	options ...Option,
) error {
	if ctx == nil {
		return invalidArgument("rebound context is nil")
	}

	if operation == nil {
		return invalidArgument("rebound operation is nil")
	}

	cfg, err := newConfig(options)
	if err != nil {
		return ctxerrors.Wrap(err, "configure rebound")
	}

	if err := ctx.Err(); err != nil {
		return ctxerrors.Wrap(err, "start rebound")
	}

	return cfg.run(ctx, operation)
}

func (cfg config) run(
	ctx context.Context,
	operation func(context.Context) error,
) error {
	startedAt := cfg.now()

	for number := 1; number <= cfg.maxAttempts; number++ {
		if err := ctx.Err(); err != nil {
			return ctxerrors.Wrap(err, "run rebound operation")
		}

		cfg.recordPreAttempt(Attempt{Number: number})

		operationErr := operation(ctx)
		decision := cfg.decide(number, startedAt, operationErr)
		cfg.recordPostAttempt(decision.attempt)

		if decision.finished {
			return decision.done
		}

		if err := cfg.wait(ctx, decision.attempt.RetryDelay); err != nil {
			return ctxerrors.Wrap(err, "wait for rebound retry")
		}
	}

	return &ExhaustedError{Attempts: cfg.maxAttempts}
}

type attemptDecision struct {
	attempt  Attempt
	finished bool
	done     error
}

func (cfg config) decide(
	number int,
	startedAt time.Time,
	operationErr error,
) attemptDecision {
	attempt := Attempt{Number: number, Err: operationErr}
	if operationErr == nil {
		return attemptDecision{attempt: attempt, finished: true}
	}

	if isContextError(operationErr) ||
		isNonRetryable(operationErr, cfg.nonRetryable) {
		return attemptDecision{
			attempt:  attempt,
			finished: true,
			done: ctxerrors.Wrap(
				operationErr,
				"rebound operation is not retryable",
			),
		}
	}

	if cfg.exhausted(number, startedAt) {
		return exhaustedDecision(attempt)
	}

	attempt.RetryDelay = cfg.delay(number, operationErr)
	if attempt.RetryDelay > cfg.remaining(startedAt) {
		return exhaustedDecision(attempt)
	}

	return attemptDecision{attempt: attempt}
}

func (cfg config) exhausted(number int, startedAt time.Time) bool {
	return number == cfg.maxAttempts ||
		cfg.now().Sub(startedAt) >= cfg.maximumElapsed
}

func (cfg config) remaining(startedAt time.Time) time.Duration {
	return cfg.maximumElapsed - cfg.now().Sub(startedAt)
}

func exhaustedDecision(attempt Attempt) attemptDecision {
	return attemptDecision{
		attempt:  attempt,
		finished: true,
		done: &ExhaustedError{
			Attempts: attempt.Number,
			Cause:    attempt.Err,
		},
	}
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}

func isNonRetryable(err error, nonRetryable []error) bool {
	for _, candidate := range nonRetryable {
		if candidate != nil && errors.Is(err, candidate) {
			return true
		}
	}

	return false
}

func (cfg config) delay(attempt int, operationErr error) time.Duration {
	delay := exponentialDelay(
		attempt,
		cfg.initialDelay,
		cfg.delayMultiplier,
		cfg.maximumDelay,
	)

	if cfg.retryAfter != nil {
		retryAfter, ok := cfg.retryAfter(operationErr)
		if ok && retryAfter > 0 {
			delay = min(retryAfter, cfg.maximumDelay)
		}
	}

	if !cfg.jitter {
		return delay
	}

	factor := minimumJitterFactor +
		(maximumJitterFactor-minimumJitterFactor)*boundedRandom(cfg.random())

	return time.Duration(float64(delay) * factor)
}

func exponentialDelay(
	attempt int,
	initialDelay time.Duration,
	delayMultiplier float64,
	maximumDelay time.Duration,
) time.Duration {
	delay := float64(initialDelay)
	maximum := float64(maximumDelay)

	for retry := 1; retry < attempt && delay < maximum; retry++ {
		nextDelay := delay * delayMultiplier
		if math.IsInf(nextDelay, 0) || nextDelay >= maximum {
			return maximumDelay
		}

		delay = nextDelay
	}

	return min(time.Duration(math.Round(delay)), maximumDelay)
}

func boundedRandom(random float64) float64 {
	if random < 0 {
		return 0
	}

	if random > 1 {
		return 1
	}

	return random
}

func (cfg config) recordPreAttempt(attempt Attempt) {
	if cfg.preAttempt != nil {
		cfg.preAttempt(attempt)
	}
}

func (cfg config) recordPostAttempt(attempt Attempt) {
	if cfg.postAttempt != nil {
		cfg.postAttempt(attempt)
	}
}
