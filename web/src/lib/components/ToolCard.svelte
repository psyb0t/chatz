<script lang="ts">
  import { onDestroy, untrack } from "svelte";
  import type { ToolCall } from "#lib/stores/conversation.svelte.js";
  import { TESTID_TOOL_CARD } from "#lib/common/test-ids.js";
  import {
    TOOL_ARGS,
    TOOL_RESULT,
    TOOL_ERROR,
    toolTitle,
    toolElapsed,
  } from "#lib/common/labels.js";
  import Icon from "#lib/components/ui/Icon.svelte";
  import Spinner from "#lib/components/ui/Spinner.svelte";

  interface Props {
    call: ToolCall;
  }

  const { call }: Props = $props();

  // MCP tools arrive as `<server>__<tool>`; the server is shown apart so the
  // title reads as the action.
  const TOOL_NAME_SEPARATOR = "__";
  const TICK_MS = 100;

  const nameParts = $derived.by(() => {
    const at = call.name.indexOf(TOOL_NAME_SEPARATOR);
    if (at <= 0) {
      return { server: "", tool: call.name };
    }

    return {
      server: call.name.slice(0, at),
      tool: call.name.slice(at + TOOL_NAME_SEPARATOR.length),
    };
  });

  // Collapsed by default so several calls don't turn into a wall of JSON. A
  // failed call opens itself, since the error is what the reader needs.
  let open = $state(false);
  let touched = false;

  // The timer runs on this client from first render while the call is live.
  // A call loaded already finished has no timing to show.
  let startedAt: number | null = untrack(() =>
    call.done ? null : performance.now(),
  );
  let elapsedMs = $state<number | null>(null);
  let timer: ReturnType<typeof setInterval> | null = null;

  function stopTimer(): void {
    if (timer !== null) {
      clearInterval(timer);
      timer = null;
    }
  }

  $effect(() => {
    if (!call.done) {
      startedAt ??= performance.now();
      if (timer === null) {
        timer = setInterval(() => {
          if (startedAt !== null) {
            elapsedMs = performance.now() - startedAt;
          }
        }, TICK_MS);
      }

      return;
    }

    stopTimer();
    if (startedAt !== null) {
      elapsedMs = performance.now() - startedAt;
    }

    if (call.isError && !touched) {
      open = true;
    }
  });

  onDestroy(stopTimer);

  function toggle(): void {
    touched = true;
    open = !open;
  }

  function pretty(raw: string): string {
    try {
      return JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      return raw;
    }
  }
</script>

<div
  class="tool"
  class:tool--error={call.isError}
  data-testid={TESTID_TOOL_CARD}
  data-tool={call.name}
  data-done={call.done}
>
  <button
    class="tool__head"
    type="button"
    onclick={toggle}
    aria-expanded={open}
  >
    <span
      class="tool__status"
      class:tool__status--ok={call.done && !call.isError}
      class:tool__status--error={call.isError}
    >
      {#if !call.done}
        <Spinner size={12} />
      {:else if call.isError}
        <Icon name="x" size={13} strokeWidth={2.2} />
      {:else}
        <Icon name="check" size={13} strokeWidth={2.2} />
      {/if}
    </span>
    <span class="tool__title" class:shimmer={!call.done}>
      {toolTitle(nameParts.tool, call.done, call.isError)}
    </span>
    {#if nameParts.server !== ""}
      <span class="tool__server">
        <Icon name="plug" size={12} />
        {nameParts.server}
      </span>
    {/if}
    <span class="tool__grow"></span>
    {#if elapsedMs !== null}
      <span class="tool__time">{toolElapsed(elapsedMs)}</span>
    {/if}
    <span class="tool__chevron" class:tool__chevron--open={open}>
      <Icon name="chevron-right" size={14} />
    </span>
  </button>

  {#if open}
    <div class="tool__body">
      <div class="tool__label">{TOOL_ARGS}</div>
      <pre class="tool__code">{pretty(call.args || "{}")}</pre>
      {#if call.result !== null}
        <div class="tool__label">{call.isError ? TOOL_ERROR : TOOL_RESULT}</div>
        <pre class="tool__code">{pretty(call.result)}</pre>
      {/if}
    </div>
  {/if}
</div>

<style>
  .tool {
    min-width: 0;
    max-width: 100%;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    overflow: hidden;
  }

  .tool--error {
    border-color: color-mix(in srgb, var(--crit) 40%, var(--border));
  }

  .tool__head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    min-width: 0;
    padding: var(--space-2) var(--space-3);
    background: transparent;
    border: none;
    border-radius: 0;
    text-align: left;
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .tool__head:hover:not(:disabled) {
    background: var(--fill-hover);
    border-color: transparent;
  }

  .tool__status {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    flex-shrink: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-sm);
    color: var(--muted);
  }

  .tool__status--ok {
    color: var(--ok);
    background: var(--ok-soft);
    border-color: transparent;
  }

  .tool__status--error {
    color: var(--crit);
    background: var(--crit-soft);
    border-color: transparent;
  }

  .tool__title {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .tool__server {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    background: var(--panel-2);
    color: var(--muted);
    font-size: var(--text-xs);
    font-weight: 400;
  }

  .tool__grow {
    flex: 1;
  }

  .tool__time {
    flex-shrink: 0;
    color: var(--faint);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  .tool__chevron {
    display: inline-flex;
    flex-shrink: 0;
    color: var(--faint);
    transition: transform var(--dur) var(--ease);
  }

  .tool__chevron--open {
    transform: rotate(90deg);
  }

  .tool__body {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    border-top: var(--border-width) solid var(--border);
    animation: fade-in var(--dur) var(--ease);
  }

  .tool__label {
    color: var(--faint);
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .tool__code {
    margin: 0 0 var(--space-2);
    max-height: 320px;
    overflow: auto;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius);
    background: var(--code-bg);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1.55;
    white-space: pre-wrap;
    word-break: break-word;
  }

  .tool__code:last-child {
    margin-bottom: 0;
  }
</style>
