<script lang="ts">
  import { safeHref } from "#lib/common/safe-url.js";
  import { MD_IMAGE_LABEL } from "#lib/common/labels.js";

  // Images in model output are never loaded on their own: an image URL can
  // carry chat data to a third party the moment it renders, and a
  // prompt-injected model (through a tool result, say) can write one. The
  // image is shown as a link the reader can choose to open instead.
  interface Props {
    src?: string;
    alt?: string;
    title?: string;
  }

  const { src, alt, title }: Props = $props();

  const safe = $derived(safeHref(src));
  const label = $derived(
    alt !== undefined && alt.trim() !== "" ? alt : MD_IMAGE_LABEL,
  );
</script>

{#if safe !== undefined}
  <a
    class="md-image"
    href={safe}
    title={title ?? safe}
    target="_blank"
    rel="noopener noreferrer nofollow">[{label}]</a
  >
{:else}
  <span class="md-image md-image--dead">[{label}]</span>
{/if}

<style>
  .md-image {
    color: var(--accent);
    word-break: break-word;
  }

  .md-image--dead {
    color: var(--muted);
  }
</style>
