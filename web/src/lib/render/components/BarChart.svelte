<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_BAR_CHART, DATA_JR_TYPE } from "$lib/common/render";
  import type { CategorySeries } from "$lib/render/charts/types";
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
  import {
    CHART_GRID_COLOR,
    CHART_PALETTE,
    CHART_TEXT_COLOR,
    chartColor,
  } from "$lib/render/charts/palette";

  type BarChartProps = {
    id?: string | null;
    title?: string | null;
    categories?: string[] | null;
    series?: CategorySeries[] | null;
    stacked?: boolean | null;
    orientation?: "horizontal" | "vertical" | null;
    height?: number | null;
  };

  interface BarMark {
    category: string;
    series: string;
    value: number;
    x: number;
    y: number;
    width: number;
    height: number;
    color: string;
    opacity: number;
  }

  const MAX_CATEGORIES = 100;
  const MAX_SERIES = 12;
  const MIN_BAR_SIZE = 1;
  const CATEGORY_LABEL_LENGTH = 18;
  const MAX_CATEGORY_LABELS = 12;
  const PLOT_WIDTH =
    CHART_VIEWBOX_WIDTH - CHART_MARGIN_LEFT - CHART_MARGIN_RIGHT;
  const PLOT_HEIGHT =
    CHART_VIEWBOX_HEIGHT - CHART_MARGIN_TOP - CHART_MARGIN_BOTTOM;

  const { props }: BaseComponentProps<BarChartProps> = $props();

  const categories = $derived(
    Array.isArray(props.categories)
      ? props.categories
          .filter(
            (category): category is string => typeof category === "string",
          )
          .slice(0, MAX_CATEGORIES)
      : [],
  );
  const series = $derived.by(() => {
    if (!Array.isArray(props.series)) {
      return [];
    }

    return props.series
      .filter(
        (item): item is CategorySeries =>
          typeof item === "object" &&
          item !== null &&
          typeof item.name === "string" &&
          Array.isArray(item.values),
      )
      .slice(0, MAX_SERIES);
  });
  const stacked = $derived(props.stacked === true);
  const horizontal = $derived(props.orientation === "horizontal");
  const height = $derived(chartHeight(props.height));
  const categoryLabelStride = $derived(
    Math.max(1, Math.ceil(categories.length / MAX_CATEGORY_LABELS)),
  );

  function seriesOpacity(index: number): number {
    const repeat = Math.floor(index / CHART_PALETTE.length);
    const repeatCount = Math.ceil(MAX_SERIES / CHART_PALETTE.length);
    return 1 - Math.sqrt(repeat / repeatCount) * 0.65;
  }

  const chart = $derived.by(() => {
    if (categories.length === 0 || series.length === 0) {
      return { marks: [] as BarMark[], domain: [0, 1] as [number, number] };
    }

    const categoryValues = categories.map((_, categoryIndex) =>
      series.map((item) => finiteNumber(item.values[categoryIndex])),
    );
    let domainValues: number[];

    if (stacked) {
      domainValues = categoryValues.flatMap((values) => {
        const positive = values.reduce<number>(
          (sum, value) => sum + (value !== null && value > 0 ? value : 0),
          0,
        );
        const negative = values.reduce<number>(
          (sum, value) => sum + (value !== null && value < 0 ? value : 0),
          0,
        );
        return [negative, positive];
      });
    } else {
      domainValues = categoryValues.flatMap((values) =>
        values.flatMap((value) => (value === null ? [] : [value])),
      );
    }

    const domain = extent(domainValues, true);
    const marks: BarMark[] = [];
    const bandSize =
      (horizontal ? PLOT_HEIGHT : PLOT_WIDTH) / categories.length;
    const groupPadding = Math.min(8, bandSize * 0.18);
    const usableBand = Math.max(MIN_BAR_SIZE, bandSize - groupPadding);

    categoryValues.forEach((values, categoryIndex) => {
      let positiveStack = 0;
      let negativeStack = 0;

      values.forEach((value, seriesIndex) => {
        if (value === null) {
          return;
        }

        let start = 0;
        let end = value;
        if (stacked) {
          if (value >= 0) {
            start = positiveStack;
            positiveStack += value;
            end = positiveStack;
          } else {
            start = negativeStack;
            negativeStack += value;
            end = negativeStack;
          }
        }

        const subgroupSize = stacked
          ? usableBand
          : usableBand / Math.max(series.length, 1);
        const subgroupOffset = stacked ? 0 : seriesIndex * subgroupSize;

        if (horizontal) {
          const x0 = scaleLinear(start, domain[0], domain[1], 0, PLOT_WIDTH);
          const x1 = scaleLinear(end, domain[0], domain[1], 0, PLOT_WIDTH);
          marks.push({
            category: categories[categoryIndex],
            series: series[seriesIndex].name,
            value,
            x: CHART_MARGIN_LEFT + Math.min(x0, x1),
            y:
              CHART_MARGIN_TOP +
              categoryIndex * bandSize +
              groupPadding / 2 +
              subgroupOffset,
            width: Math.max(MIN_BAR_SIZE, Math.abs(x1 - x0)),
            height: Math.max(MIN_BAR_SIZE, subgroupSize),
            color: chartColor(seriesIndex),
            opacity: seriesOpacity(seriesIndex),
          });
          return;
        }

        const y0 = scaleLinear(start, domain[0], domain[1], PLOT_HEIGHT, 0);
        const y1 = scaleLinear(end, domain[0], domain[1], PLOT_HEIGHT, 0);
        marks.push({
          category: categories[categoryIndex],
          series: series[seriesIndex].name,
          value,
          x:
            CHART_MARGIN_LEFT +
            categoryIndex * bandSize +
            groupPadding / 2 +
            subgroupOffset,
          y: CHART_MARGIN_TOP + Math.min(y0, y1),
          width: Math.max(MIN_BAR_SIZE, subgroupSize),
          height: Math.max(MIN_BAR_SIZE, Math.abs(y1 - y0)),
          color: chartColor(seriesIndex),
          opacity: seriesOpacity(seriesIndex),
        });
      });
    });

    return { marks, domain };
  });

  const axisTicks = $derived(ticks(chart.domain[0], chart.domain[1]));

  function shortLabel(label: string): string {
    return label.length > CATEGORY_LABEL_LENGTH
      ? `${label.slice(0, CATEGORY_LABEL_LENGTH - 1)}…`
      : label;
  }
</script>

<figure
  class="jr-chart"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_BAR_CHART }}
>
  {#if props.title}
    <figcaption>{props.title}</figcaption>
  {/if}
  {#if chart.marks.length === 0}
    <div class="jr-chart__empty">No finite bar data</div>
  {:else}
    <svg
      viewBox="0 0 {CHART_VIEWBOX_WIDTH} {CHART_VIEWBOX_HEIGHT}"
      style:height="{height}px"
      role="img"
      aria-label={props.title ?? "Bar chart"}
      preserveAspectRatio="xMidYMid meet"
    >
      {#each axisTicks as tick}
        {@const tickPosition = horizontal
          ? scaleLinear(
              tick,
              chart.domain[0],
              chart.domain[1],
              CHART_MARGIN_LEFT,
              CHART_MARGIN_LEFT + PLOT_WIDTH,
            )
          : scaleLinear(
              tick,
              chart.domain[0],
              chart.domain[1],
              CHART_MARGIN_TOP + PLOT_HEIGHT,
              CHART_MARGIN_TOP,
            )}
        {#if horizontal}
          <line
            x1={tickPosition}
            x2={tickPosition}
            y1={CHART_MARGIN_TOP}
            y2={CHART_MARGIN_TOP + PLOT_HEIGHT}
            stroke={CHART_GRID_COLOR}
          />
          <text
            x={tickPosition}
            y={CHART_MARGIN_TOP + PLOT_HEIGHT + 22}
            text-anchor="middle"
            fill={CHART_TEXT_COLOR}>{formatChartNumber(tick)}</text
          >
        {:else}
          <line
            x1={CHART_MARGIN_LEFT}
            x2={CHART_MARGIN_LEFT + PLOT_WIDTH}
            y1={tickPosition}
            y2={tickPosition}
            stroke={CHART_GRID_COLOR}
          />
          <text
            x={CHART_MARGIN_LEFT - 10}
            y={tickPosition + 4}
            text-anchor="end"
            fill={CHART_TEXT_COLOR}>{formatChartNumber(tick)}</text
          >
        {/if}
      {/each}

      {#each chart.marks as mark, index (`${mark.category}-${mark.series}-${index}`)}
        <rect
          x={mark.x}
          y={mark.y}
          width={mark.width}
          height={mark.height}
          fill={mark.color}
          fill-opacity={mark.opacity}
          rx="2"
        >
          <title>{mark.category} — {mark.series}: {mark.value}</title>
        </rect>
      {/each}

      {#each categories as category, index (index)}
        {#if index % categoryLabelStride === 0 || index === categories.length - 1}
          {#if horizontal}
            <text
              x={CHART_MARGIN_LEFT - 10}
              y={CHART_MARGIN_TOP +
                ((index + 0.5) * PLOT_HEIGHT) / categories.length +
                4}
              text-anchor="end"
              fill={CHART_TEXT_COLOR}
            >
              <title>{category}</title>{shortLabel(category)}
            </text>
          {:else}
            <text
              x={CHART_MARGIN_LEFT +
                ((index + 0.5) * PLOT_WIDTH) / categories.length}
              y={CHART_MARGIN_TOP + PLOT_HEIGHT + 22}
              text-anchor="middle"
              fill={CHART_TEXT_COLOR}
            >
              <title>{category}</title>{shortLabel(category)}
            </text>
          {/if}
        {/if}
      {/each}
    </svg>
    <ul class="jr-chart__legend" aria-label="Series">
      {#each series as item, index (index)}
        <li>
          <span
            style:background={chartColor(index)}
            style:opacity={seriesOpacity(index)}
          ></span>{item.name}
        </li>
      {/each}
    </ul>
    <table class="sr-only">
      <caption>
        {props.title ?? "Bar chart"}. {stacked ? "Stacked" : "Grouped"}
        {horizontal ? "horizontal" : "vertical"} bars. Visual series with repeated
        colors use different opacity levels.
      </caption>
      <thead>
        <tr>
          <th>Category</th>
          {#each series as item, index (`summary-head-${item.name}-${index}`)}
            <th>{item.name}</th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each categories as category, categoryIndex (`summary-${category}-${categoryIndex}`)}
          <tr>
            <th>{category}</th>
            {#each series as item, seriesIndex (`summary-value-${seriesIndex}`)}
              {@const value = finiteNumber(item.values[categoryIndex])}
              <td>{value === null ? "No value" : value}</td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</figure>

<style>
  .jr-chart {
    min-width: 0;
    margin: 0;
    padding: var(--space-4);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    color: var(--ink);
  }

  figcaption {
    margin-bottom: var(--space-3);
    font-family: var(--font-display);
    font-size: var(--text-base);
    font-weight: 600;
  }

  svg {
    display: block;
    width: 100%;
    max-width: 100%;
    font-family: var(--font-mono);
    font-size: 11px;
  }

  .jr-chart__legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-4);
    margin: var(--space-2) 0 0;
    padding: 0;
    list-style: none;
    color: var(--muted);
    font-size: var(--text-xs);
  }

  .jr-chart__legend li {
    display: flex;
    align-items: center;
    gap: var(--space-1);
  }

  .jr-chart__legend span {
    width: 0.65rem;
    height: 0.65rem;
    border-radius: 2px;
  }

  .jr-chart__empty {
    padding: var(--space-6);
    color: var(--muted);
    text-align: center;
  }

  .sr-only {
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
