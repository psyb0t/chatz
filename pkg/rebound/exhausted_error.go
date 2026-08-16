package rebound

import (
	"fmt"

	"github.com/psyb0t/ctxerrors/commerr"
)

// ExhaustedError reports that an operation remained retryable until Rebound's
// attempt or elapsed-time budget ended.
type ExhaustedError struct {
	Attempts int
	Cause    error
}

func (e *ExhaustedError) Error() string {
	if e == nil {
		return ""
	}

	return fmt.Sprintf(
		"rebound retry budget exhausted after %d attempts: %v",
		e.Attempts,
		e.Cause,
	)
}

// Unwrap exposes both the shared exhaustion sentinel and the final operation
// failure, so errors.Is works for either.
func (e *ExhaustedError) Unwrap() []error {
	if e == nil {
		return nil
	}

	return []error{commerr.ErrExhausted, e.Cause}
}
