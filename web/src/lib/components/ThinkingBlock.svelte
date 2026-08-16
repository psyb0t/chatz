<script lang="ts">
  import { TESTID_THINKING_BLOCK } from "$lib/common/test-ids";

  interface Props {
    text: string;
    streaming: boolean;
  }

  const { text, streaming }: Props = $props();

  // Collapsed by default — only the "thinking" title (+ live spinner while
  // streaming) shows; the reasoning body is tucked away until expanded, so a
  // long chain-of-thought doesn't dominate the page.
  let open = $state(false);

  function toggle(): void {
    open = !open;
  }
</script>

<div
  class="thinking"
  data-testid={TESTID_THINKING_BLOCK}
  data-streaming={streaming}
>
  <button class="thinking__head" type="button" onclick={toggle}>
    <span class="thinking__toggle" aria-hidden="true">{open ? "▾" : "▸"}</span>
    <span class="thinking__label">thinking</span>
    {#if streaming}
      <span class="thinking__spinner" aria-hidden="true">█</span>
    {/if}
  </button>

  {#if open}
    <div class="thinking__body">
      <pre class="thinking__text">{text}</pre>
    </div>
  {/if}
</div>

<style>
  .thinking {
    border: var(--border-width) dashed var(--border-strong);
    border-radius: var(--radius);
    overflow: hidden;
    background: var(--bg);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    color: var(--muted);
  }

  .thinking__head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    text-align: left;
    padding: var(--space-2) var(--space-3);
    background: transparent;
    border: none;
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--muted);
    cursor: pointer;
  }

  .thinking__head:hover {
    background: var(--panel);
  }

  .thinking__label {
    font-weight: 600;
  }

  .thinking__spinner {
    margin-left: auto;
    animation: blink 1s steps(1) infinite;
  }

  .thinking__body {
    border-top: var(--border-width) dashed var(--border);
    padding: var(--space-2) var(--space-3);
  }

  .thinking__text {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-word;
    font-style: italic;
    overflow-x: auto;
  }

  @keyframes blink {
    50% {
      opacity: 0;
    }
  }
</style>
