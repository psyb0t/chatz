<script lang="ts">
  import type { Snippet } from "svelte";

  // Fenced code block wrapper. The brutalist frame (border + panel fill) lives
  // on the <pre>; the inner <code> (rendered by MdCode) drops its own inline
  // border here via the descendant rule so the block shows a single frame. Long
  // lines scroll horizontally inside the block, never widening the page.
  interface Props {
    children?: Snippet;
  }

  const { children }: Props = $props();
</script>

<pre class="md-pre">{#if children}{@render children()}{/if}</pre>

<style>
  .md-pre {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    background: var(--panel-2);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    padding: var(--space-3);
    overflow-x: auto;
    white-space: pre;
  }

  .md-pre :global(code) {
    border: 0;
    background: transparent;
    padding: 0;
  }
</style>
