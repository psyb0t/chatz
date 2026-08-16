<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_BOX_PLOT, DATA_JR_TYPE } from "$lib/common/render";
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
  import type { BoxSummary } from "$lib/render/charts/types";

  type BoxPlotProps = {
    id?: string | null;
    title?: string | null;
    yLabel?: string | null;
    groups?: BoxSummary[] | null;
    height?: number | null;
  };
  const MAX_GROUPS = 80;
  const PLOT_LEFT = CHART_MARGIN_LEFT;
  const PLOT_RIGHT = CHART_VIEWBOX_WIDTH - CHART_MARGIN_RIGHT;
  const PLOT_TOP = CHART_MARGIN_TOP;
  const PLOT_BOTTOM = CHART_VIEWBOX_HEIGHT - CHART_MARGIN_BOTTOM;
  const { props }: BaseComponentProps<BoxPlotProps> = $props();
  const height = $derived(chartHeight(props.height));
  const groups = $derived.by((): BoxSummary[] => {
    if (!Array.isArray(props.groups)) return [];
    return props.groups.slice(0, MAX_GROUPS).flatMap((group, index) => {
      if (!group || typeof group !== "object") return [];
      const values = [
        group.min,
        group.q1,
        group.median,
        group.q3,
        group.max,
      ].map(finiteNumber);
      if (values.some((value) => value === null)) return [];
      const [min, q1, median, q3, max] = values as number[];
      if (!(min <= q1 && q1 <= median && median <= q3 && q3 <= max)) return [];
      return [
        {
          label:
            typeof group.label === "string" && group.label
              ? group.label
              : String(index + 1),
          min,
          q1,
          median,
          q3,
          max,
        },
      ];
    });
  });
  const domain = $derived(
    extent(groups.flatMap((group) => [group.min, group.max])),
  );
  const yTicks = $derived(ticks(domain[0], domain[1]));
  const slotWidth = $derived(
    groups.length > 0 ? (PLOT_RIGHT - PLOT_LEFT) / groups.length : 0,
  );
  const labelStep = $derived(Math.max(1, Math.ceil(groups.length / 12)));
  const description = $derived(
    `${groups.length} valid five-number summaries. Overall values range from ${formatChartNumber(domain[0])} to ${formatChartNumber(domain[1])}.`,
  );

  function shortLabel(label: string): string {
    const maxCharacters = 8;
    return label.length <= maxCharacters
      ? label
      : `${label.slice(0, maxCharacters - 1)}…`;
  }
</script>

<section
  class="jr-chart"
  id={props.id ?? undefined}
  style:height={`${height}px`}
  {...{ [DATA_JR_TYPE]: COMP_BOX_PLOT }}
>
  {#if props.title}<h3>{props.title}</h3>{/if}
  {#if groups.length === 0}
    <div class="jr-chart__empty">No valid box plot data</div>
  {:else}
    <svg
      viewBox={`0 0 ${CHART_VIEWBOX_WIDTH} ${CHART_VIEWBOX_HEIGHT}`}
      role="img"
      aria-label={props.title || "Box plot"}
    >
      <desc>{description}</desc>
      {#each yTicks as tick}
        {@const y = scaleLinear(
          tick,
          domain[0],
          domain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        <line
          x1={PLOT_LEFT}
          y1={y}
          x2={PLOT_RIGHT}
          y2={y}
          stroke={CHART_GRID_COLOR}
          ><title>Grid line at {formatChartNumber(tick)}</title></line
        >
        <text x={PLOT_LEFT - 10} y={y + 4} text-anchor="end"
          >{formatChartNumber(tick)}</text
        >
      {/each}
      {#each groups as group, index}
        {@const center = PLOT_LEFT + index * slotWidth + slotWidth / 2}
        {@const width = Math.min(44, Math.max(6, slotWidth * 0.58))}
        {@const minY = scaleLinear(
          group.min,
          domain[0],
          domain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        {@const q1Y = scaleLinear(
          group.q1,
          domain[0],
          domain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        {@const medianY = scaleLinear(
          group.median,
          domain[0],
          domain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        {@const q3Y = scaleLinear(
          group.q3,
          domain[0],
          domain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        {@const maxY = scaleLinear(
          group.max,
          domain[0],
          domain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        {@const summary = `${group.label}: min ${formatChartNumber(group.min)}, Q1 ${formatChartNumber(group.q1)}, median ${formatChartNumber(group.median)}, Q3 ${formatChartNumber(group.q3)}, max ${formatChartNumber(group.max)}`}
        <line
          x1={center}
          y1={maxY}
          x2={center}
          y2={minY}
          stroke={chartColor(index)}
          stroke-width="2"><title>{summary}</title></line
        >
        <line
          x1={center - width / 3}
          y1={maxY}
          x2={center + width / 3}
          y2={maxY}
          stroke={chartColor(index)}
          stroke-width="2"><title>{summary}</title></line
        >
        <line
          x1={center - width / 3}
          y1={minY}
          x2={center + width / 3}
          y2={minY}
          stroke={chartColor(index)}
          stroke-width="2"><title>{summary}</title></line
        >
        <rect
          x={center - width / 2}
          y={q3Y}
          {width}
          height={Math.max(1, q1Y - q3Y)}
          fill={chartColor(index)}
          fill-opacity="0.22"
          stroke={chartColor(index)}
          stroke-width="2"><title>{summary}</title></rect
        >
        <line
          x1={center - width / 2}
          y1={medianY}
          x2={center + width / 2}
          y2={medianY}
          stroke={chartColor(index)}
          stroke-width="3"><title>{summary}</title></line
        >
        {#if index % labelStep === 0 || index === groups.length - 1}
          <text x={center} y={PLOT_BOTTOM + 18} text-anchor="middle"
            >{shortLabel(group.label)}</text
          >
        {/if}
      {/each}
      {#if props.yLabel}<text
          class="axis-label"
          transform={`translate(16 ${(PLOT_TOP + PLOT_BOTTOM) / 2}) rotate(-90)`}
          text-anchor="middle">{props.yLabel}</text
        >{/if}
    </svg>
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
    font-size: 10px;
  }
  .axis-label {
    fill: var(--ink);
    font-family: var(--font-display);
    font-size: 12px;
  }
  .jr-chart__empty {
    flex: 1;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-size: var(--text-sm);
  }
</style>
