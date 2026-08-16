package upstreams

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/ctxerrors/commerr"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	upstreamOllama          = "ollama"
	upstreamBroken          = "broken"
	upstreamOpenai          = "openai"
	upstreamOk              = "ok"
	defaultDiscoveryTimeout = time.Second
	healthSampleUnixSeconds = 1
)

type fakeDriver struct {
	name   string
	models []string
	err    error
}

func trackerFor(ups []config.Upstream) *HealthTracker {
	names := make([]string, 0, len(ups))
	for _, upstream := range ups {
		names = append(names, upstream.Name)
	}

	return NewHealthTracker(names)
}

func (f *fakeDriver) Stream(
	_ context.Context,
	_ elelem.DriverRequest,
	_ func(elelem.Delta) error,
) (elelem.Usage, error) {
	return elelem.Usage{}, nil
}

func (f *fakeDriver) Complete(
	_ context.Context,
	_ elelem.DriverRequest,
	_ func(elelem.Delta) error,
) (elelem.Usage, error) {
	return elelem.Usage{}, nil
}

func (f *fakeDriver) ListModels(_ context.Context) ([]string, error) {
	return f.models, f.err
}

func (f *fakeDriver) Capabilities(elelem.Model) elelem.Capabilities {
	return elelem.Capabilities{}
}

func (f *fakeDriver) TokenCounter() elelem.TokenCounter {
	return elelem.DefaultTokenCounter()
}

// factoryFrom returns a ClientFactory backed by pre-built fake clients keyed by
// upstream name.
func factoryFrom(specs map[string]*fakeDriver) ClientFactory {
	return func(u config.Upstream) *elelem.Client {
		return elelem.New(specs[u.Name])
	}
}

func modelByID(models []Model, id string) (Model, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}

	return Model{}, false
}

func clientName(t *testing.T, reg *Registry, modelID string) string {
	t.Helper()

	client, ok := reg.ClientFor(modelID)
	require.True(t, ok)

	fc, ok := client.Driver().(*fakeDriver)
	require.True(t, ok)

	return fc.name
}

func TestDiscover_MergesAndRoutes(t *testing.T) {
	t.Parallel()

	ups := []config.Upstream{{Name: upstreamOpenai}, {Name: upstreamOllama}}
	specs := map[string]*fakeDriver{
		upstreamOpenai: {
			name:   upstreamOpenai,
			models: []string{"gpt-4", "gpt-3.5"},
		},
		upstreamOllama: {name: upstreamOllama, models: []string{"llama3"}},
	}

	reg := Discover(
		t.Context(),
		ups,
		factoryFrom(specs),
		defaultDiscoveryTimeout,
		trackerFor(ups),
	)

	models := reg.Models()
	require.Len(t, models, 3)
	// Sorted by id.
	assert.Equal(t,
		[]string{"gpt-3.5", "gpt-4", "llama3"},
		[]string{models[0].ID, models[1].ID, models[2].ID},
	)

	assert.Equal(t, "openai", clientName(t, reg, "gpt-4"))
	assert.Equal(t, upstreamOllama, clientName(t, reg, "llama3"))
}

func TestDiscover_AppliesConfiguredModelMetadata(t *testing.T) {
	t.Parallel()

	supportsTools := true
	supportsFiles := true
	ups := []config.Upstream{{Name: upstreamOpenai, Models: []config.Model{{
		ID: "gpt-4", Alias: "Deep", ContextWindow: 128000,
		MaxOutputTokens:     8192,
		SupportsTools:       &supportsTools,
		SupportsFiles:       &supportsFiles,
		FirstTokenLatencyMs: 900,
		InputTokenPrice: &config.TokenPrice{
			AmountSmallestUnit: 15,
			Currency:           "USD",
		},
		OutputTokenPrice: &config.TokenPrice{
			AmountSmallestUnit: 60,
			Currency:           "USD",
		},
	}}}}
	reg := Discover(t.Context(), ups, factoryFrom(map[string]*fakeDriver{
		upstreamOpenai: {name: upstreamOpenai, models: []string{"gpt-4"}},
	}), defaultDiscoveryTimeout, trackerFor(ups))

	model, found := modelByID(reg.Models(), "gpt-4")
	require.True(t, found)
	assert.Equal(t, "Deep", model.Alias)
	assert.Equal(t, 128000, model.ContextWindow)
	assert.Equal(t, 8192, model.MaxOutputTokens)
	require.NotNil(t, model.SupportsTools)
	assert.True(t, *model.SupportsTools)
	require.NotNil(t, model.SupportsFiles)
	assert.True(t, *model.SupportsFiles)
	assert.Equal(t, int64(900), model.FirstTokenLatencyMs)
	inputPrice := model.InputTokenPrice
	require.NotNil(t, inputPrice)
	assert.Equal(t, int64(15), inputPrice.AmountSmallestUnit)
	assert.Equal(t, "USD", inputPrice.Currency)

	outputPrice := model.OutputTokenPrice
	require.NotNil(t, outputPrice)
	assert.Equal(t, int64(60), outputPrice.AmountSmallestUnit)
	assert.Equal(t, "USD", outputPrice.Currency)
}

func TestRegistry_SetDefault(t *testing.T) {
	t.Parallel()

	ups := []config.Upstream{{Name: upstreamOpenai}}
	reg := Discover(t.Context(), ups, factoryFrom(map[string]*fakeDriver{
		upstreamOpenai: {name: upstreamOpenai, models: []string{"a", "b"}},
	}), defaultDiscoveryTimeout, trackerFor(ups))

	require.NoError(t, reg.SetDefault("b"))
	models := reg.Models()
	assert.False(t, models[0].Default)
	assert.True(t, models[1].Default)

	err := reg.SetDefault("missing")
	require.ErrorIs(t, err, commerr.ErrInvalidArgument)
	assert.True(t, errors.Is(err, commerr.ErrInvalidArgument))

	models = reg.Models()
	assert.False(t, models[0].Default)
	assert.True(t, models[1].Default)
}

func TestDiscover_FirstUpstreamWinsDuplicate(t *testing.T) {
	t.Parallel()

	ups := []config.Upstream{{Name: "a"}, {Name: "b"}}
	specs := map[string]*fakeDriver{
		"a": {name: "a", models: []string{"shared", "a-only"}},
		"b": {name: "b", models: []string{"shared", "b-only"}},
	}

	reg := Discover(
		t.Context(),
		ups,
		factoryFrom(specs),
		defaultDiscoveryTimeout,
		trackerFor(ups),
	)

	require.Len(t, reg.Models(), 3) // shared appears once

	shared, ok := modelByID(reg.Models(), "shared")
	require.True(t, ok)
	assert.Equal(t, "a", shared.Upstream)
	assert.Equal(t, "a", clientName(t, reg, "shared"))
}

func TestDiscover_SkipsFailedUpstream(t *testing.T) {
	t.Parallel()

	ups := []config.Upstream{{Name: upstreamBroken}, {Name: upstreamOk}}
	specs := map[string]*fakeDriver{
		upstreamBroken: {name: upstreamBroken, err: assert.AnError},
		upstreamOk:     {name: upstreamOk, models: []string{"m1"}},
	}

	health := trackerFor(ups)
	reg := Discover(
		t.Context(),
		ups,
		factoryFrom(specs),
		defaultDiscoveryTimeout,
		health,
	)

	require.Len(t, reg.Models(), 1)
	assert.Equal(t, "m1", reg.Models()[0].ID)
	assert.Equal(t, "ok", clientName(t, reg, "m1"))

	status, ok := health.Snapshot(upstreamBroken)
	require.True(t, ok)
	assert.Equal(t, HealthStateDegraded, status.State)
}

func TestDiscover_UnknownAndEmpty(t *testing.T) {
	t.Parallel()

	reg := Discover(
		t.Context(),
		nil,
		factoryFrom(map[string]*fakeDriver{}),
		defaultDiscoveryTimeout,
		NewHealthTracker(nil),
	)
	assert.Empty(t, reg.Models())

	_, ok := reg.ClientFor("nope")
	assert.False(t, ok)
}

func TestHealthsToAPI_RedactsProviderDetails(t *testing.T) {
	t.Parallel()

	sampleTime := time.Unix(healthSampleUnixSeconds, 0).UTC()
	responses := HealthsToAPI([]Health{
		{
			Upstream:           upstreamOpenai,
			State:              HealthStateDegraded,
			LastOperation:      "stream",
			LastLatency:        1500 * time.Millisecond,
			LastSuccessAt:      sampleTime,
			LastFailureAt:      sampleTime,
			LastFailureClass:   "upstream_error",
			ConsecutiveFailure: 2,
		},
	})

	require.Len(t, responses, 1)
	response := responses[0]
	assert.Equal(t, upstreamOpenai, response.Upstream)
	assert.Equal(t, "degraded", string(response.State))
	require.NotNil(t, response.LastLatencyMs)
	require.NotNil(t, response.LastSuccessAt)
	require.NotNil(t, response.LastFailureAt)
	require.NotNil(t, response.LastFailureClass)
	assert.Equal(t, int64(1500), *response.LastLatencyMs)
	assert.Equal(t, sampleTime, *response.LastSuccessAt)
	assert.Equal(t, sampleTime, *response.LastFailureAt)
	assert.Equal(t, "upstream_error", *response.LastFailureClass)
	assert.Equal(t, 2, response.ConsecutiveFailures)
}
