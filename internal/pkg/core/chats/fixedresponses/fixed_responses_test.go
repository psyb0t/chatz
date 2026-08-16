package fixedresponses

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetShowcase(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		prompt   string
		wantOK   bool
		contains string
	}{
		{
			name:     "operations prompt matches exactly",
			prompt:   ShowcasePromptOperations,
			wantOK:   true,
			contains: "REQUEST VOLUME",
		},
		{
			name:     "sales prompt",
			prompt:   ShowcasePromptSales,
			wantOK:   true,
			contains: "PIPELINE CONVERSION",
		},
		{
			name:     "customer prompt",
			prompt:   ShowcasePromptCustomers,
			wantOK:   true,
			contains: "AT-RISK ACCOUNTS",
		},
		{
			name:   "different casing does not match",
			prompt: strings.ToUpper(ShowcasePromptOperations),
		},
		{
			name:   "trailing whitespace does not match",
			prompt: ShowcasePromptOperations + " ",
		},
		{
			name:   "unknown prompt does not match",
			prompt: "Please call whatever model is configured.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := GetShowcase(tc.prompt)
			require.Equal(t, tc.wantOK, ok)

			if !tc.wantOK {
				assert.Equal(t, Response{}, got)

				return
			}

			assert.Equal(t, KindTools, got.Kind)
			assert.True(t, got.Persist)
			assert.Contains(t, got.AnswerText(), tc.contains)
			assertShowcaseSteps(t, got.Steps)
		})
	}
}

func TestShowcaseResponses_PointAtEmbeddedFiles(t *testing.T) {
	t.Parallel()

	for prompt := range showcaseResponses {
		got, ok := GetShowcase(prompt)
		require.True(t, ok)
		assert.True(t, got.Persist)
		assert.Contains(t, got.AnswerText(), "```spec")
		assertShowcaseSteps(t, got.Steps)
	}
}

func TestShowcaseResponses_EvidenceAndPacing(t *testing.T) {
	t.Parallel()

	for prompt, showcase := range showcaseResponses {
		got, ok := GetShowcase(prompt)
		require.True(t, ok, "showcase prompt %q must resolve", prompt)
		assert.Positive(t, got.InitialDelay)
		assert.Positive(t, got.TextChunkDelay)

		toolResults := make([]string, 0, len(got.Steps))

		for _, step := range got.Steps {
			if step.Kind != StepTool {
				continue
			}

			require.NotNil(t, step.Tool)
			assert.Positive(t, step.DelayBefore)
			assert.Positive(t, step.Tool.ResultDelay)
			toolResults = append(toolResults, step.Tool.ResultText)
		}

		joinedToolResults := strings.Join(toolResults, "\n")

		for _, evidence := range showcase.evidence {
			assert.Contains(t, joinedToolResults, evidence.toolResult)
			assert.Contains(t, got.AnswerText(), evidence.answer)
		}
	}
}

func assertShowcaseSteps(t *testing.T, steps []Step) {
	t.Helper()
	require.GreaterOrEqual(t, len(steps), 4)
	assert.Equal(t, StepThinking, steps[0].Kind)
	assert.Equal(t, StepTool, steps[1].Kind)
	assert.Equal(t, StepTool, steps[2].Kind)
	assert.Equal(t, StepText, steps[len(steps)-1].Kind)

	seen := map[string]struct{}{}

	for _, step := range steps[1:3] {
		require.NotNil(t, step.Tool)
		assert.NotEmpty(t, step.Tool.Name)
		assert.NotEmpty(t, step.Tool.ArgsJSON)
		assert.NotEmpty(t, step.Tool.ResultText)
		_, exists := seen[step.Tool.ToolUseID]
		assert.False(t, exists, "duplicate tool-use ID %q", step.Tool.ToolUseID)
		seen[step.Tool.ToolUseID] = struct{}{}
	}
}
