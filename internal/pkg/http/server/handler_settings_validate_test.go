package server

import (
	"strings"
	"testing"

	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/stretchr/testify/assert"
)

// TestValidateChatSettings walks every bound the settings validator enforces:
// temperature and topP ranges, positive token caps, and the reasoning-effort
// enum. A fully valid body and an all-unset body both pass.
func TestValidateChatSettings(t *testing.T) {
	t.Parallel()

	validTemp, validTopP := 1.0, 0.5
	validOut, validHist := 100, 1000
	highEffort := api.ReasoningEffortHigh
	noneEffort := api.ReasoningEffortNone

	highTemp := 2.5
	highTopP := 1.5
	zeroTokens := 0
	badEffort := api.ReasoningEffort("turbo")
	longStop := strings.Repeat("x", 65)
	nulPrompt := "before\x00after"
	longPrompt := strings.Repeat("x", 16001)

	testCases := []struct {
		name string
		body *api.ChatSettings
		ok   bool
	}{
		{
			name: "fully valid",
			body: &api.ChatSettings{
				Temperature:      &validTemp,
				TopP:             &validTopP,
				MaxOutputTokens:  &validOut,
				MaxHistoryTokens: &validHist,
				ReasoningEffort:  &highEffort,
			},
			ok: true,
		},
		{name: "all unset", body: &api.ChatSettings{}, ok: true},
		{
			name: "thinking off",
			body: &api.ChatSettings{ReasoningEffort: &noneEffort},
			ok:   true,
		},
		{
			name: "new controls valid",
			body: &api.ChatSettings{
				TopK:             new(40),
				FrequencyPenalty: new(-2.0),
				PresencePenalty:  new(2.0),
				Seed:             new(int64(0)),
				Stop:             &[]string{"a", "b", "c", "d"},
				SystemPrompt:     new("Be brief."),
			},
			ok: true,
		},
		{name: "topK zero", body: &api.ChatSettings{TopK: new(0)}, ok: false},
		{
			name: "topK too high",
			body: &api.ChatSettings{TopK: new(1001)},
			ok:   false,
		},
		{
			name: "frequencyPenalty too low",
			body: &api.ChatSettings{FrequencyPenalty: new(-2.1)},
			ok:   false,
		},
		{
			name: "presencePenalty too high",
			body: &api.ChatSettings{PresencePenalty: new(2.1)},
			ok:   false,
		},
		{
			name: "negative seed",
			body: &api.ChatSettings{Seed: new(int64(-1))},
			ok:   false,
		},
		{
			name: "too many stop sequences",
			body: &api.ChatSettings{Stop: &[]string{"a", "b", "c", "d", "e"}},
			ok:   false,
		},
		{
			name: "empty stop sequence",
			body: &api.ChatSettings{Stop: &[]string{""}},
			ok:   false,
		},
		{
			name: "long stop sequence",
			body: &api.ChatSettings{Stop: &[]string{longStop}},
			ok:   false,
		},
		{
			name: "whitespace-only stop sequence",
			body: &api.ChatSettings{Stop: &[]string{"\n"}},
			ok:   false,
		},
		{
			name: "stop sequence with NUL",
			body: &api.ChatSettings{Stop: &[]string{"a\x00b"}},
			ok:   false,
		},
		{
			name: "system prompt with NUL",
			body: &api.ChatSettings{SystemPrompt: &nulPrompt},
			ok:   false,
		},
		{
			name: "long system prompt",
			body: &api.ChatSettings{SystemPrompt: &longPrompt},
			ok:   false,
		},
		{
			name: "temperature too high",
			body: &api.ChatSettings{Temperature: &highTemp},
			ok:   false,
		},
		{
			name: "topP too high",
			body: &api.ChatSettings{TopP: &highTopP},
			ok:   false,
		},
		{
			name: "maxOutputTokens not positive",
			body: &api.ChatSettings{MaxOutputTokens: &zeroTokens},
			ok:   false,
		},
		{
			name: "maxHistoryTokens not positive",
			body: &api.ChatSettings{MaxHistoryTokens: &zeroTokens},
			ok:   false,
		},
		{
			name: "unknown reasoning effort",
			body: &api.ChatSettings{ReasoningEffort: &badEffort},
			ok:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			msg, ok := validateChatSettings(tc.body)
			assert.Equal(t, tc.ok, ok)

			if !tc.ok {
				assert.NotEmpty(t, msg)
			}
		})
	}
}
