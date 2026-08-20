package server

import (
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
	highEffort := api.ChatSettingsReasoningEffortHigh

	highTemp := 2.5
	highTopP := 1.5
	zeroTokens := 0
	badEffort := api.ChatSettingsReasoningEffort("turbo")

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
