package upstreams

import (
	"testing"

	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelToAPI_ProjectsConfiguredMetadata(t *testing.T) {
	t.Parallel()

	supportsTools := true
	supportsReasoning := false
	supportsVision := true
	supportsFiles := true

	got := ModelToAPI(Model{
		ID:                  "analysis-model",
		Upstream:            "gateway",
		Alias:               "Deep analysis",
		ContextWindow:       128000,
		MaxOutputTokens:     8192,
		SupportsTools:       &supportsTools,
		SupportsReasoning:   &supportsReasoning,
		SupportsVision:      &supportsVision,
		SupportsFiles:       &supportsFiles,
		FirstTokenLatencyMs: 900,
		InputTokenPrice: &TokenPrice{
			AmountSmallestUnit: 15,
			Currency:           "USD",
		},
		OutputTokenPrice: &TokenPrice{
			AmountSmallestUnit: 60,
			Currency:           "USD",
		},
		Default: true,
	})

	assert.Equal(t, "analysis-model", got.Id)
	assert.Equal(t, "gateway", got.Upstream)
	assert.Equal(t, api.ModelAvailabilityAvailable, got.Availability)
	assert.True(t, got.Default)
	require.NotNil(t, got.Alias)
	assert.Equal(t, "Deep analysis", *got.Alias)
	require.NotNil(t, got.ContextWindow)
	assert.Equal(t, 128000, *got.ContextWindow)
	require.NotNil(t, got.MaxOutputTokens)
	assert.Equal(t, 8192, *got.MaxOutputTokens)
	require.NotNil(t, got.SupportsTools)
	assert.True(t, *got.SupportsTools)
	require.NotNil(t, got.SupportsReasoning)
	assert.False(t, *got.SupportsReasoning)
	require.NotNil(t, got.SupportsVision)
	assert.True(t, *got.SupportsVision)
	require.NotNil(t, got.SupportsFiles)
	assert.True(t, *got.SupportsFiles)
	require.NotNil(t, got.ExpectedFirstTokenLatencyMs)
	assert.Equal(t, int64(900), *got.ExpectedFirstTokenLatencyMs)
	require.NotNil(t, got.InputPricePerMillionTokens)
	assert.Equal(t, "15", got.InputPricePerMillionTokens.Amount)
	assert.Equal(t, "USD", got.InputPricePerMillionTokens.Currency)
	require.NotNil(t, got.OutputPricePerMillionTokens)
	assert.Equal(t, "60", got.OutputPricePerMillionTokens.Amount)
	assert.Equal(t, "USD", got.OutputPricePerMillionTokens.Currency)
}

func TestModelToAPI_OmitsUnknownMetadata(t *testing.T) {
	t.Parallel()

	got := ModelToAPI(Model{ID: "base", Upstream: "gateway"})

	assert.Nil(t, got.Alias)
	assert.Nil(t, got.ContextWindow)
	assert.Nil(t, got.MaxOutputTokens)
	assert.Nil(t, got.SupportsTools)
	assert.Nil(t, got.SupportsReasoning)
	assert.Nil(t, got.SupportsVision)
	assert.Nil(t, got.SupportsFiles)
	assert.Nil(t, got.ExpectedFirstTokenLatencyMs)
	assert.Nil(t, got.InputPricePerMillionTokens)
	assert.Nil(t, got.OutputPricePerMillionTokens)
}
