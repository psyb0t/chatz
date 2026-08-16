<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_GAUGE, DATA_JR_TYPE } from "$lib/common/render";
  import {
    clamp,
    finiteNumber,
    formatChartNumber,
  } from "$lib/render/charts/math";

  type GaugeProps = {
    id?: string | null;
    label?: string | null;
    value?: number | null;
    min?: number | null;
    max?: number | null;
    unit?: string | null;
    warn?: number | null;
    crit?: number | null;
  };

  const DEFAULT_MIN = 0;
  const DEFAULT_MAX = 100;
  const ARC_LENGTH = 251.33;

  const { props }: BaseComponentProps<GaugeProps> = $props();

  const gauge = $derived.by(() => {
    const rawValue = finiteNumber(props.value);
    if (rawValue === null) {
      return null;
    }

    const first = finiteNumber(props.min) ?? DEFAULT_MIN;
    const second = finiteNumber(props.max) ?? DEFAULT_MAX;
    let min = Math.min(first, second);
    let max = Math.max(first, second);
    if (min === max) {
      if (min > 0) {
        min = 0;
      } else if (max < 0) {
        max = 0;
      } else {
        min = -1;
        max = 1;
      }
    }

    const value = clamp(rawValue, min, max);
    const range = max - min;
    const normalized = (value - min) / range;
    const percentage = Number.isFinite(normalized)
      ? normalized * 100
      : value <= min
        ? 0
        : value >= max
          ? 100
          : 50;
    const warn = finiteNumber(props.warn);
    const crit = finiteNumber(props.crit);
    let level: "ok" | "warn" | "crit" = "ok";
    if (crit !== null && rawValue >= crit) {
      level = "crit";
    } else if (warn !== null && rawValue >= warn) {
      level = "warn";
    }

    return { rawValue, value, min, max, percentage, level };
  });
</script>

<figure
  class="jr-gauge"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_GAUGE }}
>
  {#if props.label}
    <figcaption>{props.label}</figcaption>
  {/if}
  {#if gauge === null}
    <div class="jr-gauge__empty">No finite gauge value</div>
  {:else}
    <svg
      viewBox="0 0 240 150"
      role="img"
      aria-label="{props.label ?? 'Gauge'}: {gauge.rawValue}{props.unit ?? ''}"
    >
      <path
        d="M40 120 A80 80 0 0 1 200 120"
        fill="none"
        stroke="var(--panel-2)"
        stroke-width="22"
        stroke-linecap="round"
      />
      <path
        class="jr-gauge__fill jr-gauge__fill--{gauge.level}"
        d="M40 120 A80 80 0 0 1 200 120"
        fill="none"
        stroke-width="22"
        stroke-linecap="round"
        pathLength={ARC_LENGTH}
        stroke-dasharray="{(gauge.percentage / 100) * ARC_LENGTH} {ARC_LENGTH}"
      >
        <title
          >{gauge.rawValue}{props.unit ?? ""}; normalized to {gauge.percentage.toFixed(
            1,
          )}% between {gauge.min} and {gauge.max}</title
        >
      </path>
      <text x="120" y="108" text-anchor="middle" class="jr-gauge__value">
        {formatChartNumber(gauge.rawValue)}{props.unit ?? ""}
      </text>
      <text x="40" y="145" text-anchor="middle" class="jr-gauge__bound">
        {formatChartNumber(gauge.min)}
      </text>
      <text x="200" y="145" text-anchor="middle" class="jr-gauge__bound">
        {formatChartNumber(gauge.max)}
      </text>
    </svg>
    <span class="jr-gauge__status jr-gauge__status--{gauge.level}">
      {gauge.level === "ok" ? "✓" : gauge.level === "warn" ? "!" : "×"}
      {gauge.level}
    </span>
    <p class="sr-only">
      {props.label ?? "Gauge"}: {gauge.rawValue}{props.unit ?? ""}. Range
      {gauge.min} to {gauge.max}; normalized position {gauge.percentage.toFixed(
        1,
      )} percent. Threshold status: {gauge.level}.
    </p>
  {/if}
</figure>

<style>
  .jr-gauge {
    position: relative;
    min-width: 0;
    margin: 0;
    padding: var(--space-4);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    color: var(--ink);
  }

  figcaption {
    font-family: var(--font-display);
    font-weight: 600;
    text-align: center;
  }

  svg {
    display: block;
    width: 100%;
    max-height: 18rem;
  }

  .jr-gauge__fill--ok {
    stroke: var(--ok);
  }

  .jr-gauge__fill--warn {
    stroke: var(--warn);
  }

  .jr-gauge__fill--crit {
    stroke: var(--crit);
  }

  .jr-gauge__value {
    fill: var(--ink);
    font-family: var(--font-mono);
    font-size: 22px;
    font-weight: 700;
  }

  .jr-gauge__bound {
    fill: var(--muted);
    font-family: var(--font-mono);
    font-size: 10px;
  }

  .jr-gauge__status {
    position: absolute;
    right: var(--space-3);
    bottom: var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 700;
    text-transform: uppercase;
  }

  .jr-gauge__status--ok {
    color: var(--ok);
  }

  .jr-gauge__status--warn {
    color: var(--warn);
  }

  .jr-gauge__status--crit {
    color: var(--crit);
  }

  .jr-gauge__empty {
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
