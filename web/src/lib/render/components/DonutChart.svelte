<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_DONUT_CHART, DATA_JR_TYPE } from "$lib/common/render";
  import type { LabelValue } from "$lib/render/charts/types";
  import {
    chartHeight,
    finiteNumber,
    formatChartNumber,
  } from "$lib/render/charts/math";
  import { CHART_PALETTE, chartColor } from "$lib/render/charts/palette";

  type DonutChartProps = {
    id?: string | null;
    title?: string | null;
    centerLabel?: string | null;
    slices?: LabelValue[] | null;
    height?: number | null;
  };

  interface DonutSlice extends LabelValue {
    color: string;
    offset: number;
    fraction: number;
    opacity: number;
  }

  const MAX_SLICES = 100;
  const RADIUS = 82;
  const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

  const { props }: BaseComponentProps<DonutChartProps> = $props();

  const height = $derived(chartHeight(props.height));

  function sliceOpacity(index: number): number {
    const repeat = Math.floor(index / CHART_PALETTE.length);
    const repeatCount = Math.ceil(MAX_SLICES / CHART_PALETTE.length);
    return 1 - Math.sqrt(repeat / repeatCount) * 0.65;
  }

  const chart = $derived.by(() => {
    if (!Array.isArray(props.slices)) {
      return { slices: [] as DonutSlice[], total: 0 };
    }

    const valid = props.slices
      .flatMap((slice) => {
        if (typeof slice !== "object" || slice === null) {
          return [];
        }
        const value = finiteNumber(slice.value);
        if (typeof slice.label !== "string" || value === null || value <= 0) {
          return [];
        }
        return [{ label: slice.label, value }];
      })
      .slice(0, MAX_SLICES);
    const total = valid.reduce((sum, slice) => sum + slice.value, 0);
    if (!Number.isFinite(total) || total <= 0) {
      return { slices: [] as DonutSlice[], total: 0 };
    }

    let offset = 0;
    const slices = valid.map((slice, index) => {
      const fraction = slice.value / total;
      const item = {
        ...slice,
        color: chartColor(index),
        offset,
        fraction,
        opacity: sliceOpacity(index),
      };
      offset += fraction;
      return item;
    });
    return { slices, total };
  });
</script>

<figure
  class="jr-donut"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_DONUT_CHART }}
>
  {#if props.title}
    <figcaption>{props.title}</figcaption>
  {/if}
  {#if chart.slices.length === 0}
    <div class="jr-donut__empty">No positive slice data</div>
  {:else}
    <div class="jr-donut__body">
      <svg
        viewBox="0 0 240 240"
        style:height="{height}px"
        role="img"
        aria-label={props.title ?? "Donut chart"}
      >
        <circle
          cx="120"
          cy="120"
          r={RADIUS}
          fill="none"
          stroke="var(--panel-2)"
          stroke-width="34"
        />
        {#each chart.slices as slice, index (`${slice.label}-${index}`)}
          <circle
            cx="120"
            cy="120"
            r={RADIUS}
            fill="none"
            stroke={slice.color}
            stroke-opacity={slice.opacity}
            stroke-width="34"
            stroke-dasharray="{slice.fraction * CIRCUMFERENCE} {CIRCUMFERENCE}"
            stroke-dashoffset={-slice.offset * CIRCUMFERENCE}
            transform="rotate(-90 120 120)"
          >
            <title
              >{slice.label}: {slice.value} ({(slice.fraction * 100).toFixed(
                1,
              )}%)</title
            >
          </circle>
        {/each}
        <text x="120" y="112" text-anchor="middle" class="jr-donut__total">
          {formatChartNumber(chart.total)}
        </text>
        {#if props.centerLabel}
          <text x="120" y="135" text-anchor="middle" class="jr-donut__label">
            {props.centerLabel}
          </text>
        {/if}
      </svg>
      <ul aria-label="Slices">
        {#each chart.slices as slice, index (`legend-${slice.label}-${index}`)}
          <li>
            <span style:background={slice.color} style:opacity={slice.opacity}
            ></span>
            <span>{slice.label}</span>
            <strong
              >{formatChartNumber(slice.value)} · {(
                slice.fraction * 100
              ).toFixed(1)}%</strong
            >
          </li>
        {/each}
      </ul>
      <p class="sr-only">
        {props.title ?? "Donut chart"}. Total {chart.total}. {chart.slices
          .length}
        positive slices. Each slice is listed with its value and percentage; repeated
        colors use different opacity levels.
      </p>
    </div>
  {/if}
</figure>

<style>
  .jr-donut {
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
    font-weight: 600;
  }

  .jr-donut__body {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(12rem, 100%), 1fr));
    align-items: center;
    gap: var(--space-4);
  }

  svg {
    display: block;
    width: 100%;
    max-height: 32rem;
  }

  .jr-donut__total {
    fill: var(--ink);
    font-family: var(--font-mono);
    font-size: 22px;
    font-weight: 700;
  }

  .jr-donut__label {
    fill: var(--muted);
    font-family: var(--font-display);
    font-size: 12px;
  }

  ul {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: 0;
    padding: 0;
    list-style: none;
    font-size: var(--text-xs);
  }

  li {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-2);
  }

  li > span:first-child {
    width: 0.7rem;
    height: 0.7rem;
    border-radius: 50%;
  }

  li > span:nth-child(2) {
    overflow-wrap: anywhere;
  }

  strong {
    font-family: var(--font-mono);
    font-weight: 500;
    color: var(--muted);
  }

  .jr-donut__empty {
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
