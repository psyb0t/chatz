<script lang="ts">
  import type { ToolCall } from "$lib/stores/conversation.svelte";
  import { TESTID_TOOL_CARD } from "$lib/common/test-ids";

  interface Props {
    call: ToolCall;
  }

  const { call }: Props = $props();

  // Collapsed by default — the head (name + live status/spinner) already
  // shows what's calling and whether it's running/done/failed, so a chat
  // with several tool calls doesn't turn into a wall of expanded JSON.
  let open = $state(false);

  const status = $derived(
    call.done ? (call.isError ? "ERROR" : "DONE") : "CALLING",
  );

  function toggle(): void {
    open = !open;
  }
</script>

<div
  class="tool-card"
  class:tool-card--error={call.isError}
  data-testid={TESTID_TOOL_CARD}
  data-tool={call.name}
  data-done={call.done}
>
  <button class="tool-card__head" type="button" onclick={toggle}>
    <span class="tool-card__toggle" aria-hidden="true">{open ? "▾" : "▸"}</span>
    <span class="tool-card__name">{call.name}</span>
    <span
      class="tool-card__status"
      class:tool-card__status--pending={!call.done}
    >
      {#if !call.done}
        <span class="tool-card__spinner" aria-hidden="true">█</span>
      {/if}
      {status}
    </span>
  </button>

  {#if open}
    <div class="tool-card__body">
      <div class="tool-card__label">args</div>
      <pre class="tool-card__code">{call.args || "{}"}</pre>
      {#if call.result !== null}
        <div class="tool-card__label">{call.isError ? "error" : "result"}</div>
        <pre class="tool-card__code">{call.result}</pre>
      {/if}
    </div>
  {/if}
</div>

<style>
  .tool-card {
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
    background: var(--panel);
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .tool-card--error {
    border-color: var(--crit);
  }

  .tool-card__head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    text-align: left;
    padding: var(--space-2) var(--space-3);
    background: transparent;
    border: none;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
    cursor: pointer;
  }

  .tool-card__toggle {
    color: var(--muted);
  }

  .tool-card__head:hover {
    background: var(--panel-2);
  }

  .tool-card__name {
    font-weight: 600;
  }

  .tool-card__status {
    margin-left: auto;
    font-size: var(--text-xs);
    color: var(--accent);
    font-weight: 600;
  }

  .tool-card__status--pending {
    color: var(--warn);
  }

  .tool-card--error .tool-card__status {
    color: var(--crit);
  }

  .tool-card__spinner {
    animation: blink 1s steps(1) infinite;
  }

  .tool-card__body {
    border-top: var(--border-width) solid var(--border);
    padding: var(--space-2) var(--space-3);
  }

  .tool-card__label {
    color: var(--muted);
    font-size: var(--text-xs);
    margin-bottom: var(--space-1);
  }

  .tool-card__code {
    margin: 0 0 var(--space-2);
    white-space: pre-wrap;
    word-break: break-word;
    overflow-x: auto;
  }

  .tool-card__code:last-child {
    margin-bottom: 0;
  }

  @keyframes blink {
    50% {
      opacity: 0;
    }
  }
</style>
