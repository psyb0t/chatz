package rebound

import (
	"context"
	"math"
	"math/rand/v2" // nosemgrep -- jitter for retry backoff, not crypto
	"time"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
)

type config struct {
	maxAttempts     int
	initialDelay    time.Duration
	delayMultiplier float64
	maximumDelay    time.Duration
	maximumElapsed  time.Duration
	jitter          bool
	retryAfter      RetryAfter
	nonRetryable    []error
	preAttempt      AttemptHandler
	postAttempt     AttemptHandler
	now             func() time.Time
	wait            func(context.Context, time.Duration) error
	random          func() float64
}

func newConfig(options []Option) (config, error) {
	cfg := config{
		maxAttempts:     defaultMaxAttempts,
		initialDelay:    defaultInitialDelay,
		delayMultiplier: defaultDelayMultiplier,
		maximumDelay:    defaultMaximumDelay,
		maximumElapsed:  defaultMaximumAge,
		now:             time.Now,
		wait:            wait,
		random:          rand.Float64,
	}

	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}

	if err := validateConfig(cfg); err != nil {
		return config{}, ctxerrors.Wrap(err, "validate rebound config")
	}

	return cfg, nil
}

func validateConfig(cfg config) error {
	if cfg.maxAttempts <= 0 {
		return invalidArgumentf(
			"rebound WithMaxAttempts(%d): must be positive",
			cfg.maxAttempts,
		)
	}

	if cfg.initialDelay <= 0 {
		return invalidArgumentf(
			"rebound WithInitialDelay(%s): must be positive",
			cfg.initialDelay,
		)
	}

	if cfg.maximumDelay <= 0 {
		return invalidArgumentf(
			"rebound WithMaxDelay(%s): must be positive",
			cfg.maximumDelay,
		)
	}

	if invalidDelayMultiplier(cfg.delayMultiplier) {
		return invalidArgumentf(
			"rebound WithDelayMultiplier(%g): must be positive and finite",
			cfg.delayMultiplier,
		)
	}

	if cfg.maximumDelay < cfg.initialDelay {
		return invalidArgumentf(
			"rebound WithMaxDelay(%s): must be greater than or equal to "+
				"WithInitialDelay(%s)",
			cfg.maximumDelay,
			cfg.initialDelay,
		)
	}

	if cfg.maximumElapsed <= 0 {
		return invalidArgumentf(
			"rebound WithMaxElapsed(%s): must be positive",
			cfg.maximumElapsed,
		)
	}

	if cfg.now == nil || cfg.wait == nil || cfg.random == nil {
		return ctxerrors.Wrap(
			commerr.ErrInvalidState,
			"rebound internal test seams must not be nil",
		)
	}

	return nil
}

func invalidDelayMultiplier(multiplier float64) bool {
	return multiplier <= 0 ||
		math.IsNaN(multiplier) ||
		math.IsInf(multiplier, 0)
}

func invalidArgument(message string) error {
	return ctxerrors.Wrap(commerr.ErrInvalidArgument, message)
}

func invalidArgumentf(format string, arguments ...any) error {
	return ctxerrors.Wrapf(commerr.ErrInvalidArgument, format, arguments...)
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctxerrors.Wrap(ctx.Err(), "wait for rebound retry")
	case <-timer.C:
		return nil
	}
}
