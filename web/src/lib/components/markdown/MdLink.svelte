<script lang="ts">
  import type { Snippet } from "svelte";
  import { safeHref } from "#lib/common/safe-url.js";

  // svelte-exmarkdown passes the hast node's `properties` (href, title, …) as
  // props and the element's contents as a `children` snippet. Links open in a
  // new tab; rel hardening blocks tabnabbing + referrer leak for LLM-authored
  // URLs. Only http, https and mailto URLs become links (safeHref); anything
  // else, such as a javascript: URL, renders as plain text. Accent color comes
  // from the design tokens (app.css `a`).
  interface Props {
    href?: string;
    title?: string;
    children?: Snippet;
  }

  const { href, title, children }: Props = $props();

  const safe = $derived(safeHref(href));
</script>

{#if safe !== undefined}
  <a
    class="md-link"
    href={safe}
    {title}
    target="_blank"
    rel="noopener noreferrer nofollow"
  >
    {#if children}{@render children()}{/if}
  </a>
{:else}
  <span class="md-link md-link--dead" {title}
    >{#if children}{@render children()}{/if}</span
  >
{/if}

<style>
  .md-link--dead {
    color: inherit;
    text-decoration: none;
  }

  .md-link {
    color: var(--accent);
    text-decoration: underline;
    word-break: break-word;
  }
</style>
