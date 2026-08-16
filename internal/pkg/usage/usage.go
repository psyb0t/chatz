// Package usage decorates an elelem.Driver so every call emits LLM metrics and
// best-effort persists one llm_usage row.
package usage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/psyb0t/chatz/internal/pkg/db/repositories"
	chatzlogging "github.com/psyb0t/chatz/internal/pkg/logging"
	"github.com/psyb0t/chatz/internal/pkg/metrics"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
)

const (
	// serviceName labels the llm metrics + usage rows for this app.
	serviceName = "chatz"
	// auditInsertTimeout caps the detached row write after the call returned.
	auditInsertTimeout = 10 * time.Second
	// unknownModel labels a call whose model wasn't set (bounded fallback).
	unknownModel = "unknown"
)

// attribution carries the owning chat + user for a usage row. It rides on the
// context because elelem's Driver.Stream signature is fixed. The chat loop
// stamps these values on the context before invoking the wrapped driver.
type attribution struct {
	chatID uuid.UUID
	userID uuid.UUID
}

type attributionCtxKey struct{}

// WithAttribution returns a context carrying the owning chat + user id, so a
// usage row recorded during a Stream call under this context is attributed to
// them. The chat loop stamps this before invoking the wrapped client.
func WithAttribution(
	ctx context.Context,
	chatID, userID uuid.UUID,
) context.Context {
	return context.WithValue(ctx, attributionCtxKey{}, attribution{
		chatID: chatID,
		userID: userID,
	})
}

func attributionFromContext(ctx context.Context) (attribution, bool) {
	attr, ok := ctx.Value(attributionCtxKey{}).(attribution)

	return attr, ok
}

// Recorder wraps an elelem.Driver with metrics and usage persistence.
type Recorder struct {
	inner   elelem.Driver
	metrics *metrics.Metrics
	stage   string
	persist bool
}

// Wrap decorates inner. stage labels the pipeline step (e.g. "chat"). persist
// controls the DB write — pass false when no DB is wired (tests).
//

func Wrap(
	inner elelem.Driver,
	m *metrics.Metrics,
	stage string,
	persist bool,
) elelem.Driver {
	return &Recorder{
		inner:   inner,
		metrics: m,
		stage:   stage,
		persist: persist,
	}
}

// Stream runs the inner streaming call, records metrics + a usage row, and
// passes the result through unchanged.
func (r *Recorder) Stream(
	ctx context.Context,
	req elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return r.observe(ctx, req, onDelta, r.inner.Stream)
}

// Complete does the same for the non-streaming call, which elelem takes when
// streaming is off or the provider cannot stream.
//
// It must be recorded identically: the tokens are spent and the row is owed
// either way, and a decorator that only instrumented Stream would make a
// backend's usage silently vanish from the metrics the moment someone set
// WithStreaming(false).
func (r *Recorder) Complete(
	ctx context.Context,
	req elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
) (elelem.Usage, error) {
	return r.observe(ctx, req, onDelta, r.inner.Complete)
}

func (r *Recorder) observe(
	ctx context.Context,
	req elelem.DriverRequest,
	onDelta func(elelem.Delta) error,
	call func(
		context.Context,
		elelem.DriverRequest,
		func(elelem.Delta) error,
	) (elelem.Usage, error),
) (elelem.Usage, error) {
	model := req.Model.ID
	if model == "" {
		model = unknownModel
	}

	start := time.Now()
	tokenUsage, callErr := call(ctx, req, onDelta)
	dur := time.Since(start)

	outcome := metrics.OutcomeOK
	if callErr != nil {
		outcome = metrics.OutcomeError
	}

	r.record(model, outcome, dur, tokenUsage)
	r.persistRow(ctx, model, dur, tokenUsage, callErr)

	// Deliberately unwrapped — a decorator that added a frame here would put
	// this package's name on every provider error the caller inspects.
	return tokenUsage, callErr
}

// ListModels passes through to the inner client.
func (r *Recorder) ListModels(ctx context.Context) ([]string, error) {
	//nolint:wrapcheck // transparent passthrough
	return r.inner.ListModels(ctx)
}

// Capabilities passes through to the inner driver.
func (r *Recorder) Capabilities(model elelem.Model) elelem.Capabilities {
	return r.inner.Capabilities(model)
}

// TokenCounter passes through to the inner driver.
//

func (r *Recorder) TokenCounter() elelem.TokenCounter {
	return r.inner.TokenCounter()
}

func (r *Recorder) record(
	model, outcome string,
	dur time.Duration,
	u elelem.Usage,
) {
	r.metrics.RecordLLMRequest(serviceName, r.stage, model, outcome, dur)

	tokens := func(kind string, n int64) {
		r.metrics.RecordLLMTokens(serviceName, r.stage, model, kind, n)
	}
	tokens(metrics.TokenKindInput, u.Prompt+u.Retry.WastedPromptTokens)
	tokens(metrics.TokenKindOutput, u.Completion+u.Retry.WastedCompletionTokens)
	tokens(metrics.TokenKindCache, u.CacheRead)
}

// persistRow writes one llm_usage row, detached so failed/slow calls still
// record even if the caller's ctx was cancelled. Best-effort: insert failures
// warn and never affect the call result.
func (r *Recorder) persistRow(
	ctx context.Context,
	model string,
	dur time.Duration,
	u elelem.Usage,
	callErr error,
) {
	if !r.persist {
		return
	}

	row := &models.LLMUsage{
		Service:            serviceName,
		Stage:              r.stage,
		Model:              model,
		PromptTokens:       u.Prompt + u.Retry.WastedPromptTokens,
		CachedPromptTokens: u.CacheRead,
		CompletionTokens:   u.Completion + u.Retry.WastedCompletionTokens,
		ReasoningTokens:    u.Reasoning,
		TotalTokens:        u.BilledTotalTokens(),
		DurationMs:         dur.Milliseconds(),
		ErrorMessage:       errMessage(callErr),
	}

	if attr, ok := attributionFromContext(ctx); ok {
		row.ChatID = &attr.chatID
		row.UserID = &attr.userID
	}

	// The column existed from the first migration but nothing ever wrote it:
	// the request id lived only as a logger attribute, and slog exposes no way
	// to read one back. It is a scope value now, so a token spend can finally
	// be tied to the request that caused it. Empty outside a request (a
	// background job, a test), which is the honest answer for those.
	row.RequestID = chatzlogging.RequestIDFromScope(ctx)

	insertCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		auditInsertTimeout,
	)
	defer cancel()

	err := repositories.LLMUsage.WithContext(insertCtx).Create(row)
	if err != nil {
		ctxscope.GetLogger(ctx).Warn(
			"usage: insert audit row failed",
			"err", err,
			"model", model,
			"reason", "usage_write_failed",
		)
	}
}

func errMessage(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
