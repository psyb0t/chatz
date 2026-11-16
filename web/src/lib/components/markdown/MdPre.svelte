<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "#lib/components/ui/Icon.svelte";
  import { A11Y_COPY_CODE, LABEL_COPIED } from "#lib/common/labels.js";

  // Fenced code block wrapper. The frame (fill + radius) lives on the
  // wrapper; the inner <code> (rendered by MdCode) drops its own inline chip
  // styling here via the descendant rule so the block shows a single frame.
  // Long lines scroll horizontally inside the block, never widening the page.
  interface Props {
    children?: Snippet;
  }

  const { children }: Props = $props();

  const COPIED_RESET_MS = 1500;
  let preEl: HTMLPreElement | undefined = $state();
  let copied = $state(false);

  async function copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(preEl?.textContent ?? "");
      copied = true;
      setTimeout(() => (copied = false), COPIED_RESET_MS);
    } catch {
      // Clipboard access can be refused; the button then does nothing.
    }
  }
</script>

<div class="md-pre-wrap">
  <button
    class="md-pre__copy"
    type="button"
    onclick={copy}
    aria-label={A11Y_COPY_CODE}
    title={copied ? LABEL_COPIED : A11Y_COPY_CODE}
  >
    <Icon name={copied ? "check" : "copy"} size={13} />
  </button>
  <pre
    class="md-pre"
    bind:this={preEl}>{#if children}{@render children()}{/if}</pre>
</div>

<style>
  .md-pre-wrap {
    position: relative;
    min-width: 0;
    max-width: 100%;
    margin: var(--space-2) 0;
  }

  .md-pre {
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    line-height: 1.6;
    background: var(--code-bg);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    padding: var(--space-3) var(--space-4);
    overflow-x: auto;
    white-space: pre;
  }

  .md-pre :global(code) {
    border: 0;
    background: transparent;
    padding: 0;
    font-size: inherit;
  }

  .md-pre__copy {
    position: absolute;
    top: var(--space-2);
    right: var(--space-2);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: var(--control-sm);
    height: var(--control-sm);
    padding: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--surface);
    color: var(--muted);
    opacity: 0;
    transition: opacity var(--dur-fast) var(--ease);
  }

  .md-pre-wrap:hover .md-pre__copy,
  .md-pre__copy:focus-visible {
    opacity: 1;
  }

  .md-pre__copy:hover:not(:disabled) {
    background: var(--panel-2);
    color: var(--ink);
  }

  @media (hover: none) {
    .md-pre__copy {
      opacity: 1;
    }
  }
</style>
