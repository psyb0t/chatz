<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_SPARKLINE, DATA_JR_TYPE } from "$lib/common/render";
  import {
    extent,
    finiteValues,
    formatChartNumber,
    linePath,
    scaleLinear,
  } from "$lib/render/charts/math";
  import { chartColor } from "$lib/render/charts/palette";

  type Trend = "up" | "down" | "flat";

  type SparklineProps = {
    id?: string | null;
    label?: string | null;
    value?: string | null;
    unit?: string | null;
    values?: number[] | null;
    trend?: Trend | null;
  };

  const SPARK_WIDTH = 180;
  const SPARK_HEIGHT = 52;
  const SPARK_PADDING = 4;
  const MAX_VALUES = 300;
  const MAX_SUMMARY_LABEL_LENGTH = 40;

  const { props }: BaseComponentProps<SparklineProps> = $props();

  function safeText(value: unknown): string {
    return typeof value === "string" ? value : "";
  }

  function shortSummaryLabel(value: string): string {
    return value.length <= MAX_SUMMARY_LABEL_LENGTH
      ? value
      : `${value.slice(0, MAX_SUMMARY_LABEL_LENGTH - 1)}…`;
  }

  function summarizeValues(
    chartLabel: string,
    displayValue: string,
    displayUnit: string,
    chartValues: readonly number[],
    chartTrend: Trend | null,
  ): string {
    const name = shortSummaryLabel(chartLabel || "Sparkline");
    const reading = shortSummaryLabel(displayValue || "not provided");
    const unitSummary = displayUnit ? ` ${shortSummaryLabel(displayUnit)}` : "";
    if (chartValues.length === 0) {
      return `${name} is ${reading}${unitSummary}. No finite trend samples are available.`;
    }

    const minimum = Math.min(...chartValues);
    const maximum = Math.max(...chartValues);
    const trendSummary = chartTrend
      ? ` The declared trend is ${chartTrend}.`
      : "";
    return `${name} is ${reading}${unitSummary}, based on ${chartValues.length} finite samples. The series starts at ${formatChartNumber(chartValues[0])}, ends at ${formatChartNumber(chartValues.at(-1)!)}, and ranges from ${formatChartNumber(minimum)} to ${formatChartNumber(maximum)}.${trendSummary}`;
  }

  const rootID = $derived(typeof props.id === "string" ? props.id : undefined);
  const label = $derived(safeText(props.label));
  const unit = $derived(safeText(props.unit));
  const value = $derived(safeText(props.value));
  const values = $derived(finiteValues(props.values).slice(0, MAX_VALUES));
  const trend = $derived<Trend | null>(
    props.trend === "up" || props.trend === "down" || props.trend === "flat"
      ? props.trend
      : null,
  );
  const domain = $derived(extent(values));
  const path = $derived(
    linePath(
      values.map((item, index) => ({
        x:
          values.length === 1
            ? SPARK_WIDTH / 2
            : scaleLinear(
                index,
                0,
                values.length - 1,
                SPARK_PADDING,
                SPARK_WIDTH - SPARK_PADDING,
              ),
        y: scaleLinear(
          item,
          domain[0],
          domain[1],
          SPARK_HEIGHT - SPARK_PADDING,
          SPARK_PADDING,
        ),
      })),
    ),
  );
  const trendSymbol = $derived(
    trend === "up" ? "▲" : trend === "down" ? "▼" : trend === "flat" ? "—" : "",
  );
  const summary = $derived(summarizeValues(label, value, unit, values, trend));
</script>

<section
  class="jr-sparkline"
  id={rootID}
  aria-label={label || "Sparkline"}
  {...{ [DATA_JR_TYPE]: COMP_SPARKLINE }}
>
  <p class="jr-sparkline__sr">{summary}</p>
  <div class="jr-sparkline__content">
    <div class="jr-sparkline__label">{label}</div>
    <div class="jr-sparkline__reading">
      <span class="jr-sparkline__value">
        {value || "—"}
      </span>
      {#if unit}<span class="jr-sparkline__unit">{unit}</span>{/if}
      {#if trend}
        <span class="jr-sparkline__trend jr-sparkline__trend--{trend}">
          <span aria-hidden="true">{trendSymbol}</span>
          <span class="jr-sparkline__sr">Trend {trend}</span>
        </span>
      {/if}
    </div>
  </div>

  {#if values.length > 0}
    <svg
      viewBox={`0 0 ${SPARK_WIDTH} ${SPARK_HEIGHT}`}
      role="img"
      aria-label={`${shortSummaryLabel(label || "Sparkline")} trend line with ${values.length} samples`}
    >
      <title>{label || "Sparkline"} values</title>
      <path
        d={path}
        fill="none"
        stroke={chartColor(0)}
        stroke-width="3"
        stroke-linecap="round"
        stroke-linejoin="round"
        vector-effect="non-scaling-stroke"
      />
      {#if values.length === 1}
        <circle
          cx={SPARK_WIDTH / 2}
          cy={SPARK_HEIGHT / 2}
          r="3"
          fill={chartColor(0)}
        >
          <title>{formatChartNumber(values[0])}</title>
        </circle>
      {/if}
    </svg>
  {:else}
    <div class="jr-sparkline__empty" role="status">No finite trend data</div>
  {/if}
</section>

<style>
  .jr-sparkline {
    min-width: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-3);
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(7rem, 11rem);
    align-items: center;
    gap: var(--space-3);
  }

  .jr-sparkline__content {
    min-width: 0;
  }

  .jr-sparkline__label {
    overflow-wrap: anywhere;
    color: var(--muted);
    font-family: var(--font-display);
    font-size: var(--text-xs);
  }

  .jr-sparkline__reading {
    display: flex;
    align-items: baseline;
    gap: var(--space-1);
    min-width: 0;
    font-family: var(--font-mono);
  }

  .jr-sparkline__value {
    overflow-wrap: anywhere;
    font-size: var(--text-xl);
    font-weight: 600;
  }

  .jr-sparkline__unit {
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--muted);
    font-size: var(--text-xs);
    white-space: nowrap;
  }

  .jr-sparkline__trend {
    font-size: var(--text-xs);
    font-weight: 700;
  }

  .jr-sparkline__trend--up {
    color: var(--ok);
  }

  .jr-sparkline__trend--down {
    color: var(--crit);
  }

  .jr-sparkline__trend--flat {
    color: var(--muted);
  }

  svg {
    display: block;
    width: 100%;
    max-width: 11rem;
    justify-self: end;
  }

  .jr-sparkline__empty {
    color: var(--muted);
    font-size: var(--text-xs);
    text-align: right;
  }

  .jr-sparkline__sr {
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

  @media (max-width: 30rem) {
    .jr-sparkline {
      grid-template-columns: 1fr;
    }

    svg {
      max-width: none;
      justify-self: stretch;
    }

    .jr-sparkline__empty {
      text-align: left;
    }
  }
</style>
