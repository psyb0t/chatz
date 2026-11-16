package upstreams

import (
	"maps"
	"slices"

	"github.com/psyb0t/chatz/internal/pkg/config"
	"github.com/psyb0t/elelem"
	"github.com/psyb0t/elelem/drivers/zaicoding"
)

// extraTopKKey is the request body field that carries top_k. elelem has no
// typed field for it; both the Anthropic driver and OpenAI-compatible servers
// read it from the top level of the body.
const extraTopKKey = "top_k"

// Controls lists the generation settings one model accepts. The UI shows only
// these, and ApplyTo drops the rest so a chat whose settings were made for
// another model still runs.
type Controls struct {
	// ReasoningEfforts holds the effort levels the model takes, lowest first.
	// elelem.ReasoningEffortNone, when present, turns thinking off.
	ReasoningEfforts  []elelem.ReasoningEffort
	SupportsSampling  bool
	SupportsPenalties bool
	SupportsSeed      bool
	SupportsTopK      bool
	SupportsStop      bool
}

// effortLadder is elelem's effort vocabulary in rank order, without "none".
//
//nolint:gochecknoglobals // an ordered constant list; Go has no const slices.
var effortLadder = []elelem.ReasoningEffort{
	elelem.ReasoningEffortMinimal,
	elelem.ReasoningEffortLow,
	elelem.ReasoningEffortMedium,
	elelem.ReasoningEffortHigh,
	elelem.ReasoningEffortXHigh,
	elelem.ReasoningEffortMax,
}

// defaultEffortCeiling bounds the advertised levels when the driver reports
// no ceiling, which it does for model IDs it does not know. low, medium and
// high are the levels OpenAI-compatible servers commonly accept; an operator
// who knows better lists the exact set in reasoningEfforts.
const defaultEffortCeiling = elelem.ReasoningEffortHigh

// resolveModel builds the elelem model a turn runs against. Z.ai Coding
// models start from the driver's catalog entry, which carries their context
// window and reasoning levels. Configured metadata wins where it is set.
func resolveModel(
	provider config.UpstreamProvider,
	id string,
	metadata config.Model,
) elelem.Model {
	model := elelem.Model{ID: id}
	if provider == config.UpstreamProviderZAICoding {
		model = zaicoding.LookupModel(id)
	}

	if metadata.SupportsReasoning != nil {
		model.SupportsReasoning = *metadata.SupportsReasoning
	}

	return model
}

// controlsFor derives a model's controls from its driver capabilities and the
// instance overrides in its configured metadata.
func controlsFor(
	provider config.UpstreamProvider,
	model elelem.Model,
	capabilities elelem.Capabilities,
	metadata config.Model,
) Controls {
	controls := Controls{
		ReasoningEfforts:  reasoningEfforts(model, capabilities),
		SupportsSampling:  capabilities.SupportsSamplingParams,
		SupportsPenalties: capabilities.SupportsSamplingPenalties,
		SupportsSeed:      capabilities.SupportsSeed,
		SupportsTopK: provider == config.UpstreamProviderAnthropic &&
			capabilities.SupportsSamplingParams,
		// The OpenAI reasoning families the driver knows reject `stop` along
		// with the sampling knobs, and the driver signals them by turning
		// sampling off. Anthropic and Z.ai take stop sequences on every model.
		SupportsStop: provider != config.UpstreamProviderOpenAI ||
			capabilities.SupportsSamplingParams,
	}

	if len(metadata.ReasoningEfforts) > 0 {
		controls.ReasoningEfforts = slices.Clone(metadata.ReasoningEfforts)
	}

	if metadata.SupportsTopK != nil {
		controls.SupportsTopK = *metadata.SupportsTopK
	}

	if metadata.SupportsStop != nil {
		controls.SupportsStop = *metadata.SupportsStop
	}

	return controls
}

func reasoningEfforts(
	model elelem.Model,
	capabilities elelem.Capabilities,
) []elelem.ReasoningEffort {
	efforts := make([]elelem.ReasoningEffort, 0, len(effortLadder)+1)
	if capabilities.SupportsDisablingReasoning {
		efforts = append(efforts, elelem.ReasoningEffortNone)
	}

	if !capabilities.SupportsReasoningEffort {
		return efforts
	}

	ceiling := capabilities.MaxReasoningEffort
	if ceiling == elelem.ReasoningEffortUnset {
		ceiling = defaultEffortCeiling
	}

	floorIndex := max(slices.Index(effortLadder, model.ReasoningLevelMin()), 0)

	ceilingIndex := slices.Index(effortLadder, ceiling)
	if ceilingIndex < 0 {
		ceilingIndex = slices.Index(effortLadder, defaultEffortCeiling)
	}

	if floorIndex > ceilingIndex {
		return efforts
	}

	return append(efforts, effortLadder[floorIndex:ceilingIndex+1]...)
}

// AcceptsEffort reports whether the model takes the given effort level.
func (c Controls) AcceptsEffort(effort elelem.ReasoningEffort) bool {
	return slices.Contains(c.ReasoningEfforts, effort)
}

// ApplyTo returns params without the settings the model does not accept, and
// with top_k moved into Extra where the model takes it. topK is nil when the
// chat has no top_k set.
func (c Controls) ApplyTo(
	params elelem.GenerationParams,
	topK *int,
) elelem.GenerationParams {
	if !c.SupportsSampling {
		params.Temperature = nil
		params.TopP = nil
	}

	if !c.SupportsPenalties {
		params.FrequencyPenalty = nil
		params.PresencePenalty = nil
	}

	if !c.SupportsSeed {
		params.Seed = nil
	}

	if !c.SupportsStop {
		params.Stop = nil
	}

	if params.ReasoningEffort != elelem.ReasoningEffortUnset &&
		!c.AcceptsEffort(params.ReasoningEffort) {
		params.ReasoningEffort = elelem.ReasoningEffortUnset
	}

	if topK != nil && c.SupportsTopK {
		params.Extra = maps.Clone(params.Extra)
		if params.Extra == nil {
			params.Extra = make(map[string]any, 1)
		}

		params.Extra[extraTopKKey] = *topK
	}

	return params
}

// UnknownModelControls is what a turn sends when no registry describes its
// model: only the sampling settings and stop sequences nearly every endpoint
// takes. Reasoning effort, penalties, seed and top_k stay off rather than
// risk a refused request.
func UnknownModelControls() Controls {
	return Controls{
		ReasoningEfforts: []elelem.ReasoningEffort{},
		SupportsSampling: true,
		SupportsStop:     true,
	}
}

// AllControls accepts every setting.
func AllControls() Controls {
	return Controls{
		ReasoningEfforts: append(
			[]elelem.ReasoningEffort{elelem.ReasoningEffortNone},
			effortLadder...,
		),
		SupportsSampling:  true,
		SupportsPenalties: true,
		SupportsSeed:      true,
		SupportsTopK:      true,
		SupportsStop:      true,
	}
}
