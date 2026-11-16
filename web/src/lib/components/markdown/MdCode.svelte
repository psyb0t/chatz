<script lang="ts">
  import type { Snippet } from "svelte";

  // Inline `code` spans and the <code> inside fenced blocks both route here.
  // Mono on a soft fill; block-level layout is
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
    font-size: 0.875em;
    background: var(--panel-2);
    border-radius: var(--radius-sm);
    padding: 0.15em 0.4em;
  }
</style>
