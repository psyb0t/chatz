package rebound

import (
	"context"
	"time"
)

// Option configures one Do invocation. Options never mutate a shared retry
// policy: Do builds and validates a fresh configuration for every call.
type Option func(*config)

// RetryAfter extracts a server-requested retry delay from an operation error.
// Returning false leaves the exponential backoff delay unchanged.
type RetryAfter func(error) (time.Duration, bool)

// WithMaxAttempts sets the total number of operation invocations, including
// the first attempt.
func WithMaxAttempts(attempts int) Option {
	return func(cfg *config) {
		cfg.maxAttempts = attempts
	}
}

// WithInitialDelay sets the delay before Rebound's first retry.
func WithInitialDelay(delay time.Duration) Option {
	return func(cfg *config) {
		cfg.initialDelay = delay
	}
}

// WithDelayMultiplier sets the factor applied after each retryable failure.
// It must be positive and finite; values below one deliberately decrease
// subsequent retry delays.
func WithDelayMultiplier(multiplier float64) Option {
	return func(cfg *config) {
		cfg.delayMultiplier = multiplier
	}
}

// WithMaxDelay bounds exponential and retry-after delays.
func WithMaxDelay(delay time.Duration) Option {
	return func(cfg *config) {
		cfg.maximumDelay = delay
	}
}

// WithMaxElapsed sets the total time Rebound may spend retrying, including
// operation time and waits.
func WithMaxElapsed(elapsed time.Duration) Option {
	return func(cfg *config) {
		cfg.maximumElapsed = elapsed
	}
}

// WithJitter enables or disables equal jitter on each retry delay.
func WithJitter(enabled bool) Option {
	return func(cfg *config) {
		cfg.jitter = enabled
	}
}

// WithRetryAfter registers a per-call extractor for server-requested delays.
func WithRetryAfter(extract RetryAfter) Option {
	return func(cfg *config) {
		cfg.retryAfter = extract
	}
}

// WithNonRetryables registers sentinels that stop this Do invocation when
// errors.Is finds one anywhere in an operation error's wrapping chain.
func WithNonRetryables(errs ...error) Option {
	return func(cfg *config) {
		cfg.nonRetryable = append(cfg.nonRetryable, errs...)
	}
}

// WithPreAttemptHandler receives each attempt immediately before the operation
// is invoked. It is synchronous so setup, logs, and metrics preserve order.
func WithPreAttemptHandler(handler AttemptHandler) Option {
	return func(cfg *config) {
		cfg.preAttempt = handler
	}
}

// WithPostAttemptHandler receives each completed attempt, including a
// successful or terminal one. RetryDelay is non-zero only when Rebound will
// invoke the operation again.
func WithPostAttemptHandler(handler AttemptHandler) Option {
	return func(cfg *config) {
		cfg.postAttempt = handler
	}
}

func withClock(now func() time.Time) Option {
	return func(cfg *config) {
		cfg.now = now
	}
}

func withWait(wait func(context.Context, time.Duration) error) Option {
	return func(cfg *config) {
		cfg.wait = wait
	}
}

func withRandom(random func() float64) Option {
	return func(cfg *config) {
		cfg.random = random
	}
}
