<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_SCATTER_PLOT, DATA_JR_TYPE } from "$lib/common/render";
  import {
    CHART_MARGIN_BOTTOM,
    CHART_MARGIN_LEFT,
    CHART_MARGIN_RIGHT,
    CHART_MARGIN_TOP,
    CHART_VIEWBOX_HEIGHT,
    CHART_VIEWBOX_WIDTH,
    chartHeight,
    extent,
    finiteNumber,
    formatChartNumber,
    scaleLinear,
    ticks,
  } from "$lib/render/charts/math";
  import { CHART_GRID_COLOR, chartColor } from "$lib/render/charts/palette";
  import type { XYSeries } from "$lib/render/charts/types";

  type ScatterPlotProps = {
    id?: string | null;
    title?: string | null;
    xLabel?: string | null;
    yLabel?: string | null;
    height?: number | null;
    series?: XYSeries[] | null;
  };

  interface PlottedPoint {
    x: number;
    y: number;
    label: string;
  }

  interface PlottedSeries {
    name: string;
    points: PlottedPoint[];
  }

  const MAX_SERIES = 12;
  const MAX_POINTS_PER_SERIES = 500;
  const PLOT_LEFT = CHART_MARGIN_LEFT;
  const PLOT_RIGHT = CHART_VIEWBOX_WIDTH - CHART_MARGIN_RIGHT;
  const PLOT_TOP = CHART_MARGIN_TOP;
  const PLOT_BOTTOM = CHART_VIEWBOX_HEIGHT - CHART_MARGIN_BOTTOM;

  const { props }: BaseComponentProps<ScatterPlotProps> = $props();

  const height = $derived(chartHeight(props.height));
  const series = $derived.by((): PlottedSeries[] => {
    if (!Array.isArray(props.series)) {
      return [];
    }

    return props.series
      .slice(0, MAX_SERIES)
      .flatMap((candidate, seriesIndex) => {
        if (!candidate || typeof candidate !== "object") {
          return [];
        }

        const rawPoints = Array.isArray(candidate.points)
          ? candidate.points
          : [];
        const points = rawPoints
          .slice(0, MAX_POINTS_PER_SERIES)
          .flatMap((point): PlottedPoint[] => {
            if (!point || typeof point !== "object") {
              return [];
            }

            const x = finiteNumber(point.x);
            const y = finiteNumber(point.y);
            if (x === null || y === null) {
              return [];
            }

            return [
              {
                x,
                y,
                label: typeof point.label === "string" ? point.label : "",
              },
            ];
          });

        if (points.length === 0) {
          return [];
        }

        return [
          {
            name:
              typeof candidate.name === "string" && candidate.name
                ? candidate.name
                : `Series ${seriesIndex + 1}`,
            points,
          },
        ];
      });
  });
  const allPoints = $derived(series.flatMap((item) => item.points));
  const xDomain = $derived(extent(allPoints.map((point) => point.x)));
  const yDomain = $derived(extent(allPoints.map((point) => point.y)));
  const xTicks = $derived(ticks(xDomain[0], xDomain[1]));
  const yTicks = $derived(ticks(yDomain[0], yDomain[1]));
  const description = $derived(
    `${series.length} series with ${allPoints.length} finite points. X values range from ${formatChartNumber(xDomain[0])} to ${formatChartNumber(xDomain[1])}; Y values range from ${formatChartNumber(yDomain[0])} to ${formatChartNumber(yDomain[1])}.`,
  );
</script>

<section
  class="jr-chart"
  id={props.id ?? undefined}
  style:height={`${height}px`}
  {...{ [DATA_JR_TYPE]: COMP_SCATTER_PLOT }}
>
  {#if props.title}
    <h3>{props.title}</h3>
  {/if}
  {#if allPoints.length === 0}
    <div class="jr-chart__empty">No scatter data</div>
  {:else}
    <svg
      viewBox={`0 0 ${CHART_VIEWBOX_WIDTH} ${CHART_VIEWBOX_HEIGHT}`}
      role="img"
      aria-label={props.title || "Scatter plot"}
    >
      <desc>{description}</desc>
      {#each xTicks as tick}
        {@const x = scaleLinear(
          tick,
          xDomain[0],
          xDomain[1],
          PLOT_LEFT,
          PLOT_RIGHT,
        )}
        <line
          x1={x}
          y1={PLOT_TOP}
          x2={x}
          y2={PLOT_BOTTOM}
          stroke={CHART_GRID_COLOR}
        >
          <title>X grid line at {formatChartNumber(tick)}</title>
        </line>
        <text {x} y={PLOT_BOTTOM + 20} text-anchor="middle"
          >{formatChartNumber(tick)}</text
        >
      {/each}
      {#each yTicks as tick}
        {@const y = scaleLinear(
          tick,
          yDomain[0],
          yDomain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        <line
          x1={PLOT_LEFT}
          y1={y}
          x2={PLOT_RIGHT}
          y2={y}
          stroke={CHART_GRID_COLOR}
        >
          <title>Y grid line at {formatChartNumber(tick)}</title>
        </line>
        <text x={PLOT_LEFT - 10} y={y + 4} text-anchor="end"
          >{formatChartNumber(tick)}</text
        >
      {/each}
      {#each series as item, seriesIndex}
        {#each item.points as point}
          {@const x = scaleLinear(
            point.x,
            xDomain[0],
            xDomain[1],
            PLOT_LEFT,
            PLOT_RIGHT,
          )}
          {@const y = scaleLinear(
            point.y,
            yDomain[0],
            yDomain[1],
            PLOT_BOTTOM,
            PLOT_TOP,
          )}
          <circle
            cx={x}
            cy={y}
            r="5"
            fill={chartColor(seriesIndex)}
            fill-opacity="0.82"
          >
            <title
              >{item.name}{point.label ? ` — ${point.label}` : ""}: {formatChartNumber(
                point.x,
              )}, {formatChartNumber(point.y)}</title
            >
          </circle>
        {/each}
      {/each}
      {#if props.xLabel}
        <text
          class="axis-label"
          x={(PLOT_LEFT + PLOT_RIGHT) / 2}
          y={CHART_VIEWBOX_HEIGHT - 5}
          text-anchor="middle">{props.xLabel}</text
        >
      {/if}
      {#if props.yLabel}
        <text
          class="axis-label"
          transform={`translate(16 ${(PLOT_TOP + PLOT_BOTTOM) / 2}) rotate(-90)`}
          text-anchor="middle">{props.yLabel}</text
        >
      {/if}
    </svg>
    <div class="jr-chart__legend" aria-label="Series legend">
      {#each series as item, index}
        <span><i style:background={chartColor(index)}></i>{item.name}</span>
      {/each}
    </div>
  {/if}
</section>

<style>
  .jr-chart {
    min-width: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    overflow: hidden;
  }

  h3 {
    font-size: var(--text-sm);
  }

  svg {
    width: 100%;
    min-height: 0;
    flex: 1;
  }

  text {
    fill: var(--muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }

  .axis-label {
    fill: var(--ink);
    font-family: var(--font-display);
    font-size: 12px;
  }

  .jr-chart__legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-4);
    color: var(--muted);
    font-size: var(--text-xs);
  }

  .jr-chart__legend span {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }

  .jr-chart__legend i {
    width: 0.65rem;
    height: 0.65rem;
    border-radius: 50%;
  }

  .jr-chart__empty {
    flex: 1;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-size: var(--text-sm);
  }
</style>
