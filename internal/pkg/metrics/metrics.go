// Package metrics owns chatz's Prometheus registry and the per-subsystem
// metric vectors. It is constructed once at boot (no package globals) and
// passed to the recorders that emit; the /metrics handler and the pprof mux are
// served on internal-only ports (never the public ingress).
package metrics

import (
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/psyb0t/ctxerrors"
)

// Token kinds for the llm_tokens_total counter.
const (
	TokenKindInput  = "input"
	TokenKindOutput = "output"
	TokenKindCache  = "cache_read"
)

// Outcomes for request counters (bounded set).
const (
	OutcomeOK    = "ok"
	OutcomeError = "error"
)

// Prometheus label names.
const (
	labelService = "service"
	labelStage   = "stage"
	labelModel   = "model"
	labelKind    = "kind"
	labelOutcome = "outcome"
)

// llmDurationBuckets brackets LLM call latency (streaming calls run long).
//
//nolint:gochecknoglobals // histogram bucket bounds in seconds
var llmDurationBuckets = []float64{0.25, 0.5, 1, 2, 5, 10, 20, 30, 60, 120}

// Metrics holds the registry and the metric vectors. Emit via the Record*
// methods; expose via Handler / PprofHandler.
type Metrics struct {
	registry *prometheus.Registry

	llmRequestDuration *prometheus.HistogramVec
	llmRequestsTotal   *prometheus.CounterVec
	llmTokensTotal     *prometheus.CounterVec
}

// New builds the registry, registers the runtime + build-info collectors and
// the subsystem vectors, and returns the Metrics.
func New() (*Metrics, error) {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,
		llmRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "llm_request_duration_seconds",
				Help:    "LLM chat/completions call latency.",
				Buckets: llmDurationBuckets,
			},
			[]string{labelService, labelStage, labelModel, labelOutcome},
		),
		llmRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_requests_total",
				Help: "LLM chat/completions calls.",
			},
			[]string{labelService, labelStage, labelModel, labelOutcome},
		),
		llmTokensTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "llm_tokens_total",
				Help: "LLM tokens counted per call, by kind.",
			},
			[]string{labelService, labelStage, labelModel, labelKind},
		),
	}

	if err := m.register(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Metrics) register() error {
	collectorSet := []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.llmRequestDuration,
		m.llmRequestsTotal,
		m.llmTokensTotal,
	}

	for _, c := range collectorSet {
		if err := m.registry.Register(c); err != nil {
			return ctxerrors.Wrap(err, "register collector")
		}
	}

	return nil
}

// RecordLLMRequest records one LLM call's latency + count under the outcome.
func (m *Metrics) RecordLLMRequest(
	service, stage, model, outcome string,
	dur time.Duration,
) {
	labels := prometheus.Labels{
		labelService: service,
		labelStage:   stage,
		labelModel:   model,
		labelOutcome: outcome,
	}
	m.llmRequestDuration.With(labels).Observe(dur.Seconds())
	m.llmRequestsTotal.With(labels).Inc()
}

// RecordLLMTokens adds token counts for a call, by kind.
func (m *Metrics) RecordLLMTokens(
	service, stage, model, kind string,
	n int64,
) {
	if n <= 0 {
		return
	}

	m.llmTokensTotal.With(prometheus.Labels{
		labelService: service,
		labelStage:   stage,
		labelModel:   model,
		labelKind:    kind,
	}).Add(float64(n))
}

// Handler serves the Prometheus exposition for the registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// PprofHandler returns a mux with the net/http/pprof endpoints under
// /debug/pprof/. Internal-only: never mount on the public ingress.
func (m *Metrics) PprofHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	return mux
}
