package rebound

import "time"

// Attempt describes one completed operation attempt. RetryDelay is non-zero
// only when Rebound will make another attempt after that delay.
type Attempt struct {
	Number     int
	Err        error
	RetryDelay time.Duration
}

// AttemptHandler observes an operation attempt. Pre-attempt handlers receive
// its number before invocation; post-attempt handlers additionally receive the
// operation error and any scheduled retry delay.
type AttemptHandler func(Attempt)
