//go:build api

package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/psyb0t/chatz/internal/pkg/core/chats/fixedresponses"
	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	genUIVisualSpecSel        = "[data-testid=spec-block]"
	genUIVisualThemeToggleSel = "[data-testid=theme-toggle]"
	genUIVisualWaitSec        = 120

	genUIVisualDesktopViewport = "desktop"
	genUIVisualMobileViewport  = "mobile"
	genUIVisualThemeLight      = "light"
	genUIVisualThemeDark       = "dark"
	genUIVisualConsoleSSEError = "sse.error"

	genUIComponentText            = "Text"
	genUIComponentHeading         = "Heading"
	genUIComponentCard            = "Card"
	genUIComponentStack           = "Stack"
	genUIComponentGrid            = "Grid"
	genUIComponentStat            = "Stat"
	genUIComponentBadge           = "Badge"
	genUIComponentTable           = "Table"
	genUIComponentKeyValue        = "KeyValue"
	genUIComponentCallout         = "Callout"
	genUIComponentTimeline        = "Timeline"
	genUIComponentProgress        = "Progress"
	genUIComponentTimeSeriesChart = "TimeSeriesChart"
	genUIComponentAreaChart       = "AreaChart"
	genUIComponentSparkline       = "Sparkline"
	genUIComponentBarChart        = "BarChart"
	genUIComponentDonutChart      = "DonutChart"
	genUIComponentFunnelChart     = "FunnelChart"
	genUIComponentGauge           = "Gauge"
	genUIComponentScatterPlot     = "ScatterPlot"
	genUIComponentHeatmap         = "Heatmap"
	genUIComponentHistogram       = "Histogram"
	genUIComponentBoxPlot         = "BoxPlot"
	genUIComponentTreemap         = "Treemap"
	genUIComponentNetworkGraph    = "NetworkGraph"
	genUIComponentLogViewer       = "LogViewer"
)

func genUIVisualAllComponentTypes() []string {
	return []string{
		genUIComponentText,
		genUIComponentHeading,
		genUIComponentCard,
		genUIComponentStack,
		genUIComponentGrid,
		genUIComponentStat,
		genUIComponentBadge,
		genUIComponentTable,
		genUIComponentKeyValue,
		genUIComponentCallout,
		genUIComponentTimeline,
		genUIComponentProgress,
		genUIComponentTimeSeriesChart,
		genUIComponentAreaChart,
		genUIComponentSparkline,
		genUIComponentBarChart,
		genUIComponentDonutChart,
		genUIComponentFunnelChart,
		genUIComponentGauge,
		genUIComponentScatterPlot,
		genUIComponentHeatmap,
		genUIComponentHistogram,
		genUIComponentBoxPlot,
		genUIComponentTreemap,
		genUIComponentNetworkGraph,
		genUIComponentLogViewer,
	}
}

const genUIVisualThemeJS = `JSON.stringify(
	document.documentElement.getAttribute('data-theme')
)`

const genUIVisualProbeJS = `(() => {
	const GEOMETRY_SLACK = 1;
	const specs = [...document.querySelectorAll('[data-testid=spec-block]')];
	const spec = specs.at(-1);
	const specRect = spec?.getBoundingClientRect();
	const components = spec
		? [...spec.querySelectorAll('[data-jr-type]')]
		: [];
	const escapedComponents = components.filter((component) => {
		if (!specRect) {
			return true;
		}

		const rect = component.getBoundingClientRect();
		return rect.left < specRect.left - GEOMETRY_SLACK ||
			rect.right > specRect.right + GEOMETRY_SLACK;
	});
	const documentBounded =
		document.documentElement.scrollWidth <= window.innerWidth;

	return JSON.stringify({
		documentBounded,
		escapedComponents: escapedComponents.length,
		types: [...new Set(
			components.map((component) => component.dataset.jrType),
		)],
	});
})()`

type genUIVisualProbe struct {
	DocumentBounded   bool     `json:"documentBounded"`
	EscapedComponents int      `json:"escapedComponents"`
	Types             []string `json:"types"`
}

type genUIVisualShowcase struct {
	name          string
	prompt        string
	expectedTypes []string
}

// TestGenUIVisualMatrix captures every registered renderer through the same
// fixed-response SSE path users see, at both breakpoints and colour themes.
func TestGenUIVisualMatrix(t *testing.T) {
	t.Parallel()

	showcases := []genUIVisualShowcase{
		{
			name:   "operations",
			prompt: fixedresponses.ShowcasePromptOperations,
			expectedTypes: []string{
				genUIComponentStack, genUIComponentGrid, genUIComponentStat,
				genUIComponentCard, genUIComponentHeading, genUIComponentText,
				genUIComponentKeyValue, genUIComponentCallout,
				genUIComponentProgress, genUIComponentBadge,
				genUIComponentTimeSeriesChart, genUIComponentBarChart,
				genUIComponentTable, genUIComponentLogViewer,
			},
		},
		{
			name:   "sales",
			prompt: fixedresponses.ShowcasePromptSales,
			expectedTypes: []string{
				genUIComponentStack, genUIComponentGrid, genUIComponentStat,
				genUIComponentFunnelChart, genUIComponentBarChart,
				genUIComponentSparkline, genUIComponentTable,
			},
		},
		{
			name:   "customers",
			prompt: fixedresponses.ShowcasePromptCustomers,
			expectedTypes: []string{
				genUIComponentStack, genUIComponentGrid, genUIComponentStat,
				genUIComponentAreaChart, genUIComponentGauge,
				genUIComponentDonutChart, genUIComponentSparkline,
				genUIComponentTimeline, genUIComponentTable,
			},
		},
		{
			name:   "infrastructure",
			prompt: fixedresponses.ShowcasePromptInfrastructure,
			expectedTypes: []string{
				genUIComponentStack, genUIComponentGrid, genUIComponentStat,
				genUIComponentAreaChart, genUIComponentHeatmap,
				genUIComponentGauge, genUIComponentTable,
			},
		},
		{
			name:   "experiment",
			prompt: fixedresponses.ShowcasePromptExperiment,
			expectedTypes: []string{
				genUIComponentStack, genUIComponentGrid, genUIComponentStat,
				genUIComponentHistogram, genUIComponentBoxPlot,
				genUIComponentScatterPlot,
			},
		},
		{
			name:   "finance",
			prompt: fixedresponses.ShowcasePromptFinance,
			expectedTypes: []string{
				genUIComponentStack, genUIComponentGrid, genUIComponentStat,
				genUIComponentTreemap, genUIComponentDonutChart,
				genUIComponentSparkline,
			},
		},
		{
			name:   "reliability",
			prompt: fixedresponses.ShowcasePromptReliability,
			expectedTypes: []string{
				genUIComponentStack, genUIComponentGrid, genUIComponentStat,
				genUIComponentNetworkGraph, genUIComponentTable,
				genUIComponentLogViewer,
			},
		},
	}

	testCases := []struct {
		name          string
		viewport      string
		browserOption testinfra.BrowserOptions
	}{
		{
			name:     genUIVisualDesktopViewport,
			viewport: genUIVisualDesktopViewport,
		},
		{
			name:     genUIVisualMobileViewport,
			viewport: genUIVisualMobileViewport,
			browserOption: testinfra.BrowserOptions{
				Viewport: mobileViewport,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runGenUIVisualViewport(
				t,
				tc.viewport,
				tc.browserOption,
				showcases,
			)
		})
	}
}

func runGenUIVisualViewport(
	t *testing.T,
	viewport string,
	browserOption testinfra.BrowserOptions,
	showcases []genUIVisualShowcase,
) {
	t.Helper()

	ctx := t.Context()
	stack, client := newStack(
		t,
		testinfra.APIOptions{ShowcaseMode: true},
		browserOption,
	)
	setupAdmin(ctx, t, stack, client)

	seenTypeSet := map[string]struct{}{}
	allComponentTypes := genUIVisualAllComponentTypes()
	seenTypes := make([]string, 0, len(allComponentTypes))

	for _, showcase := range showcases {
		require.NoError(t, client.Fill(ctx, composerInput, showcase.prompt))
		require.NoError(t, client.Eval(ctx, submitJS(showcaseSendSel)))
		require.NoError(t, client.WaitForElement(
			ctx,
			genUIVisualSpecSel,
			stateVisible,
			genUIVisualWaitSec,
		))
		require.NoError(t, client.WaitForElement(
			ctx,
			showcaseStopSel,
			stateHidden,
			genUIVisualWaitSec,
		))

		for _, componentType := range assertVisualDashboard(
			ctx,
			t,
			client,
			showcase.expectedTypes,
		) {
			if _, seen := seenTypeSet[componentType]; seen {
				continue
			}

			seenTypeSet[componentType] = struct{}{}
			seenTypes = append(seenTypes, componentType)
		}

		var initialTheme string
		require.NoError(t,
			client.EvalJSON(ctx, genUIVisualThemeJS, &initialTheme))
		assert.Contains(t, []string{
			genUIVisualThemeLight,
			genUIVisualThemeDark,
		}, initialTheme)
		require.NoError(t, client.Screenshot(
			ctx,
			filepath.Join(
				t.TempDir(),
				viewport+"_"+showcase.name+"_"+initialTheme+".png",
			),
		))

		require.NoError(t,
			client.Eval(ctx, clickJS(genUIVisualThemeToggleSel)))

		var toggledTheme string
		require.NoError(t,
			client.EvalJSON(ctx, genUIVisualThemeJS, &toggledTheme))
		assert.NotEqual(t, initialTheme, toggledTheme)
		assert.Contains(t, []string{
			genUIVisualThemeLight,
			genUIVisualThemeDark,
		}, toggledTheme)
		assertVisualDashboard(ctx, t, client, showcase.expectedTypes)
		require.NoError(t, client.Screenshot(
			ctx,
			filepath.Join(
				t.TempDir(),
				viewport+"_"+showcase.name+"_"+toggledTheme+".png",
			),
		))

		require.NoError(t, client.Goto(ctx, stack.AppURL, networkIdle))
		require.NoError(t, client.WaitForElement(
			ctx,
			composerInput,
			stateVisible,
			genUIVisualWaitSec,
		))
	}

	assert.ElementsMatch(t, allComponentTypes, seenTypes)

	events := consoleEvents(ctx, t, client)
	assert.False(t, events[genUIVisualConsoleSSEError])
}

func assertVisualDashboard(
	ctx context.Context,
	t *testing.T,
	client *testinfra.BrowserClient,
	expectedTypes []string,
) []string {
	t.Helper()

	var probe genUIVisualProbe
	require.NoError(t, client.EvalJSON(ctx, genUIVisualProbeJS, &probe))
	assert.True(t, probe.DocumentBounded)
	assert.Zero(t, probe.EscapedComponents)
	assert.ElementsMatch(t, expectedTypes, probe.Types)

	return probe.Types
}
