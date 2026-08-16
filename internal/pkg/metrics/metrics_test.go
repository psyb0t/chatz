package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_RecordLLM(t *testing.T) {
	t.Parallel()

	m, err := New()
	require.NoError(t, err)

	m.RecordLLMRequest("chatz", "chat", "gpt", OutcomeOK, 2*time.Second)
	m.RecordLLMRequest("chatz", "chat", "gpt", OutcomeOK, 1*time.Second)
	m.RecordLLMTokens("chatz", "chat", "gpt", TokenKindInput, 10)
	m.RecordLLMTokens("chatz", "chat", "gpt", TokenKindInput, 0) // no-op

	reqLabels := prometheus.Labels{
		labelService: "chatz",
		labelStage:   "chat",
		labelModel:   "gpt",
		labelOutcome: OutcomeOK,
	}
	assert.Equal(t, float64(2), testutil.ToFloat64(
		m.llmRequestsTotal.With(reqLabels),
	))

	tokLabels := prometheus.Labels{
		labelService: "chatz",
		labelStage:   "chat",
		labelModel:   "gpt",
		labelKind:    TokenKindInput,
	}
	assert.Equal(t, float64(10), testutil.ToFloat64(
		m.llmTokensTotal.With(tokLabels),
	))
}

func TestMetrics_HandlerServes(t *testing.T) {
	t.Parallel()

	m, err := New()
	require.NoError(t, err)

	// Recorded metrics must show up in the served exposition (same registry the
	// recorder emits into) — the point of wiring /metrics.
	m.RecordLLMRequest("chatz", "chat", "gpt", OutcomeOK, time.Second)

	body := serve(t, m.Handler(), "/metrics")
	assert.Contains(t, body, "llm_requests_total")
	assert.Contains(t, body, `outcome="ok"`)
}

func TestMetrics_PprofHandlerServes(t *testing.T) {
	t.Parallel()

	m, err := New()
	require.NoError(t, err)

	body := serve(t, m.PprofHandler(), "/debug/pprof/")
	assert.Contains(t, body, "goroutine")
}

// serve runs one GET against a handler and returns the 200 body.
func serve(t *testing.T, handler http.Handler, path string) string {
	t.Helper()

	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	raw, err := io.ReadAll(rec.Result().Body)
	require.NoError(t, err)

	return string(raw)
}
