<script lang="ts">
  import type { Snippet } from "svelte";

  // Blockquote with a heavy brutalist left rule + panel fill, tokens only.
  interface Props {
    children?: Snippet;
  }

  const { children }: Props = $props();
</script>

<blockquote class="md-quote">
  {#if children}{@render children()}{/if}
</blockquote>

<style>
  .md-quote {
    border-left: var(--space-1) solid var(--border);
    background: var(--panel);
    padding: var(--space-2) var(--space-3);
    color: var(--muted);
    font-style: italic;
  }
</style>
