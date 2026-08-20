package server

import (
	"context"
	"testing"

	"github.com/psyb0t/aichteeteapee"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
	"github.com/stretchr/testify/assert"
)

// Local aliases keep the table below within the line-length budget; the codes
// themselves are the client-facing switch keys defined in aichteeteapee.
const (
	codeBad    = aichteeteapee.ErrorCodeBadRequest
	codeNF     = aichteeteapee.ErrorCodeNotFound
	codeUnauth = aichteeteapee.ErrorCodeUnauthorized
	codeVal    = aichteeteapee.ErrorCodeValidationFailed
)

// TestErrorEnvelopeHelpers pins the transport-error builders: each wraps the
// shared envelope with a stable UPPER_SNAKE code the client switches on, plus a
// message. Message-taking builders pass their argument through verbatim; the
// fixed ones (wantMsg empty here) carry the right code and a non-empty message.
func TestErrorEnvelopeHelpers(t *testing.T) {
	t.Parallel()

	const msg = "boom"

	type errCase struct {
		name     string
		got      api.Error
		wantCode string
		wantMsg  string
	}

	var cases []errCase

	add := func(name string, got api.Error, wantCode, wantMsg string) {
		cases = append(cases, errCase{name, got, wantCode, wantMsg})
	}

	add(
		"setupBadRequest",
		api.Error(setupBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"createChatBadRequest",
		api.Error(createChatBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"continueChatBadRequest",
		api.Error(continueChatBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"renameChatBadRequest",
		api.Error(renameChatBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"updateChatSettingsBadRequest",
		api.Error(updateChatSettingsBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	valFailed := updateChatSettingsValidationFailed(msg)
	add(
		"updateChatSettingsValidationFailed",
		api.Error(valFailed.BadRequestJSONResponse),
		codeVal, msg,
	)
	add(
		"previewChatContextBadRequest",
		api.Error(previewChatContextBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"updateChatMCPServerBadRequest",
		api.Error(updateChatMCPServerBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"createMCPBadRequest",
		api.Error(createMCPBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"importMCPBadRequest",
		api.Error(importMCPBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"updateMCPBadRequest",
		api.Error(updateMCPBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"createUserBadRequest",
		api.Error(createUserBadRequest(msg).BadRequestJSONResponse),
		codeBad, msg,
	)
	add(
		"chatWorkflowNotFound",
		api.Error(chatWorkflowNotFound(msg)),
		codeNF, msg,
	)
	add(
		"loginUnauthorized",
		api.Error(loginUnauthorized().UnauthorizedJSONResponse),
		codeUnauth, "",
	)
	add(
		"continueChatNotFound",
		api.Error(continueChatNotFound().NotFoundJSONResponse),
		codeNF, "",
	)
	add(
		"renameChatNotFound",
		api.Error(renameChatNotFound().NotFoundJSONResponse),
		codeNF, "",
	)
	add(
		"updateChatSettingsNotFound",
		api.Error(updateChatSettingsNotFound().NotFoundJSONResponse),
		codeNF, "",
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.wantCode, tc.got.Code)

			if tc.wantMsg != "" {
				assert.Equal(t, tc.wantMsg, tc.got.Message)

				return
			}

			assert.NotEmpty(t, tc.got.Message)
		})
	}
}

// TestToChatSettings maps the wire settings shape to the core Settings, leaving
// unset optional pointers nil and turning the reasoning-effort enum into its
// string form only when present.
func TestToChatSettings(t *testing.T) {
	t.Parallel()

	temp := 0.5
	topP := 0.9
	maxOut := 1024
	maxHist := 50000
	effort := api.ChatSettingsReasoningEffort("high")

	t.Run("all fields including reasoning effort", func(t *testing.T) {
		t.Parallel()

		got := toChatSettings(&api.ChatSettings{
			Temperature:      &temp,
			TopP:             &topP,
			MaxOutputTokens:  &maxOut,
			MaxHistoryTokens: &maxHist,
			ReasoningEffort:  &effort,
		})
		assert.Equal(t, &temp, got.Temperature)
		assert.Equal(t, &topP, got.TopP)
		assert.Equal(t, &maxOut, got.MaxOutputTokens)
		assert.Equal(t, &maxHist, got.MaxHistoryTokens)
		assert.Equal(t, "high", got.ReasoningEffort)
	})

	t.Run("unset reasoning effort stays empty", func(t *testing.T) {
		t.Parallel()

		got := toChatSettings(&api.ChatSettings{})
		assert.Nil(t, got.Temperature)
		assert.Empty(t, got.ReasoningEffort)
	})
}

// TestTokenFromContext returns the session token the auth middleware stashed on
// the context, or empty string when none is present.
func TestTokenFromContext(t *testing.T) {
	t.Parallel()

	t.Run("present", func(t *testing.T) {
		t.Parallel()

		ctx := context.WithValue(
			context.Background(), tokenCtxKey{}, "sess-123",
		)
		assert.Equal(t, "sess-123", tokenFromContext(ctx))
	})

	t.Run("absent", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, tokenFromContext(context.Background()))
	})
}
