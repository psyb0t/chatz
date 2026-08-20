//go:build api

package api

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	smokeThemeToggleSel = "[data-testid=theme-toggle]"
	smokeThemeLight     = "light"
	smokeThemeDark      = "dark"

	smokeShowcasePrompt = "Where is our cloud spend going this " +
		"quarter and are we on budget?"

	smokeToolCardSel            = "[data-testid=tool-card]"
	smokeToolCardHeadSel        = "[data-testid=tool-card] .tool-card__head"
	smokeToolCardWaitSec        = 30
	smokeSettleAfterTool        = 1500
	smokeSendSel                = "[data-testid=send]"
	smokeStreamDonePollMs       = 100
	smokeStreamDonePollAttempts = 450

	smokeSettingsToggleSel  = "[data-testid=chat-settings-toggle]"
	smokeSettingsWaitSec    = 10
	smokeSettleAfterGear    = 300
	smokePresetSelectSel    = "[data-testid=chat-settings-preset]"
	smokePresetNameSel      = "[data-testid=chat-settings-preset-name]"
	smokePresetSaveSel      = "[data-testid=chat-settings-preset-save]"
	smokePresetDeleteSel    = "[data-testid=chat-settings-preset-delete]"
	smokePrecisePresetID    = "built-in-precise"
	smokeSavedPresetName    = "Incident review"
	smokePresetExpectedTemp = "0.2"
	smokePresetSettleMS     = 300

	smokeContextMeterSel     = "[data-testid=context-meter]"
	smokeContextMeterWaitSec = 10

	smokeModelPickerSel       = "[data-testid=model-picker]"
	smokeModelPickerSearchSel = "[data-testid=model-picker-search]"
	smokeModelPickerWaitSec   = 5
	smokeModelFilterQuery     = "mini"
	smokeSettleAfterFilter    = 200

	smokeConsoleErrorEvent = "sse.error"
)

// smokeThemeJS returns the current theme attribute as a JSON string.
const smokeThemeJS = `
JSON.stringify(document.documentElement.getAttribute('data-theme'))
`

const smokeContextMeterJS = `
(() => {
  const meter = document.querySelector('[data-testid=context-meter]');
  return JSON.stringify(meter ? meter.textContent.trim() : '');
})()
`

// smokeToolCardStateJS reports whether the tool card is present and whether
// its expandable body is currently in the DOM (collapsed cards omit it).
const smokeToolCardCollapsedJS = `
(() => {
  const card = document.querySelector('[data-testid=tool-card]');
  const body = card ? card.querySelector('.tool-card__body') : null;
  return JSON.stringify({
    found: !!card,
    bodyVisibleBeforeExpand: !!body,
  });
})()
`

const smokeToolCardExpandedJS = `
(() => {
  const card = document.querySelector('[data-testid=tool-card]');
  const body = card ? card.querySelector('.tool-card__body') : null;
  return JSON.stringify({ bodyVisibleAfterExpand: !!body });
})()
`

// smokeSettingsAnchorJS reports whether the settings popover rendered and
// whether it sits horizontally close to the gear that opened it.
const smokeSettingsAnchorJS = `
(() => {
  const gear = document.querySelector(
    '[data-testid=chat-settings-toggle]'
  );
  const popover = document.querySelector(
    '.composer__settings-anchor > div:not(.composer__gear)'
  ) || gear.parentElement.querySelector(':scope > *:not(button)');
  const g = gear.getBoundingClientRect();
  const p = popover ? popover.getBoundingClientRect() : null;
  return JSON.stringify({
    popoverPresent: !!p,
    horizontalOverlapOrClose: p ? Math.abs(p.left - g.left) < 400 : false,
  });
})()
`

const smokeApplyPrecisePresetJS = `(() => {
	const select = document.querySelector('[data-testid=chat-settings-preset]');
	select.value = 'built-in-precise';
	select.dispatchEvent(new Event('change', {bubbles: true}));
})()`

const smokePresetProbeJS = `(() => {
	const select = document.querySelector('[data-testid=chat-settings-preset]');
	const temperature = document.querySelector('input[type=number][max="2"]');
	const settings = document.querySelector(
		'[data-testid=chat-settings-panel]'
	);
	return JSON.stringify({
		selectedId: select?.value ?? '',
		temperature: temperature?.value ?? '',
		deleteVisible: !!document.querySelector(
			'[data-testid=chat-settings-preset-delete]'
		),
		noHorizontalOverflow:
			!settings || settings.scrollWidth <= settings.clientWidth,
	});
})()`

// smokeModelOptionsJS returns the visible option labels in the model picker.
const smokeModelOptionsJS = `
(() => {
  return JSON.stringify(
    Array.from(document.querySelectorAll('.picker__option'))
      .map(o => o.textContent.trim())
  );
})()
`

// smokeModelPickerEnabledJS reports whether the model-picker button accepts
// interaction. It is disabled for the whole streamed turn, even after the
// tool card arrives.
const smokeModelPickerEnabledJS = `
(() => {
  const picker = document.querySelector('[data-testid=model-picker]');
  return JSON.stringify(
    picker instanceof HTMLButtonElement && !picker.disabled
  );
})()
`

type smokeToolCardCollapsed struct {
	Found                   bool `json:"found"`
	BodyVisibleBeforeExpand bool `json:"bodyVisibleBeforeExpand"`
}

type smokeToolCardExpanded struct {
	BodyVisibleAfterExpand bool `json:"bodyVisibleAfterExpand"`
}

type smokeSettingsAnchor struct {
	PopoverPresent           bool `json:"popoverPresent"`
	HorizontalOverlapOrClose bool `json:"horizontalOverlapOrClose"`
}

type smokePresetProbe struct {
	SelectedID           string `json:"selectedId"`
	Temperature          string `json:"temperature"`
	DeleteVisible        bool   `json:"deleteVisible"`
	NoHorizontalOverflow bool   `json:"noHorizontalOverflow"`
}

func TestSmoke(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	stack, client := newStack(
		t,
		testinfra.APIOptions{ShowcaseMode: true},
		testinfra.BrowserOptions{},
	)

	setupAdmin(ctx, t, stack, client)

	var themeBefore string
	require.NoError(t, client.EvalJSON(ctx, smokeThemeJS, &themeBefore))
	require.Contains(t, []string{smokeThemeLight, smokeThemeDark}, themeBefore)
	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), "theme-before.png"))

	require.NoError(t, client.Eval(ctx, clickJS(smokeThemeToggleSel)))

	var themeAfter string
	require.NoError(t, client.EvalJSON(ctx, smokeThemeJS, &themeAfter))
	assert.NotEqual(t, themeBefore, themeAfter)
	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), "theme-after.png"))

	require.NoError(t,
		client.Fill(ctx, composerInput, smokeShowcasePrompt))
	require.NoError(t, client.WaitForElement(
		ctx, smokeContextMeterSel, stateVisible, smokeContextMeterWaitSec,
	))

	var contextMeter string
	require.NoError(t,
		client.EvalJSON(ctx, smokeContextMeterJS, &contextMeter))
	assert.Contains(t, contextMeter, "tokens")
	assert.Contains(t, contextMeter, "system")
	assert.Contains(t, contextMeter, "draft")

	require.NoError(t, client.Eval(ctx, submitJS(smokeSendSel)))
	require.NoError(t, client.WaitForElement(
		ctx, smokeToolCardSel, stateVisible, smokeToolCardWaitSec,
	))
	require.NoError(t, client.Eval(ctx, settleJS(smokeSettleAfterTool)))
	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), "tool-card.png"))

	var collapsed smokeToolCardCollapsed
	require.NoError(t,
		client.EvalJSON(ctx, smokeToolCardCollapsedJS, &collapsed))
	assert.True(t, collapsed.Found)
	assert.False(t, collapsed.BodyVisibleBeforeExpand)

	require.NoError(t, client.Eval(ctx, clickJS(smokeToolCardHeadSel)))

	var expanded smokeToolCardExpanded
	require.NoError(t,
		client.EvalJSON(ctx, smokeToolCardExpandedJS, &expanded))
	assert.True(t, expanded.BodyVisibleAfterExpand)
	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), "tool-card-open.png"))

	// The tool card arrives before the streamed response completes. Wait for
	// the control this test will use, not merely for an incidental DOM change.
	modelPickerEnabled := false
	for attempt := 0; attempt < smokeStreamDonePollAttempts; attempt++ {
		require.NoError(t, client.EvalJSON(
			ctx, smokeModelPickerEnabledJS, &modelPickerEnabled,
		))

		if modelPickerEnabled {
			break
		}

		if attempt+1 < smokeStreamDonePollAttempts {
			require.NoError(t,
				client.Eval(ctx, settleJS(smokeStreamDonePollMs)))
		}
	}

	require.True(t, modelPickerEnabled, "model picker never became enabled")

	require.NoError(t, client.WaitForElement(
		ctx, smokeSettingsToggleSel, stateVisible, smokeSettingsWaitSec,
	))
	require.NoError(t, client.Eval(ctx, clickJS(smokeSettingsToggleSel)))
	require.NoError(t, client.Eval(ctx, settleJS(smokeSettleAfterGear)))

	var anchor smokeSettingsAnchor
	require.NoError(t,
		client.EvalJSON(ctx, smokeSettingsAnchorJS, &anchor))
	assert.True(t, anchor.PopoverPresent)
	assert.True(t, anchor.HorizontalOverlapOrClose)
	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), "settings.png"))

	require.NoError(t, client.Eval(ctx, smokeApplyPrecisePresetJS))
	require.NoError(t, client.Eval(ctx, settleJS(smokePresetSettleMS)))

	var precisePreset smokePresetProbe
	require.NoError(t,
		client.EvalJSON(ctx, smokePresetProbeJS, &precisePreset))
	assert.Equal(t, smokePrecisePresetID, precisePreset.SelectedID)
	assert.Equal(t, smokePresetExpectedTemp, precisePreset.Temperature)
	assert.False(t, precisePreset.DeleteVisible)
	assert.True(t, precisePreset.NoHorizontalOverflow)

	require.NoError(t,
		client.Fill(ctx, smokePresetNameSel, smokeSavedPresetName))
	require.NoError(t, client.Eval(ctx, clickJS(smokePresetSaveSel)))
	require.NoError(t, client.Eval(ctx, settleJS(smokePresetSettleMS)))

	var savedPreset smokePresetProbe
	require.NoError(t,
		client.EvalJSON(ctx, smokePresetProbeJS, &savedPreset))
	assert.Contains(t, savedPreset.SelectedID, "saved-")
	assert.True(t, savedPreset.DeleteVisible)
	assert.True(t, savedPreset.NoHorizontalOverflow)

	require.NoError(t, client.Eval(ctx, clickJS(smokePresetDeleteSel)))
	require.NoError(t, client.Eval(ctx, settleJS(smokePresetSettleMS)))

	var deletedPreset smokePresetProbe
	require.NoError(t,
		client.EvalJSON(ctx, smokePresetProbeJS, &deletedPreset))
	assert.Empty(t, deletedPreset.SelectedID)
	assert.False(t, deletedPreset.DeleteVisible)
	assert.True(t, deletedPreset.NoHorizontalOverflow)

	require.NoError(t, client.Eval(ctx, clickJS(smokeModelPickerSel)))
	require.NoError(t, client.WaitForElement(
		ctx,
		smokeModelPickerSearchSel,
		stateVisible,
		smokeModelPickerWaitSec,
	))

	var optionsBefore []string
	require.NoError(t,
		client.EvalJSON(ctx, smokeModelOptionsJS, &optionsBefore))
	assert.NotEmpty(t, optionsBefore)

	require.NoError(t, client.Fill(
		ctx, smokeModelPickerSearchSel, smokeModelFilterQuery,
	))
	require.NoError(t, client.Eval(ctx, settleJS(smokeSettleAfterFilter)))

	var optionsAfter []string
	require.NoError(t,
		client.EvalJSON(ctx, smokeModelOptionsJS, &optionsAfter))
	assert.NotEmpty(t, optionsAfter)
	assert.LessOrEqual(t, len(optionsAfter), len(optionsBefore))

	for _, option := range optionsAfter {
		assert.Contains(t,
			strings.ToLower(option), smokeModelFilterQuery)
	}

	events := consoleEvents(ctx, t, client)
	assert.False(t, events[smokeConsoleErrorEvent])
	_ = client.Screenshot(ctx, filepath.Join(t.TempDir(), "final.png"))
}
