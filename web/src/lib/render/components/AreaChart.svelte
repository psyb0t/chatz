<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_AREA_CHART, DATA_JR_TYPE } from "$lib/common/render";
  import {
    CHART_MARGIN_BOTTOM,
    CHART_MARGIN_LEFT,
    CHART_MARGIN_RIGHT,
    CHART_MARGIN_TOP,
    CHART_VIEWBOX_HEIGHT,
    CHART_VIEWBOX_WIDTH,
    areaPath,
    chartHeight,
    extent,
    finiteNumber,
    formatChartNumber,
    scaleLinear,
    ticks,
    type NumericPoint,
  } from "$lib/render/charts/math";
  import {
    CHART_GRID_COLOR,
    CHART_PALETTE,
    chartColor,
  } from "$lib/render/charts/palette";
  import type { TimeSeries } from "$lib/render/charts/types";

  type AreaChartProps = {
    id?: string | null;
    title?: string | null;
    xLabel?: string | null;
    yLabel?: string | null;
    height?: number | null;
    series?: TimeSeries[] | null;
  };

  interface SafePoint {
    x: string;
    y: number;
  }

  interface SafeSeries {
    name: string;
    segments: SafePoint[][];
  }

  const MAX_SERIES = 12;
  const MAX_POINTS_PER_SERIES = 500;
  const MAX_AXIS_LABEL_LENGTH = 24;
  const MAX_SUMMARY_LABEL_LENGTH = 40;
  const MAX_SUMMARY_SERIES = 4;
  const REPEATED_COLOR_DASH = "8 4";
  const PRIMARY_FILL_OPACITY = 0.18;
  const REPEATED_COLOR_FILL_OPACITY = 0.08;
  const PLOT_LEFT = CHART_MARGIN_LEFT;
  const PLOT_RIGHT = CHART_VIEWBOX_WIDTH - CHART_MARGIN_RIGHT;
  const PLOT_TOP = CHART_MARGIN_TOP;
  const PLOT_BOTTOM = CHART_VIEWBOX_HEIGHT - CHART_MARGIN_BOTTOM;
  const X_TICK_COUNT = 3;

  const { props }: BaseComponentProps<AreaChartProps> = $props();

  function safeText(value: unknown): string {
    return typeof value === "string" ? value : "";
  }

  function shortLabel(value: string): string {
    return value.length <= MAX_AXIS_LABEL_LENGTH
      ? value
      : `${value.slice(0, MAX_AXIS_LABEL_LENGTH - 1)}…`;
  }

  function shortSummaryLabel(value: string): string {
    return value.length <= MAX_SUMMARY_LABEL_LENGTH
      ? value
      : `${value.slice(0, MAX_SUMMARY_LABEL_LENGTH - 1)}…`;
  }

  function seriesDash(index: number): string | undefined {
    return index >= CHART_PALETTE.length ? REPEATED_COLOR_DASH : undefined;
  }

  function seriesFillOpacity(index: number): number {
    return index >= CHART_PALETTE.length
      ? REPEATED_COLOR_FILL_OPACITY
      : PRIMARY_FILL_OPACITY;
  }

  function normalizeSeries(value: unknown): SafeSeries[] {
    if (!Array.isArray(value)) {
      return [];
    }

    return value.slice(0, MAX_SERIES).flatMap((candidate) => {
      if (typeof candidate !== "object" || candidate === null) {
        return [];
      }

      const record = candidate as Record<string, unknown>;
      if (!Array.isArray(record.points)) {
        return [];
      }

      const segments: SafePoint[][] = [];
      let segment: SafePoint[] = [];
      for (const candidatePoint of record.points.slice(
        0,
        MAX_POINTS_PER_SERIES,
      )) {
        if (typeof candidatePoint !== "object" || candidatePoint === null) {
          if (segment.length > 0) {
            segments.push(segment);
            segment = [];
          }
          continue;
        }

        const point = candidatePoint as Record<string, unknown>;
        const x = safeText(point.x);
        const y = finiteNumber(point.y);
        if (!x || y === null) {
          if (segment.length > 0) {
            segments.push(segment);
            segment = [];
          }
          continue;
        }

        segment.push({ x, y });
      }

      if (segment.length > 0) {
        segments.push(segment);
      }

      return segments.length > 0
        ? [{ name: safeText(record.name), segments }]
        : [];
    });
  }

  function allPoints(value: readonly SafeSeries[]): SafePoint[] {
    return value.flatMap((item) => item.segments.flat());
  }

  function xCoordinates(points: readonly SafePoint[]): Map<string, number> {
    const labels = [...new Set(points.map((point) => point.x))];
    const parsed = labels.map((label) => Date.parse(label));
    if (parsed.every(Number.isFinite)) {
      return new Map(labels.map((label, index) => [label, parsed[index]]));
    }

    return new Map(labels.map((label, index) => [label, index]));
  }

  function sampledXLabels(points: readonly SafePoint[]): string[] {
    const labels = [...new Set(points.map((point) => point.x))];
    if (labels.length <= X_TICK_COUNT) {
      return labels;
    }

    return [
      labels[0],
      labels[Math.floor((labels.length - 1) / 2)],
      labels.at(-1)!,
    ];
  }

  function summarizeSeries(item: SafeSeries, index: number): string {
    const itemPoints = item.segments.flat();
    const first = itemPoints[0];
    const last = itemPoints.at(-1)!;
    const name = shortSummaryLabel(item.name || `Series ${index + 1}`);
    return `${name} has ${itemPoints.length} points, from ${shortSummaryLabel(first.x)} at ${formatChartNumber(first.y)} to ${shortSummaryLabel(last.x)} at ${formatChartNumber(last.y)}`;
  }

  function summarizeChart(
    series: readonly SafeSeries[],
    allChartPoints: readonly SafePoint[],
  ): string {
    if (allChartPoints.length === 0) {
      return "No finite area-chart data is available.";
    }

    const values = allChartPoints.map((point) => point.y);
    const minimum = Math.min(...values);
    const maximum = Math.max(...values);
    const summaries = series
      .slice(0, MAX_SUMMARY_SERIES)
      .map(summarizeSeries)
      .join(". ");
    const omitted = series.length - MAX_SUMMARY_SERIES;
    const omittedSummary =
      omitted > 0 ? `. ${omitted} more series not summarized` : "";
    return `${series.length} series with ${allChartPoints.length} finite points. Values range from ${formatChartNumber(minimum)} to ${formatChartNumber(maximum)}. ${summaries}${omittedSummary}.`;
  }

  const rootID = $derived(typeof props.id === "string" ? props.id : undefined);
  const title = $derived(safeText(props.title));
  const xLabel = $derived(safeText(props.xLabel));
  const yLabel = $derived(safeText(props.yLabel));
  const height = $derived(chartHeight(props.height));
  const safeSeries = $derived(normalizeSeries(props.series));
  const points = $derived(allPoints(safeSeries));
  const xMap = $derived(xCoordinates(points));
  const xDomain = $derived(extent([...xMap.values()]));
  const yDomain = $derived(
    extent(
      points.map((point) => point.y),
      true,
    ),
  );
  const yTicks = $derived(ticks(yDomain[0], yDomain[1]));
  const xTicks = $derived(sampledXLabels(points));
  const summary = $derived(summarizeChart(safeSeries, points));
  const baseline = $derived(
    scaleLinear(0, yDomain[0], yDomain[1], PLOT_BOTTOM, PLOT_TOP),
  );
  const positionedSeries = $derived(
    safeSeries.map((item) => ({
      name: item.name,
      segments: item.segments.map((segment): NumericPoint[] =>
        segment.map((point) => ({
          x: scaleLinear(
            xMap.get(point.x) ?? xDomain[0],
            xDomain[0],
            xDomain[1],
            PLOT_LEFT,
            PLOT_RIGHT,
          ),
          y: scaleLinear(
            point.y,
            yDomain[0],
            yDomain[1],
            PLOT_BOTTOM,
            PLOT_TOP,
          ),
        })),
      ),
    })),
  );
</script>

<section
  class="jr-area"
  id={rootID}
  aria-label={title || "Area chart"}
  {...{ [DATA_JR_TYPE]: COMP_AREA_CHART }}
>
  {#if title}<h3>{title}</h3>{/if}
  <p class="jr-area__sr">{summary}</p>

  {#if points.length === 0}
    <div class="jr-area__empty" role="status">No finite area-chart data</div>
  {:else}
    <svg
      viewBox={`0 0 ${CHART_VIEWBOX_WIDTH} ${CHART_VIEWBOX_HEIGHT}`}
      {height}
      role="img"
      aria-label={title || "Area values"}
    >
      <title>{title || "Area values"}</title>

      {#each yTicks as tick (tick)}
        {@const y = scaleLinear(
          tick,
          yDomain[0],
          yDomain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        <line
          x1={PLOT_LEFT}
          x2={PLOT_RIGHT}
          y1={y}
          y2={y}
          stroke={CHART_GRID_COLOR}
          vector-effect="non-scaling-stroke"
        />
        <text x={PLOT_LEFT - 8} y={y + 4} text-anchor="end">
          {formatChartNumber(tick)}
        </text>
      {/each}

      {#each xTicks as tick, index (tick)}
        {@const x = scaleLinear(
          xMap.get(tick) ?? xDomain[0],
          xDomain[0],
          xDomain[1],
          PLOT_LEFT,
          PLOT_RIGHT,
        )}
        <text
          {x}
          y={PLOT_BOTTOM + 22}
          text-anchor={index === 0
            ? "start"
            : index === xTicks.length - 1
              ? "end"
              : "middle"}
        >
          <title>{tick}</title>
          {shortLabel(tick)}
        </text>
      {/each}

      {#each positionedSeries as item, seriesIndex (seriesIndex)}
        {#each item.segments as segment, segmentIndex (segmentIndex)}
          <path
            d={areaPath(segment, baseline)}
            fill={chartColor(seriesIndex)}
            fill-opacity={seriesFillOpacity(seriesIndex)}
            stroke={chartColor(seriesIndex)}
            stroke-width="2"
            stroke-linejoin="round"
            stroke-dasharray={seriesDash(seriesIndex)}
            vector-effect="non-scaling-stroke"
          >
            <title>{item.name || `Series ${seriesIndex + 1}`}</title>
          </path>
          {#each segment as point, pointIndex (pointIndex)}
            <circle
              cx={point.x}
              cy={point.y}
              r="2"
              fill={chartColor(seriesIndex)}
            >
              <title>
                {item.name || `Series ${seriesIndex + 1}`}: {safeSeries[
                  seriesIndex
                ].segments[segmentIndex][pointIndex].x}, {formatChartNumber(
                  safeSeries[seriesIndex].segments[segmentIndex][pointIndex].y,
                )}
              </title>
            </circle>
          {/each}
        {/each}
      {/each}

      {#if xLabel}
        <text
          class="jr-area__axis-label"
          x={(PLOT_LEFT + PLOT_RIGHT) / 2}
          y={CHART_VIEWBOX_HEIGHT - 6}
          text-anchor="middle">{shortLabel(xLabel)}</text
        >
      {/if}
      {#if yLabel}
        <text
          class="jr-area__axis-label"
          x={14}
          y={(PLOT_TOP + PLOT_BOTTOM) / 2}
          text-anchor="middle"
          transform={`rotate(-90 14 ${(PLOT_TOP + PLOT_BOTTOM) / 2})`}
          >{shortLabel(yLabel)}</text
        >
      {/if}
    </svg>

    <ul class="jr-area__legend" aria-label="Series legend">
      {#each safeSeries as item, index (index)}
        <li>
          <span
            class="jr-area__swatch"
            class:jr-area__swatch--dashed={seriesDash(index)}
            style:border-top-color={chartColor(index)}
          ></span>
          <span title={item.name}>{item.name || `Series ${index + 1}`}</span>
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .jr-area {
    min-width: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-4);
    overflow: hidden;
  }

  h3 {
    margin: 0 0 var(--space-3);
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-size: var(--text-base);
  }

  svg {
    display: block;
    width: 100%;
    max-width: 100%;
  }

  text {
    fill: var(--muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }

  .jr-area__axis-label {
    fill: var(--ink);
    font-family: var(--font-display);
    font-size: 12px;
  }

  .jr-area__empty {
    min-height: 8rem;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-size: var(--text-sm);
  }

  .jr-area__legend {
    list-style: none;
    margin: var(--space-2) 0 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-4);
    color: var(--muted);
    font-size: var(--text-xs);
  }

  .jr-area__legend li {
    min-width: 0;
    max-width: 100%;
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }

  .jr-area__legend li span:last-child {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .jr-area__swatch {
    width: 0.75rem;
    height: 0;
    flex: 0 0 auto;
    border-top: 3px solid;
  }

  .jr-area__swatch--dashed {
    border-top-style: dashed;
  }

  .jr-area__sr {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
