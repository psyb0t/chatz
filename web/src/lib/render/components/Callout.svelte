<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_CALLOUT, DATA_JR_TYPE } from "$lib/common/render";

  type CalloutProps = {
    id?: string | null;
    variant?: "info" | "warn" | "error" | null;
    title?: string | null;
    text?: string | null;
  };

  const { props }: BaseComponentProps<CalloutProps> = $props();

  const variant = $derived(
    props.variant === "warn" || props.variant === "error"
      ? props.variant
      : "info",
  );
</script>

<div
  class="jr-callout jr-callout--{variant}"
  id={props.id ?? undefined}
  role="note"
  {...{ [DATA_JR_TYPE]: COMP_CALLOUT }}
>
  {#if props.title}
    <div class="jr-callout__title">{props.title}</div>
  {/if}
  <div class="jr-callout__text">{props.text ?? ""}</div>
</div>

<style>
  .jr-callout {
    min-width: 0;
    max-width: 100%;
    border: var(--border-width) solid var(--border);
    border-left-width: 3px;
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-3) var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .jr-callout--info {
    border-left-color: var(--accent);
  }

  .jr-callout--warn {
    border-left-color: var(--warn);
  }

  .jr-callout--error {
    border-left-color: var(--crit);
  }

  .jr-callout__title {
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .jr-callout__text {
    overflow-wrap: anywhere;
    font-size: var(--text-sm);
    line-height: 1.5;
  }
</style>
