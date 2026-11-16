<script lang="ts">
  import { untrack } from "svelte";
  import { TESTID_THINKING_BLOCK } from "#lib/common/test-ids.js";
  import {
    THINKING_LIVE,
    THINKING_DONE,
    thinkingDuration,
  } from "#lib/common/labels.js";
  import Markdown from "#lib/components/markdown/Markdown.svelte";
  import Icon from "#lib/components/ui/Icon.svelte";

  interface Props {
    text: string;
    streaming: boolean;
  }

  const { text, streaming }: Props = $props();

  // Open while the model is thinking so the reasoning is visible as it
  // arrives, then fold away once it is done so the answer leads. A block
  // loaded from history starts folded. A manual toggle wins from then on.
  let open = $state(untrack(() => streaming));
  let touched = false;

  // The duration is measured on this client: from the first render while
  // streaming to the moment streaming stops. History has no timing, so a
  // reloaded block shows the plain "Thought" label.
  let startedAt: number | null = untrack(() =>
    streaming ? performance.now() : null,
  );
  let seconds = $state<number | null>(null);
  let bodyEl: HTMLDivElement | undefined = $state();

  $effect(() => {
    if (streaming) {
      startedAt ??= performance.now();

      return;
    }

    if (startedAt !== null && seconds === null) {
      seconds = (performance.now() - startedAt) / 1000;
      if (!touched) {
        open = false;
      }
    }
  });

  // Keep the newest reasoning in view while it streams.
  $effect(() => {
    void text;
    if (streaming && open && bodyEl !== undefined) {
      bodyEl.scrollTop = bodyEl.scrollHeight;
    }
  });

  function toggle(): void {
    touched = true;
    open = !open;
  }

  const label = $derived(
    streaming
      ? THINKING_LIVE
      : seconds === null
        ? THINKING_DONE
        : thinkingDuration(seconds),
  );
</script>

<div
  class="thinking"
  data-testid={TESTID_THINKING_BLOCK}
  data-streaming={streaming}
>
  <button
    class="thinking__head"
    type="button"
    onclick={toggle}
    aria-expanded={open}
  >
    <span class="thinking__icon" class:thinking__icon--live={streaming}>
      <Icon name="brain" size={14} />
    </span>
    <span class="thinking__label" class:shimmer={streaming}>{label}</span>
    <span class="thinking__chevron" class:thinking__chevron--open={open}>
      <Icon name="chevron-right" size={14} />
    </span>
  </button>

  {#if open}
    <div class="thinking__body" bind:this={bodyEl}>
      <Markdown md={text} />
    </div>
  {/if}
</div>

<style>
  .thinking {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: var(--space-2);
    min-width: 0;
    max-width: 100%;
  }

  .thinking__head {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-2) var(--space-1) var(--space-1);
    margin-left: calc(-1 * var(--space-1));
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--muted);
  }

  .thinking__head:hover:not(:disabled) {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .thinking__icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--muted);
  }

  .thinking__icon--live {
    color: var(--accent);
    border-color: var(--accent-soft);
  }

  .thinking__chevron {
    display: inline-flex;
    color: var(--faint);
    transition: transform var(--dur) var(--ease);
  }

  .thinking__chevron--open {
    transform: rotate(90deg);
  }

  .thinking__body {
    width: 100%;
    max-height: min(40vh, 320px);
    overflow-y: auto;
    padding: var(--space-1) 0 var(--space-1) var(--space-4);
    border-left: 2px solid var(--border);
    color: var(--muted);
    font-size: var(--text-sm);
    line-height: 1.6;
    animation: fade-in var(--dur) var(--ease);
  }

  .thinking__body :global(p) {
    margin: 0 0 var(--space-2);
  }

  .thinking__body :global(p:last-child) {
    margin-bottom: 0;
  }
</style>
