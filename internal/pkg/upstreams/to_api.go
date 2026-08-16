package upstreams

import (
	"strconv"

	api "github.com/psyb0t/chatz/internal/pkg/http/api"
)

// ModelToAPI projects an available model to the wire shape.
func ModelToAPI(m Model) api.Model {
	out := api.Model{
		Availability: api.ModelAvailabilityAvailable,
		Default:      m.Default,
		Id:           m.ID,
		Upstream:     m.Upstream,
	}

	if m.Alias != "" {
		out.Alias = &m.Alias
	}

	if m.ContextWindow > 0 {
		out.ContextWindow = &m.ContextWindow
	}

	if m.MaxOutputTokens > 0 {
		out.MaxOutputTokens = &m.MaxOutputTokens
	}

	out.SupportsTools = m.SupportsTools
	out.SupportsReasoning = m.SupportsReasoning
	out.SupportsVision = m.SupportsVision
	out.SupportsFiles = m.SupportsFiles

	if m.FirstTokenLatencyMs > 0 {
		out.ExpectedFirstTokenLatencyMs = &m.FirstTokenLatencyMs
	}

	out.InputPricePerMillionTokens = tokenPriceToAPI(
		m.InputTokenPrice,
	)
	out.OutputPricePerMillionTokens = tokenPriceToAPI(
		m.OutputTokenPrice,
	)

	return out
}

func tokenPriceToAPI(price *TokenPrice) *api.TokenPrice {
	if price == nil {
		return nil
	}

	return &api.TokenPrice{
		Amount:   strconv.FormatInt(price.AmountSmallestUnit, 10),
		Currency: price.Currency,
	}
}

// HealthToAPI projects a redacted upstream-health snapshot to the wire shape.
func HealthToAPI(health Health) api.UpstreamHealth {
	response := api.UpstreamHealth{
		ConsecutiveFailures: health.ConsecutiveFailure,
		LastFailureClass:    optionalString(health.LastFailureClass),
		LastOperation:       optionalString(health.LastOperation),
		State:               api.UpstreamHealthState(health.State),
		Upstream:            health.Upstream,
	}

	if health.LastOperation != "" {
		latencyMilliseconds := health.LastLatency.Milliseconds()
		response.LastLatencyMs = &latencyMilliseconds
	}

	if !health.LastSuccessAt.IsZero() {
		response.LastSuccessAt = &health.LastSuccessAt
	}

	if !health.LastFailureAt.IsZero() {
		response.LastFailureAt = &health.LastFailureAt
	}

	return response
}

// HealthsToAPI projects the ordered registry snapshots to the wire shape.
func HealthsToAPI(health []Health) []api.UpstreamHealth {
	responses := make([]api.UpstreamHealth, 0, len(health))
	for _, snapshot := range health {
		responses = append(responses, HealthToAPI(snapshot))
	}

	return responses
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}
