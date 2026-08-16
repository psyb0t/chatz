// Package upstreams discovers models across the configured OpenAI-compatible
// endpoints and routes a chosen model id back to the client that serves it. It
// calls each upstream's /models endpoint and merges the results (e.g. an Ollama
// base + an OpenAI base expose a single combined list); discovery is
// best-effort so one unreachable upstream doesn't hide the others.
package upstreams

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/chatz/internal/pkg/operations"
	"github.com/psyb0t/chatz/pkg/rebound"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/ctxscope"
	"github.com/psyb0t/elelem"
)

const (
	discoveryAttempts     = 2
	discoveryInitialDelay = 250 * time.Millisecond
)

// Model is a discovered model id and the upstream that serves it.
type Model struct {
	ID                  string `json:"id"`
	Upstream            string `json:"upstream"`
	Alias               string `json:"alias,omitempty"`
	ContextWindow       int    `json:"contextWindow,omitempty"`
	MaxOutputTokens     int    `json:"maxOutputTokens,omitempty"`
	SupportsTools       *bool  `json:"supportsTools,omitempty"`
	SupportsReasoning   *bool  `json:"supportsReasoning,omitempty"`
	SupportsVision      *bool  `json:"supportsVision,omitempty"`
	SupportsFiles       *bool  `json:"supportsFiles,omitempty"`
	FirstTokenLatencyMs int64
	InputTokenPrice     *TokenPrice
	OutputTokenPrice    *TokenPrice
	Default             bool `json:"default"`
}

// TokenPrice is a model's configured price per million input or output tokens.
// AmountSmallestUnit uses the configured currency's smallest unit.
type TokenPrice struct {
	AmountSmallestUnit int64
	Currency           string
}

// Candidate is one configured model and the provider client that serves it.
// CandidatesFor always returns the selected model first, followed by its
// explicitly configured fallback order.
type Candidate struct {
	ModelID string
	Client  *elelem.Client
}

// ClientFactory builds the elelem client for an upstream. Injected so tests can
// supply fakes instead of real HTTP clients.
type ClientFactory func(u config.Upstream) *elelem.Client

// Registry is the merged view of models across upstreams plus the per-upstream
// clients, so a chosen model routes to its owner.
type Registry struct {
	models    []Model
	clients   map[string]*elelem.Client
	owner     map[string]string // model id -> upstream name
	fallbacks map[string][]string
	upstreams []string
	health    *HealthTracker
}

// Discover builds each upstream's client, lists its models, and merges them.
// The first upstream to claim a model id wins (later duplicates are skipped
// with a warning). An upstream whose /models call fails is logged and skipped —
// the rest still populate the registry.
func Discover(
	ctx context.Context,
	ups []config.Upstream,
	factory ClientFactory,
	connectTimeout time.Duration,
	health *HealthTracker,
) *Registry {
	logger := ctxscope.GetLogger(ctx)

	reg := &Registry{
		clients:   make(map[string]*elelem.Client, len(ups)),
		owner:     make(map[string]string),
		fallbacks: make(map[string][]string),
		health:    health,
		upstreams: make([]string, 0, len(ups)),
	}

	for _, u := range ups {
		reg.upstreams = append(reg.upstreams, u.Name)
		client := factory(u)
		reg.clients[u.Name] = client

		startedAt := time.Now()

		ids, err := listModels(ctx, client, connectTimeout)
		if err != nil {
			health.RecordFailure(
				u.Name,
				"model_discovery",
				time.Since(startedAt),
				err,
			)
			logger.Warn("upstream models discovery failed",
				"upstream", u.Name,
				"err", err,
				"reason", "list_models_failed",
			)

			continue
		}

		health.RecordSuccess(
			u.Name,
			"model_discovery",
			time.Since(startedAt),
		)

		reg.merge(ctx, u, ids)
	}

	sort.Slice(reg.models, func(i, j int) bool {
		return reg.models[i].ID < reg.models[j].ID
	})

	logger.Info("upstream model discovery complete",
		"upstreams", len(ups),
		"models", len(reg.models),
	)

	return reg
}

// SetFallbacks applies model fallback lists after discovery, when the
// registry can prove every target model is currently routable. A configured
// source model that is not advertised is ignored: discovery is best-effort,
// so a temporarily unavailable upstream must not prevent the rest from
// starting. An advertised source with an unavailable target is rejected.
func (r *Registry) SetFallbacks(configured []config.Upstream) error {
	fallbacks := make(map[string][]string)

	for _, upstream := range configured {
		for _, model := range upstream.Models {
			if len(model.FallbackModels) == 0 {
				continue
			}

			if _, available := r.owner[model.ID]; !available {
				continue
			}

			for _, fallbackID := range model.FallbackModels {
				if _, available := r.owner[fallbackID]; !available {
					return ctxerrors.Wrapf(
						commerr.ErrInvalidArgument,
						//nolint:lll // one error message; a line break changes its API text
						"configured fallback model %q for %q was not advertised by an upstream",
						fallbackID,
						model.ID,
					)
				}
			}

			fallbacks[model.ID] = append([]string(nil), model.FallbackModels...)
		}
	}

	r.fallbacks = fallbacks

	return nil
}

func listModels(
	ctx context.Context,
	client *elelem.Client,
	connectTimeout time.Duration,
) ([]string, error) {
	if connectTimeout <= 0 {
		return nil, ctxerrors.Wrap(
			ErrInvalidDiscovery,
			"validate model discovery timeout",
		)
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	var models []string

	err := rebound.Do(
		discoveryCtx,
		func(retryCtx context.Context) error {
			var err error

			models, err = client.Driver().ListModels(retryCtx)
			if err != nil {
				return ctxerrors.Wrap(err, "list upstream models")
			}

			return nil
		},
		rebound.WithMaxAttempts(discoveryAttempts),
		rebound.WithInitialDelay(discoveryInitialDelay),
		rebound.WithMaxElapsed(connectTimeout),
		rebound.WithNonRetryables(
			context.Canceled,
			context.DeadlineExceeded,
			commerr.ErrInvalidArgument,
			commerr.ErrNotAuthenticated,
		),
	)
	if err == nil {
		return models, nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return nil, ctxerrors.Wrap(ErrDiscoveryTimeout, "list upstream models")
	}

	return nil, ctxerrors.Wrap(err, "discover upstream models")
}

func (r *Registry) merge(
	ctx context.Context,
	upstream config.Upstream,
	ids []string,
) {
	logger := ctxscope.GetLogger(ctx)

	metadata := make(map[string]config.Model, len(upstream.Models))
	for _, model := range upstream.Models {
		metadata[model.ID] = model
	}

	for _, id := range ids {
		if kept, dup := r.owner[id]; dup {
			logger.Warn("duplicate model across upstreams, keeping first",
				"model", id,
				"kept", kept,
				"skipped", upstream.Name,
				"reason", "model_conflict",
			)

			continue
		}

		r.owner[id] = upstream.Name
		r.models = append(
			r.models,
			modelFromConfig(id, upstream.Name, metadata[id]),
		)
	}
}

func modelFromConfig(id, upstream string, metadata config.Model) Model {
	return Model{
		ID:                  id,
		Upstream:            upstream,
		Alias:               metadata.Alias,
		ContextWindow:       metadata.ContextWindow,
		MaxOutputTokens:     metadata.MaxOutputTokens,
		SupportsTools:       metadata.SupportsTools,
		SupportsReasoning:   metadata.SupportsReasoning,
		SupportsVision:      metadata.SupportsVision,
		SupportsFiles:       metadata.SupportsFiles,
		FirstTokenLatencyMs: metadata.FirstTokenLatencyMs,
		InputTokenPrice: tokenPriceFromConfig(
			metadata.InputTokenPrice,
		),
		OutputTokenPrice: tokenPriceFromConfig(
			metadata.OutputTokenPrice,
		),
	}
}

func tokenPriceFromConfig(price *config.TokenPrice) *TokenPrice {
	if price == nil {
		return nil
	}

	return &TokenPrice{
		AmountSmallestUnit: price.AmountSmallestUnit,
		Currency:           price.Currency,
	}
}

// Models returns the merged model list, sorted by id.
func (r *Registry) Models() []Model {
	return append([]Model(nil), r.models...)
}

// SetDefault marks exactly one advertised model as the instance default. An
// empty model ID leaves the registry without an explicit default.
func (r *Registry) SetDefault(modelID string) error {
	if modelID == "" {
		return nil
	}

	if _, found := r.owner[modelID]; !found {
		return ctxerrors.Wrapf(
			commerr.ErrInvalidArgument,
			"configured default model %q was not advertised by an upstream",
			modelID,
		)
	}

	for index := range r.models {
		r.models[index].Default = r.models[index].ID == modelID
	}

	return nil
}

// Health returns current upstream status in the original configuration order.
func (r *Registry) Health() []Health {
	return r.health.Snapshots(r.upstreams)
}

// ReadinessHealth returns the redacted health subset for admin readiness.
func (r *Registry) ReadinessHealth() []operations.UpstreamHealth {
	healths := r.Health()

	readinessHealths := make([]operations.UpstreamHealth, 0, len(healths))
	for _, health := range healths {
		readinessHealths = append(readinessHealths, operations.UpstreamHealth{
			Upstream:           health.Upstream,
			State:              operations.UpstreamState(health.State),
			LastOperation:      health.LastOperation,
			LastLatency:        health.LastLatency,
			LastSuccessAt:      health.LastSuccessAt,
			LastFailureAt:      health.LastFailureAt,
			LastFailureClass:   health.LastFailureClass,
			ConsecutiveFailure: health.ConsecutiveFailure,
		})
	}

	return readinessHealths
}

// ClientFor returns the client that serves modelID, or false if no upstream
// advertised that model.
//

func (r *Registry) ClientFor(
	modelID string,
) (*elelem.Client, bool) {
	owner, ok := r.owner[modelID]
	if !ok {
		return nil, false
	}

	client, ok := r.clients[owner]

	return client, ok
}

// CandidatesFor resolves a selected model into its primary client followed by
// its validated fallbacks. It returns no candidates when the selected model is
// not currently routable.
func (r *Registry) CandidatesFor(modelID string) []Candidate {
	modelIDs := append([]string{modelID}, r.fallbacks[modelID]...)
	candidates := make([]Candidate, 0, len(modelIDs))

	for _, candidateModelID := range modelIDs {
		client, ok := r.ClientFor(candidateModelID)
		if !ok {
			continue
		}

		candidates = append(candidates, Candidate{
			ModelID: candidateModelID,
			Client:  client,
		})
	}

	return candidates
}
