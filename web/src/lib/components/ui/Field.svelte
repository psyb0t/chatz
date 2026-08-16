<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    label: string;
    // Optional inline error text rendered under the control.
    error?: string | null;
    errorTestid?: string;
    // The form control (input / textarea / select), supplied by the caller so
    // Field stays agnostic to the control type + its bindings.
    control: Snippet;
  }

  const { label, error = null, errorTestid, control }: Props = $props();
</script>

<label class="field">
  <span class="field__label">{label}</span>
  {@render control()}
  {#if error}
    <span class="field__error" role="alert" data-testid={errorTestid}>
      {error}
    </span>
  {/if}
</label>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .field__label {
    font-family: var(--font-display);
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--muted);
  }

  .field__error {
    border: var(--border-width) solid var(--crit);
    border-radius: var(--radius);
    color: var(--crit);
    background: var(--bg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--text-sm);
  }
</style>
