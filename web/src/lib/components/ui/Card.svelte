<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    title?: string | null;
    description?: string | null;
    id?: string;
    testid?: string;
    // Extra root attributes (e.g. the json-render data-jr-type stamp).
    rootAttrs?: Record<string, string>;
    children?: Snippet;
  }

  const {
    title = null,
    description = null,
    id,
    testid,
    rootAttrs = {},
    children,
  }: Props = $props();

  const hasHeader = $derived(Boolean(title) || Boolean(description));
</script>

<section class="card" {id} data-testid={testid} {...rootAttrs}>
  {#if hasHeader}
    <header class="card__head">
      {#if title}
        <div class="card__title">{title}</div>
      {/if}
      {#if description}
        <div class="card__desc">{description}</div>
      {/if}
    </header>
  {/if}
  {#if children}
    <div class="card__body">
      {@render children()}
    </div>
  {/if}
</section>

<style>
  .card {
    min-width: 0;
    max-width: 100%;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--panel);
    box-shadow: var(--shadow-sm);
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .card__head {
    display: flex;
    flex-direction: column;
    min-width: 0;
    gap: var(--space-1);
    border-bottom: var(--border-width) solid var(--border);
    padding-bottom: var(--space-3);
  }

  .card__title {
    overflow-wrap: anywhere;
    font-family: var(--font-display);
    font-weight: 600;
    font-size: var(--text-lg);
  }

  .card__desc {
    overflow-wrap: anywhere;
    font-size: var(--text-sm);
    color: var(--muted);
  }

  .card__body {
    display: flex;
    flex-direction: column;
    min-width: 0;
    max-width: 100%;
    gap: var(--space-3);
  }
</style>
