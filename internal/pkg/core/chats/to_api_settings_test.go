package chats

import (
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChatSettingsToAPI covers both projection branches: unset generation
// pointers stay nil, and an empty ReasoningEffort maps to a nil pointer (the
// wire omits it) while a set one becomes the typed enum pointer.
func TestChatSettingsToAPI(t *testing.T) {
	t.Parallel()

	temp := 0.7
	maxOut := 2048

	t.Run("empty reasoning effort omits the field", func(t *testing.T) {
		t.Parallel()

		got := ChatSettingsToAPI(&models.Chat{
			Temperature:     &temp,
			MaxOutputTokens: &maxOut,
			ReasoningEffort: "",
		})
		require.NotNil(t, got)
		assert.Equal(t, &temp, got.Temperature)
		assert.Equal(t, &maxOut, got.MaxOutputTokens)
		assert.Nil(t, got.TopP)
		assert.Nil(t, got.ReasoningEffort)
	})

	t.Run("set reasoning effort maps to enum pointer", func(t *testing.T) {
		t.Parallel()

		got := ChatSettingsToAPI(&models.Chat{ReasoningEffort: "high"})
		require.NotNil(t, got.ReasoningEffort)
		assert.Equal(
			t,
			api.ChatSettingsReasoningEffort("high"),
			*got.ReasoningEffort,
		)
	})
}
