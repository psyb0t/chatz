<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_PROGRESS, DATA_JR_TYPE } from "$lib/common/render";

  type ProgressProps = {
    id?: string | null;
    label?: string | null;
    value?: number | null;
    max?: number | null;
    warn?: number | null;
    crit?: number | null;
  };

  const { props }: BaseComponentProps<ProgressProps> = $props();

  const max = $derived(
    typeof props.max === "number" && props.max > 0 ? props.max : 100,
  );
  const value = $derived(
    typeof props.value === "number"
      ? Math.max(0, Math.min(props.value, max))
      : 0,
  );
  const pct = $derived(Math.round((value / max) * 100));

  // Threshold coloring: at/above crit = critical, at/above warn = warning,
  // otherwise ok. Thresholds are in the same units as value/max.
  const level = $derived.by(() => {
    if (typeof props.crit === "number" && value >= props.crit) {
      return "crit";
    }
    if (typeof props.warn === "number" && value >= props.warn) {
      return "warn";
    }
    return "ok";
  });
</script>

<div
  class="jr-progress"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_PROGRESS }}
>
  <div class="jr-progress__head">
    <span class="jr-progress__label">{props.label ?? ""}</span>
    <span class="jr-progress__value">{value} / {max}</span>
  </div>
  <div
    class="jr-progress__track"
    role="progressbar"
    aria-valuenow={value}
    aria-valuemin={0}
    aria-valuemax={max}
  >
    <div
      class="jr-progress__fill jr-progress__fill--{level}"
      style="width: {pct}%"
    ></div>
  </div>
</div>

<style>
  .jr-progress {
    min-width: 0;
    max-width: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .jr-progress__head {
    display: flex;
    min-width: 0;
    justify-content: space-between;
    align-items: baseline;
  }

  .jr-progress__label {
    min-width: 0;
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--muted);
  }

  .jr-progress__value {
    min-width: 0;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    overflow-wrap: anywhere;
  }

  .jr-progress__track {
    height: var(--space-2);
    border-radius: var(--radius-sm);
    overflow: hidden;
    background: var(--panel-2);
  }

  .jr-progress__fill {
    height: 100%;
  }

  .jr-progress__fill--ok {
    background: var(--ok);
  }

  .jr-progress__fill--warn {
    background: var(--warn);
  }

  .jr-progress__fill--crit {
    background: var(--crit);
  }
</style>
