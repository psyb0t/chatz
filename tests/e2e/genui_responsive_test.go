//go:build e2e

package e2e

import (
	"path/filepath"
	"testing"

	"github.com/psyb0t/chatz/tests/testinfra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	genUIResponsiveSpecSel     = "[data-testid=spec-block]"
	genUIResponsiveWaitSec     = 120
	genUIResponsiveScreenshot  = "genui_responsive_mobile.png"
	genUIResponsiveGridColumns = 1
)

const genUIResponsiveProbeJS = `
(() => {
  const GEOMETRY_SLACK = 1;
  const spec = document.querySelector('[data-testid=spec-block]');
  const specRect = spec?.getBoundingClientRect();
  const components = [...document.querySelectorAll('[data-jr-type]')];
  const outside = components.filter((component) => {
    if (!specRect) {
      return true;
    }

    const rect = component.getBoundingClientRect();
    return rect.left < specRect.left - GEOMETRY_SLACK ||
      rect.right > specRect.right + GEOMETRY_SLACK;
  });
  const grids = [...document.querySelectorAll('.jr-grid')];
  const gridColumns = grids.map((grid) => {
    const columns = getComputedStyle(grid).gridTemplateColumns;
    return columns.trim().split(/\s+/).filter(Boolean).length;
  });
  const logViewports = [
    ...document.querySelectorAll('.jr-logs__viewport'),
  ];
  const logsBounded = logViewports.every(
    (viewport) => viewport.getBoundingClientRect().right <=
      window.innerWidth + GEOMETRY_SLACK,
  );

  return JSON.stringify({
    documentBounded: document.documentElement.scrollWidth <= window.innerWidth,
    componentCount: components.length,
    outsideCount: outside.length,
    gridColumns,
    logsBounded,
  });
})()
`

type genUIResponsiveProbe struct {
	DocumentBounded bool  `json:"documentBounded"`
	ComponentCount  int   `json:"componentCount"`
	OutsideCount    int   `json:"outsideCount"`
	GridColumns     []int `json:"gridColumns"`
	LogsBounded     bool  `json:"logsBounded"`
}

// TestGenUIResponsiveDashboard verifies a real streamed dashboard at phone
// width: all components stay in their assistant block and model-requested
// multi-column grids reflow to one readable column.
func TestGenUIResponsiveDashboard(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	stack, client := newStack(
		t,
		testinfra.E2EOptions{ShowcaseMode: true},
		testinfra.BrowserOptions{Viewport: mobileViewport},
	)
	setupAdmin(ctx, t, stack, client)

	require.NoError(t, client.Fill(ctx, composerInput, showcasePrompt))
	require.NoError(t, client.Eval(ctx, submitJS(showcaseSendSel)))
	require.NoError(t, client.WaitForElement(
		ctx,
		genUIResponsiveSpecSel,
		stateVisible,
		genUIResponsiveWaitSec,
	))
	require.NoError(t, client.WaitForElement(
		ctx,
		showcaseStopSel,
		stateHidden,
		genUIResponsiveWaitSec,
	))

	var probe genUIResponsiveProbe
	require.NoError(t, client.EvalJSON(ctx, genUIResponsiveProbeJS, &probe))
	assert.True(t, probe.DocumentBounded)
	assert.Positive(t, probe.ComponentCount)
	assert.Zero(t, probe.OutsideCount)
	assert.NotEmpty(t, probe.GridColumns)

	for _, columns := range probe.GridColumns {
		assert.Equal(t, genUIResponsiveGridColumns, columns)
	}

	assert.True(t, probe.LogsBounded)

	_ = client.Screenshot(
		ctx,
		filepath.Join(t.TempDir(), genUIResponsiveScreenshot),
	)
}
