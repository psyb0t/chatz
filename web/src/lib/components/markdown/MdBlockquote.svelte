<script lang="ts">
  import type { Snippet } from "svelte";

  // Blockquote with a left rule in muted text, tokens only.
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
    margin: var(--space-2) 0;
    border-left: 3px solid var(--border-strong);
    padding: var(--space-1) var(--space-4);
    color: var(--muted);
  }
</style>
