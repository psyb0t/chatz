<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_HISTOGRAM, DATA_JR_TYPE } from "$lib/common/render";
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
  import { CHART_GRID_COLOR } from "$lib/render/charts/palette";
  import type { LabelValue } from "$lib/render/charts/types";

  type HistogramProps = {
    id?: string | null;
    title?: string | null;
    xLabel?: string | null;
    yLabel?: string | null;
    bins?: LabelValue[] | null;
    height?: number | null;
  };
  interface Bin {
    label: string;
    value: number;
  }

  const MAX_BINS = 200;
  const PLOT_LEFT = CHART_MARGIN_LEFT;
  const PLOT_RIGHT = CHART_VIEWBOX_WIDTH - CHART_MARGIN_RIGHT;
  const PLOT_TOP = CHART_MARGIN_TOP;
  const PLOT_BOTTOM = CHART_VIEWBOX_HEIGHT - CHART_MARGIN_BOTTOM;
  const { props }: BaseComponentProps<HistogramProps> = $props();
  const height = $derived(chartHeight(props.height));
  const bins = $derived.by((): Bin[] => {
    if (!Array.isArray(props.bins)) return [];
    return props.bins.slice(0, MAX_BINS).flatMap((bin, index) => {
      if (!bin || typeof bin !== "object") return [];
      const value = finiteNumber(bin.value);
      if (value === null) return [];
      return [
        {
          label:
            typeof bin.label === "string" && bin.label
              ? bin.label
              : String(index + 1),
          value,
        },
      ];
    });
  });
  const domain = $derived(
    extent(
      bins.map((bin) => bin.value),
      true,
    ),
  );
  const yTicks = $derived(ticks(domain[0], domain[1]));
  const baseline = $derived(
    scaleLinear(0, domain[0], domain[1], PLOT_BOTTOM, PLOT_TOP),
  );
  const slotWidth = $derived(
    bins.length > 0 ? (PLOT_RIGHT - PLOT_LEFT) / bins.length : 0,
  );
  const description = $derived(
    `${bins.length} finite bins. Values range from ${formatChartNumber(domain[0])} to ${formatChartNumber(domain[1])}.`,
  );
</script>

<section
  class="jr-chart"
  id={props.id ?? undefined}
  style:height={`${height}px`}
  {...{ [DATA_JR_TYPE]: COMP_HISTOGRAM }}
>
  {#if props.title}<h3>{props.title}</h3>{/if}
  {#if bins.length === 0}
    <div class="jr-chart__empty">No histogram data</div>
  {:else}
    <svg
      viewBox={`0 0 ${CHART_VIEWBOX_WIDTH} ${CHART_VIEWBOX_HEIGHT}`}
      role="img"
      aria-label={props.title || "Histogram"}
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
      {#each bins as bin, index}
        {@const valueY = scaleLinear(
          bin.value,
          domain[0],
          domain[1],
          PLOT_BOTTOM,
          PLOT_TOP,
        )}
        {@const x = PLOT_LEFT + index * slotWidth + 1}
        <rect
          {x}
          y={Math.min(valueY, baseline)}
          width={Math.max(1, slotWidth - 2)}
          height={Math.max(1, Math.abs(baseline - valueY))}
          fill="var(--accent)"
          fill-opacity="0.82"
        >
          <title>{bin.label}: {formatChartNumber(bin.value)}</title>
        </rect>
        {#if bins.length <= 24 || index % Math.ceil(bins.length / 24) === 0}
          <text x={x + slotWidth / 2} y={PLOT_BOTTOM + 18} text-anchor="middle"
            >{bin.label}</text
          >
        {/if}
      {/each}
      {#if props.xLabel}<text
          class="axis-label"
          x={(PLOT_LEFT + PLOT_RIGHT) / 2}
          y={CHART_VIEWBOX_HEIGHT - 5}
          text-anchor="middle">{props.xLabel}</text
        >{/if}
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
