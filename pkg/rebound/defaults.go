package rebound

import "time"

const (
	defaultMaxAttempts     = 3
	defaultInitialDelay    = 250 * time.Millisecond
	defaultMaximumDelay    = 5 * time.Second
	defaultMaximumAge      = 30 * time.Second
	defaultDelayMultiplier = 2.0

	minimumJitterFactor = 0.5
	maximumJitterFactor = 1
)
