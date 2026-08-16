<script lang="ts">
  import type { Snippet } from "svelte";

  // Inline `code` spans and the <code> inside fenced blocks both route here.
  // Mono + a hairline brutalist border + panel fill; block-level layout is
  // handled by the surrounding <pre> (MdPre), so this stays inline-safe.
  interface Props {
    children?: Snippet;
  }

  const { children }: Props = $props();
</script>

<code class="md-code"
  >{#if children}{@render children()}{/if}</code
>

<style>
  .md-code {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    background: var(--panel-2);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-sm);
    padding: 0.1em var(--space-1);
  }
</style>
