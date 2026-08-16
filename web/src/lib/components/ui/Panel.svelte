<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    // `shadow` lifts the panel with the hard brutalist drop-shadow (used by the
    // auth gate cards). `pad` picks the interior padding step from the spacing
    // scale — never a raw px value.
    shadow?: boolean;
    pad?: "md" | "lg";
    id?: string;
    testid?: string;
    children: Snippet;
  }

  const { shadow = false, pad = "md", id, testid, children }: Props = $props();
</script>

<div
  class="panel panel--pad-{pad}"
  class:panel--shadow={shadow}
  {id}
  data-testid={testid}
>
  {@render children()}
</div>

<style>
  .panel {
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--panel);
  }

  .panel--pad-md {
    padding: var(--space-4);
  }

  .panel--pad-lg {
    padding: var(--space-8);
  }

  .panel--shadow {
    box-shadow: var(--shadow);
  }
</style>
