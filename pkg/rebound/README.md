# Rebound

Rebound is a small, stateless Go retry primitive for context-aware operations.
Each `Do` call owns its configuration, retry budget, and backoff state. There
are no global policies to share, mutate, or leak across requests.

Use it for operations that are safe to repeat, such as a model catalog fetch
or an HTTP `GET`. Do not use it for an action with possible remote side effects
unless the remote operation is explicitly idempotent.

```bash
go get github.com/psyb0t/rebound
```

## Usage

`Do` receives the caller's context, a context-aware operation, and zero or
more per-call options:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/rebound"
)

const reportEndpoint = "https://reports.example.com/current"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	report, err := fetchReport(ctx, client, reportEndpoint)
	if err != nil {
		slog.Error("fetch report", "err", err)

		return
	}

	fmt.Println(string(report))
}

func fetchReport(
	ctx context.Context,
	client *http.Client,
	endpoint string,
) ([]byte, error) {
	var report []byte

	err := rebound.Do(
		ctx,
		func(ctx context.Context) error {
			request, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				endpoint,
				nil,
			)
			if err != nil {
				return ctxerrors.Wrap(err, "create report request")
			}

			response, err := client.Do(request)
			if err != nil {
				return ctxerrors.Wrap(err, "request report")
			}

			body, readErr := io.ReadAll(response.Body)
			closeErr := response.Body.Close()
			if readErr != nil {
				return ctxerrors.Wrap(readErr, "read report response")
			}
			if closeErr != nil {
				return ctxerrors.Wrap(closeErr, "close report response")
			}

			if response.StatusCode == http.StatusTooManyRequests {
				return ctxerrors.Wrap(
					commerr.ErrRateLimited,
					"report service rate limited the request",
				)
			}
			if response.StatusCode == http.StatusUnauthorized ||
				response.StatusCode == http.StatusForbidden {
				return ctxerrors.Wrap(
					commerr.ErrNotAuthenticated,
					"report request was not authorized",
				)
			}
			if response.StatusCode >= http.StatusBadRequest &&
				response.StatusCode < http.StatusInternalServerError {
				return ctxerrors.Wrap(
					commerr.ErrInvalidArgument,
					"report request was rejected",
				)
			}
			if response.StatusCode >= http.StatusInternalServerError {
				return ctxerrors.Wrap(
					commerr.ErrUnavailable,
					"report service is unavailable",
				)
			}

			report = body

			return nil
		},
		rebound.WithMaxAttempts(4),
		rebound.WithInitialDelay(200*time.Millisecond),
		rebound.WithDelayMultiplier(1.4),
		rebound.WithMaxDelay(2*time.Second),
		rebound.WithMaxElapsed(10*time.Second),
		rebound.WithJitter(true),
		rebound.WithNonRetryables(
			commerr.ErrInvalidArgument,
			commerr.ErrNotAuthenticated,
		),
		rebound.WithPreAttemptHandler(func(attempt rebound.Attempt) {
			slog.DebugContext(
				ctx,
				"starting report attempt",
				"attempt", attempt.Number,
			)
		}),
		rebound.WithPostAttemptHandler(func(attempt rebound.Attempt) {
			slog.DebugContext(
				ctx,
				"report attempt completed",
				"attempt", attempt.Number,
				"will_retry", attempt.RetryDelay > 0,
			)
		}),
	)
	if err == nil {
		return report, nil
	}

	var exhausted *rebound.ExhaustedError
	if errors.As(err, &exhausted) {
		return nil, ctxerrors.Wrapf(
			err,
			"fetch report exhausted after %d attempts",
			exhausted.Attempts,
		)
	}

	return nil, err
}
```

The closure captures `report` from `fetchReport`'s enclosing scope. On success, `Do`
returns `nil` and the caller uses that value normally. Rebound intentionally
does not offer a separate value-returning API: the closure remains the one
place that owns the operation's state and cleanup.

## Behavior

The operation runs immediately. A failed retryable attempt waits before the
next invocation. Rebound stops when any of the following happens:

- The operation returns `nil`.
- The operation returns an error matched by `WithNonRetryables`.
- The operation returns or wraps `context.Canceled` or
  `context.DeadlineExceeded`.
- The caller's context ends.
- The maximum attempt count or elapsed-time budget is exhausted.

Rebound uses `errors.Is`, so wrapped sentinel errors classify the same as the
sentinels themselves. Context cancellation and deadlines are never retried,
even if the caller did not register them as non-retryable.

## Defaults

With no options, one `Do` invocation uses:

| Setting | Default |
| --- | --- |
| Maximum attempts | 3 total invocations, including the first |
| Initial delay | 250 ms |
| Maximum delay | 5 s |
| Maximum elapsed time | 30 s, including operation time and waits |
| Jitter | Disabled |

The base retry delays use the configured multiplier: the first retry waits the
initial delay, then each later retry multiplies that delay until the maximum
delay. The default multiplier is `2`.

## Options

All options affect only the `Do` call that receives them.

| Option | Effect |
| --- | --- |
| `WithMaxAttempts(n)` | Sets the total number of operation invocations. `n` must be positive; the initial invocation counts as attempt one. |
| `WithInitialDelay(d)` | Sets the delay before the first retry. `d` must be positive. |
| `WithDelayMultiplier(f)` | Sets the positive finite factor applied to each later delay. Values below `1` deliberately decrease delays; the default is `2`. |
| `WithMaxDelay(d)` | Caps exponential and server-requested retry delays. `d` must be positive and no smaller than the initial delay. |
| `WithMaxElapsed(d)` | Caps all time spent in the call, including work and waits. `d` must be positive. |
| `WithJitter(enabled)` | Enables equal jitter. A selected delay is randomized between 50% and 100% of its calculated value. |
| `WithRetryAfter(extractor)` | Lets the operation provide a server-requested delay. When the extractor returns a positive duration and `true`, that duration replaces exponential backoff, still capped by `WithMaxDelay`. |
| `WithNonRetryables(errs...)` | Registers error sentinels that must stop retrying. Each operation error is checked with `errors.Is`. Nil entries are ignored. |
| `WithPreAttemptHandler(handler)` | Receives every attempt immediately before the operation runs. `Number` is set; `Err` and `RetryDelay` are zero values. |
| `WithPostAttemptHandler(handler)` | Receives every completed operation attempt synchronously. `RetryDelay` is non-zero only when Rebound will retry. |

`WithRetryAfter` is usually paired with a typed operation error that retains a
parsed `Retry-After` value:

```go
rebound.WithRetryAfter(func(err error) (time.Duration, bool) {
	var responseErr *ResponseError
	if !errors.As(err, &responseErr) || responseErr.RetryAfter <= 0 {
		return 0, false
	}

	return responseErr.RetryAfter, true
})
```

## Errors

Invalid input to `Do` returns an error matching `commerr.ErrInvalidArgument`.
Its wrapped message names the exact option and value, such as
`WithDelayMultiplier(NaN): must be positive and finite` or
`WithMaxDelay(1ms): must be greater than or equal to WithInitialDelay(1s)`.

If retryable failures exhaust the budget, `Do` returns an
`*ExhaustedError`. It matches both `commerr.ErrExhausted` and the final
operation error:

```go
if errors.Is(err, commerr.ErrExhausted) {
	var exhausted *rebound.ExhaustedError
	if errors.As(err, &exhausted) {
		// exhausted.Attempts is the number of invocations made.
		// exhausted.Cause is the final operation failure.
	}
}
```

An error classified as non-retryable is returned immediately, preserving its
wrapping chain for `errors.Is` and `errors.As`.

## Choosing the right operations

Good candidates are read-only requests, idempotent writes protected by a
remote idempotency key, and retryable connection setup. Do not retry a tool
invocation, payment, email send, or mutation solely because its response was
lost: the remote system may already have completed it. Make that operation
idempotent first, then configure the appropriate non-retryable sentinels.
