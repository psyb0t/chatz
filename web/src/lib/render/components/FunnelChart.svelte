<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_FUNNEL_CHART, DATA_JR_TYPE } from "$lib/common/render";
  import type { LabelValue } from "$lib/render/charts/types";
  import {
    chartHeight,
    finiteNumber,
    formatChartNumber,
  } from "$lib/render/charts/math";
  import { CHART_PALETTE, chartColor } from "$lib/render/charts/palette";

  type FunnelChartProps = {
    id?: string | null;
    title?: string | null;
    stages?: LabelValue[] | null;
    height?: number | null;
  };

  interface FunnelStage extends LabelValue {
    color: string;
    percentage: number;
    points: string;
    textY: number;
    opacity: number;
  }

  const MAX_STAGES = 50;
  const VIEWBOX_WIDTH = 720;
  const VIEWBOX_HEIGHT = 360;
  const MIN_STAGE_WIDTH = 24;
  const MAX_STAGE_WIDTH = 600;
  const STAGE_GAP = 4;

  const { props }: BaseComponentProps<FunnelChartProps> = $props();

  const height = $derived(chartHeight(props.height));

  function stageOpacity(index: number): number {
    const repeat = Math.floor(index / CHART_PALETTE.length);
    const repeatCount = Math.ceil(MAX_STAGES / CHART_PALETTE.length);
    return 1 - Math.sqrt(repeat / repeatCount) * 0.65;
  }

  const stages = $derived.by(() => {
    if (!Array.isArray(props.stages)) {
      return [];
    }

    const valid = props.stages
      .flatMap((stage) => {
        if (typeof stage !== "object" || stage === null) {
          return [];
        }
        const value = finiteNumber(stage.value);
        if (typeof stage.label !== "string" || value === null || value < 0) {
          return [];
        }
        return [{ label: stage.label, value }];
      })
      .slice(0, MAX_STAGES);
    if (valid.length === 0) {
      return [];
    }

    const baseline = valid[0].value;
    const maximum = Math.max(...valid.map((stage) => stage.value), 0);
    const stageHeight = (VIEWBOX_HEIGHT - 24) / valid.length;

    return valid.map((stage, index): FunnelStage => {
      const nextValue = valid[index + 1]?.value ?? stage.value;
      const startWidth =
        maximum > 0
          ? Math.max(MIN_STAGE_WIDTH, (stage.value / maximum) * MAX_STAGE_WIDTH)
          : MIN_STAGE_WIDTH;
      const endWidth =
        maximum > 0
          ? Math.max(MIN_STAGE_WIDTH, (nextValue / maximum) * MAX_STAGE_WIDTH)
          : MIN_STAGE_WIDTH;
      const y1 = 12 + index * stageHeight;
      const y2 = y1 + stageHeight - STAGE_GAP;
      const left1 = (VIEWBOX_WIDTH - startWidth) / 2;
      const left2 = (VIEWBOX_WIDTH - endWidth) / 2;
      const percentage = baseline > 0 ? (stage.value / baseline) * 100 : 0;

      return {
        ...stage,
        color: chartColor(index),
        percentage,
        points: `${left1},${y1} ${left1 + startWidth},${y1} ${left2 + endWidth},${y2} ${left2},${y2}`,
        textY: y1 + stageHeight / 2 + 4,
        opacity: stageOpacity(index),
      };
    });
  });
</script>

<figure
  class="jr-funnel"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_FUNNEL_CHART }}
>
  {#if props.title}
    <figcaption>{props.title}</figcaption>
  {/if}
  {#if stages.length === 0}
    <div class="jr-funnel__empty">No finite funnel data</div>
  {:else}
    <svg
      viewBox="0 0 {VIEWBOX_WIDTH} {VIEWBOX_HEIGHT}"
      style:height="{height}px"
      role="img"
      aria-label={props.title ?? "Conversion funnel"}
      preserveAspectRatio="xMidYMid meet"
    >
      {#each stages as stage, index (`${stage.label}-${index}`)}
        <polygon
          points={stage.points}
          fill={stage.color}
          fill-opacity={stage.opacity}
        >
          <title
            >{stage.label}: {stage.value} ({stage.percentage.toFixed(1)}% of
            first stage)</title
          >
        </polygon>
        <text x={VIEWBOX_WIDTH / 2} y={stage.textY} text-anchor="middle">
          {stage.label} · {formatChartNumber(stage.value)} · {stage.percentage.toFixed(
            1,
          )}%
        </text>
      {/each}
    </svg>
    <ol class="jr-funnel__details" aria-label="Funnel stages">
      {#each stages as stage, index (`detail-${stage.label}-${index}`)}
        <li>
          <span>{stage.label}</span>
          <strong
            >{formatChartNumber(stage.value)} · {stage.percentage.toFixed(
              1,
            )}%</strong
          >
        </li>
      {/each}
    </ol>
    <p class="sr-only">
      {props.title ?? "Conversion funnel"}. {stages.length} stages. Percentages compare
      each stage with the first stage. Repeated colors use different opacity levels.
    </p>
    <ol class="sr-only">
      {#each stages as stage, index (`summary-${stage.label}-${index}`)}
        <li>
          {stage.label}: {stage.value}; {stage.percentage.toFixed(1)} percent of the
          first stage.
        </li>
      {/each}
    </ol>
  {/if}
</figure>

<style>
  .jr-funnel {
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

  svg {
    display: block;
    width: 100%;
    max-width: 100%;
  }

  svg text {
    fill: var(--ink);
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .jr-funnel__details {
    display: none;
    margin: var(--space-3) 0 0;
    padding-left: var(--space-5);
    font-size: var(--text-xs);
  }

  .jr-funnel__details li {
    justify-content: space-between;
    gap: var(--space-3);
  }

  .jr-funnel__details strong {
    color: var(--muted);
    font-family: var(--font-mono);
  }

  .jr-funnel__empty {
    padding: var(--space-6);
    color: var(--muted);
    text-align: center;
  }

  @media (max-width: 36rem) {
    svg text {
      display: none;
    }

    .jr-funnel__details {
      display: block;
    }

    .jr-funnel__details li {
      display: flex;
    }
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
