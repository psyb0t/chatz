package upstreams

import (
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/elelem"
	"github.com/stretchr/testify/assert"
)

func TestControlsFor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		provider config.UpstreamProvider
		model    elelem.Model
		caps     elelem.Capabilities
		metadata config.Model
		want     Controls
	}{
		{
			name:     "no reasoning control and no sampling",
			provider: config.UpstreamProviderAnthropic,
			model:    elelem.Model{ID: "plain"},
			want: Controls{
				ReasoningEfforts: []elelem.ReasoningEffort{},
				SupportsStop:     true,
			},
		},
		{
			name:     "unknown ceiling stops at high",
			provider: config.UpstreamProviderOpenAI,
			model:    elelem.Model{ID: "compat"},
			caps: elelem.Capabilities{
				SupportsReasoningEffort:    true,
				SupportsDisablingReasoning: true,
				SupportsSamplingParams:     true,
				SupportsSamplingPenalties:  true,
				SupportsSeed:               true,
			},
			want: Controls{
				ReasoningEfforts: []elelem.ReasoningEffort{
					"none", "minimal", "low", "medium", "high",
				},
				SupportsSampling:  true,
				SupportsPenalties: true,
				SupportsSeed:      true,
				SupportsStop:      true,
			},
		},
		{
			name:     "model floor and driver ceiling bound the ladder",
			provider: config.UpstreamProviderAnthropic,
			model: elelem.Model{
				ID:              "effort-model",
				ReasoningLevels: elelem.ReasoningLevels{Min: "low"},
			},
			caps: elelem.Capabilities{
				SupportsReasoningEffort: true,
				SupportsSamplingParams:  true,
				MaxReasoningEffort:      "max",
			},
			want: Controls{
				ReasoningEfforts: []elelem.ReasoningEffort{
					"low", "medium", "high", "xhigh", "max",
				},
				SupportsSampling: true,
				SupportsTopK:     true,
				SupportsStop:     true,
			},
		},
		{
			name:     "a known OpenAI reasoning family takes no stop sequences",
			provider: config.UpstreamProviderOpenAI,
			model:    elelem.Model{ID: "reasoning-family"},
			caps: elelem.Capabilities{
				SupportsReasoningEffort: true,
				SupportsSeed:            true,
				MaxReasoningEffort:      "high",
			},
			want: Controls{
				ReasoningEfforts: []elelem.ReasoningEffort{
					"minimal", "low", "medium", "high",
				},
				SupportsSeed: true,
			},
		},
		{
			name:     "configured overrides win",
			provider: config.UpstreamProviderOpenAI,
			model:    elelem.Model{ID: "glm"},
			caps:     elelem.Capabilities{SupportsReasoningEffort: true},
			metadata: config.Model{
				ReasoningEfforts: []string{"none", "high", "max"},
				SupportsTopK:     new(true),
				SupportsStop:     new(true),
			},
			want: Controls{
				ReasoningEfforts: []elelem.ReasoningEffort{
					"none", "high", "max",
				},
				SupportsTopK: true,
				SupportsStop: true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := controlsFor(tc.provider, tc.model, tc.caps, tc.metadata)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestControls_ApplyTo(t *testing.T) {
	t.Parallel()

	params := elelem.GenerationParams{
		Temperature:      new(0.5),
		TopP:             new(0.9),
		FrequencyPenalty: new(0.1),
		PresencePenalty:  new(0.2),
		Seed:             new(int64(3)),
		Stop:             []string{"END"},
		ReasoningEffort:  elelem.ReasoningEffortNone,
		Extra:            map[string]any{"keep": true},
	}

	t.Run("everything accepted", func(t *testing.T) {
		t.Parallel()

		got := AllControls().ApplyTo(params, new(20))
		assert.Equal(t, params.Temperature, got.Temperature)
		assert.Equal(t, elelem.ReasoningEffortNone, got.ReasoningEffort)
		assert.Equal(t, map[string]any{"keep": true, "top_k": 20}, got.Extra)
		assert.Equal(t, map[string]any{"keep": true}, params.Extra,
			"the caller's Extra map is not modified")
	})

	t.Run("nothing accepted", func(t *testing.T) {
		t.Parallel()

		got := Controls{}.ApplyTo(params, new(20))
		assert.Equal(t, elelem.GenerationParams{
			Extra: map[string]any{"keep": true},
		}, got)
	})
}

func TestResolveModel(t *testing.T) {
	t.Parallel()

	glm := resolveModel(
		config.UpstreamProviderZAICoding,
		"glm-5.3",
		config.Model{},
	)
	assert.True(t, glm.SupportsReasoning, "Z.ai catalog entry applies")
	assert.Positive(t, glm.ContextSize)

	plain := resolveModel(
		config.UpstreamProviderOpenAI,
		"gpt-x",
		config.Model{SupportsReasoning: new(true)},
	)
	assert.Equal(t, elelem.Model{ID: "gpt-x", SupportsReasoning: true}, plain)
}
