<script lang="ts">
  import type { Snippet } from "svelte";
  import { BUTTON_DEFAULT, type ButtonVariant } from "./variants";

  interface Props {
    variant?: ButtonVariant;
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    id?: string;
    testid?: string;
    // ariaLabel gives a stable accessible name for buttons whose visible text
    // changes to a non-descriptive glyph mid-action (e.g. a spinner "…").
    ariaLabel?: string;
    onclick?: (event: MouseEvent) => void;
    children: Snippet;
  }

  const {
    variant = BUTTON_DEFAULT,
    type = "button",
    disabled = false,
    id,
    testid,
    ariaLabel,
    onclick,
    children,
  }: Props = $props();
</script>

<button
  class="btn btn--{variant}"
  {type}
  {disabled}
  {id}
  data-testid={testid}
  aria-label={ariaLabel}
  {onclick}
>
  {@render children()}
</button>

<style>
  .btn {
    background: var(--panel);
    color: var(--ink);
    border: var(--border-width) solid var(--border-strong);
    border-radius: var(--radius);
    padding: var(--space-2) var(--space-4);
    font-family: var(--font-display);
    font-weight: 500;
    transition:
      background-color 0.12s ease,
      border-color 0.12s ease,
      color 0.12s ease;
  }

  .btn:hover:not(:disabled) {
    background: var(--panel-2);
  }

  .btn:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .btn--primary {
    background: var(--accent);
    color: var(--on-accent);
    border-color: transparent;
  }

  .btn--primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  .btn--danger {
    border-color: var(--crit);
    color: var(--crit);
  }

  .btn--danger:hover:not(:disabled) {
    background: var(--crit);
    border-color: var(--crit);
    color: var(--on-accent);
  }
</style>
