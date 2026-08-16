<script lang="ts">
  import type { Snippet } from "svelte";

  // svelte-exmarkdown passes the hast node's `properties` (href, title, …) as
  // props and the element's contents as a `children` snippet. Links open in a
  // new tab; rel hardening blocks tabnabbing + referrer leak for LLM-authored
  // URLs. Accent color comes from the design tokens (app.css `a`).
  interface Props {
    href?: string;
    title?: string;
    children?: Snippet;
  }

  const { href, title, children }: Props = $props();
</script>

<a
  class="md-link"
  {href}
  {title}
  target="_blank"
  rel="noopener noreferrer nofollow"
>
  {#if children}{@render children()}{/if}
</a>

<style>
  .md-link {
    color: var(--accent);
    text-decoration: underline;
    word-break: break-word;
  }
</style>
