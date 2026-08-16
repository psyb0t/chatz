<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_STAT, DATA_JR_TYPE } from "$lib/common/render";

  type StatProps = {
    id?: string | null;
    label?: string | null;
    value?: string | null;
    unit?: string | null;
    delta?: number | null;
  };

  const { props }: BaseComponentProps<StatProps> = $props();

  const delta = $derived(typeof props.delta === "number" ? props.delta : null);
  // Positive delta = improvement (▲ ok), negative = decline (▼ crit).
  const deltaClass = $derived(
    delta === null
      ? ""
      : delta >= 0
        ? "jr-stat__delta--ok"
        : "jr-stat__delta--crit",
  );
  const deltaMark = $derived(delta === null ? "" : delta >= 0 ? "▲" : "▼");
</script>

<div
  class="jr-stat"
  id={props.id ?? undefined}
  {...{ [DATA_JR_TYPE]: COMP_STAT }}
>
  <div class="jr-stat__label">{props.label ?? ""}</div>
  <div class="jr-stat__value">
    <span class="jr-stat__num">{props.value ?? ""}</span>
    {#if props.unit}
      <span class="jr-stat__unit">{props.unit}</span>
    {/if}
  </div>
  {#if delta !== null}
    <div class="jr-stat__delta {deltaClass}">
      {deltaMark}
      {Math.abs(delta)}
    </div>
  {/if}
</div>

<style>
  .jr-stat {
    min-width: 0;
    max-width: 100%;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .jr-stat__label {
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-size: var(--text-xs);
    font-weight: 500;
    color: var(--muted);
  }

  .jr-stat__value {
    display: flex;
    min-width: 0;
    align-items: baseline;
    flex-wrap: wrap;
    gap: var(--space-1);
    font-family: var(--font-mono);
  }

  .jr-stat__num {
    min-width: 0;
    font-size: var(--text-2xl);
    font-weight: 600;
    line-height: 1;
    overflow-wrap: anywhere;
  }

  .jr-stat__unit {
    overflow-wrap: anywhere;
    font-size: var(--text-sm);
    color: var(--muted);
  }

  .jr-stat__delta {
    overflow-wrap: anywhere;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    font-weight: 700;
  }

  .jr-stat__delta--ok {
    color: var(--ok);
  }

  .jr-stat__delta--crit {
    color: var(--crit);
  }
</style>
