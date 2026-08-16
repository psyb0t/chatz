package chats

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

// TestMessageToAPI is the only direct unit coverage of the row->wire projection
// reload depends on — the integration test only ever exercises it through ONE
// scenario (a single tool call). Multiple tool calls in one row (executeTools
// runs calls in parallel, so this is a common real shape) and the
// malformed-JSON error path have no other coverage.
func TestMessageToAPI(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.New()

	testCases := []struct {
		name      string
		in        *models.Message
		wantErr   bool
		wantThink *string
		// wantCalls entries render as "id:name:arguments" for easy comparison.
		wantCalls   *[]string
		wantToolID  *string
		wantIsErr   *bool
		wantPartial *bool
	}{
		{
			name: "plain text row has no optional fields set",
			in: &models.Message{
				Base:    models.Base{ID: id, CreatedAt: fixedTime},
				Role:    models.MessageRoleUser,
				Content: "hi there",
			},
		},
		{
			name: "incomplete assistant checkpoint is marked",
			in: &models.Message{
				Base:         models.Base{ID: id, CreatedAt: fixedTime},
				Role:         models.MessageRoleAssistant,
				Content:      "partial answer",
				TurnComplete: new(false),
			},
			wantPartial: new(true),
		},
		{
			name: "reasoning-only round (no text, no tool calls)",
			in: &models.Message{
				Base:      models.Base{ID: id, CreatedAt: fixedTime},
				Role:      models.MessageRoleAssistant,
				Content:   "",
				Reasoning: "thinking it over",
			},
			wantThink: new("thinking it over"),
		},
		{
			name: "single tool call",
			in: &models.Message{
				Base:    models.Base{ID: id, CreatedAt: fixedTime},
				Role:    models.MessageRoleAssistant,
				Content: "",
				ToolCalls: datatypes.JSON(
					[]byte(`[{"id":"c1","name":"search","arguments":"{}"}]`),
				),
			},
			wantCalls: &[]string{"c1:search:{}"},
		},
		{
			name: "multiple parallel tool calls preserve order",
			in: &models.Message{
				Base:    models.Base{ID: id, CreatedAt: fixedTime},
				Role:    models.MessageRoleAssistant,
				Content: "",
				ToolCalls: datatypes.JSON([]byte(
					`[{"id":"c1","name":"weather",` +
						`"arguments":"{\"city\":\"a\"}"},` +
						`{"id":"c2","name":"weather",` +
						`"arguments":"{\"city\":\"b\"}"}]`,
				)),
			},
			wantCalls: &[]string{
				`c1:weather:{"city":"a"}`,
				`c2:weather:{"city":"b"}`,
			},
		},
		{
			name: "tool result row with error",
			in: &models.Message{
				Base:       models.Base{ID: id, CreatedAt: fixedTime},
				Role:       models.MessageRoleTool,
				Content:    "timed out",
				ToolCallID: "c1",
				IsError:    true,
			},
			wantToolID: new("c1"),
			wantIsErr:  new(true),
		},
		{
			name: "tool result row without error still sets isError=false",
			in: &models.Message{
				Base:       models.Base{ID: id, CreatedAt: fixedTime},
				Role:       models.MessageRoleTool,
				Content:    "ok",
				ToolCallID: "c1",
				IsError:    false,
			},
			wantToolID: new("c1"),
			wantIsErr:  new(false),
		},
		{
			name: "nil tool calls column stays nil, not an empty slice",
			in: &models.Message{
				Base:    models.Base{ID: id, CreatedAt: fixedTime},
				Role:    models.MessageRoleAssistant,
				Content: "hello",
			},
		},
		{
			name: "malformed tool_calls JSON returns an error, not a panic",
			in: &models.Message{
				Base:      models.Base{ID: id, CreatedAt: fixedTime},
				Role:      models.MessageRoleAssistant,
				ToolCalls: datatypes.JSON([]byte(`not json`)),
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := MessageToAPI(tc.in)
			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)

			assert.Equal(t, tc.in.ID, got.Id)
			assert.Equal(t, tc.in.Content, got.Content)
			assert.Equal(t, tc.wantThink, got.Thinking)
			assert.Equal(t, tc.wantToolID, got.ToolCallId)
			assert.Equal(t, tc.wantIsErr, got.IsError)
			assert.Equal(t, tc.wantPartial, got.Incomplete)

			if tc.wantCalls == nil {
				assert.Nil(t, got.ToolCalls)

				return
			}

			require.NotNil(t, got.ToolCalls)

			gotCalls := make([]string, len(*got.ToolCalls))
			for i, c := range *got.ToolCalls {
				gotCalls[i] = c.Id + ":" + c.Name + ":" + c.Arguments
			}

			assert.Equal(t, *tc.wantCalls, gotCalls)
		})
	}
}
