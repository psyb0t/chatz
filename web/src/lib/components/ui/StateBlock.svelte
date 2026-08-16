<script lang="ts">
  import type { Snippet } from "svelte";
  import { STATE_ERROR, STATE_LOADING, type StateVariant } from "./variants";

  interface Props {
    // variant drives the border/text color: loading + empty read as muted
    // scaffolding; error reads as critical. Defaults to loading.
    variant?: StateVariant;
    // label is the primary line (e.g. "NO USERS", "LOADING…", an error
    // message). Rendered in the display face, uppercased by the caller's copy.
    label: string;
    id?: string;
    testid?: string;
    // actions is an optional snippet for affordances (dismiss / retry buttons)
    // rendered below the label — used by the chat error banner.
    actions?: Snippet;
  }

  const {
    variant = STATE_LOADING,
    label,
    id,
    testid,
    actions,
  }: Props = $props();
</script>

<div
  class="state state--{variant}"
  class:state--pulse={variant === STATE_LOADING}
  role={variant === STATE_ERROR ? "alert" : "status"}
  {id}
  data-testid={testid}
>
  <span class="state__label">{label}</span>
  {#if actions}
    <span class="state__actions">{@render actions()}</span>
  {/if}
</div>

<style>
  .state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    border: var(--border-width) dashed var(--border-strong);
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-4);
    text-align: center;
  }

  .state__label {
    font-family: var(--font-display);
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--muted);
    word-break: break-word;
  }

  .state--error {
    border-style: solid;
    border-color: var(--crit);
  }

  .state--error .state__label {
    color: var(--crit);
    font-family: var(--font-mono);
  }

  .state__actions {
    display: flex;
    gap: var(--space-2);
  }

  .state--pulse {
    animation: state-pulse 1s steps(2) infinite;
  }

  @keyframes state-pulse {
    50% {
      opacity: 0.5;
    }
  }
</style>
