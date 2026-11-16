<script lang="ts">
  import Icon from "#lib/components/ui/Icon.svelte";
  import { chatMcpServers } from "#lib/stores/chatMcpServers.svelte.js";
  import { mcpStatusLabel } from "#lib/common/mcp.js";
  import { clampToViewport } from "#lib/actions/clampToViewport.js";
  import {
    A11Y_CHAT_MCP,
    CHAT_MCP_TITLE,
    CHAT_MCP_TRIGGER,
    CHAT_MCP_EMPTY,
    LABEL_CLOSE,
  } from "#lib/common/labels.js";
  import {
    TESTID_CHAT_MCP_TOGGLE,
    TESTID_CHAT_MCP_PANEL,
    TESTID_CHAT_MCP_ITEM,
    TESTID_CHAT_MCP_ITEM_TOGGLE,
  } from "#lib/common/test-ids.js";

  interface Props {
    chatId: string;
  }

  const { chatId }: Props = $props();

  // Poll cadence while the popup is open and a server's status is still
  // settling — matches the admin MCP page.
  const POLL_INTERVAL_MS = 2000;

  let open = $state(false);
  let rootEl: HTMLDivElement | undefined = $state();

  // Load whenever the active chat changes (tracks chatId; runs on mount too).
  $effect(() => {
    const id = chatId;
    void chatMcpServers.load(id);
  });

  // While open AND any server could still change, poll refresh() so a silent
  // connect/recovery surfaces without reopening the popup.
  $effect(() => {
    if (!open || !chatMcpServers.anySettling) {
      return;
    }

    const iv = setInterval(() => {
      void chatMcpServers.refresh();
    }, POLL_INTERVAL_MS);

    return () => clearInterval(iv);
  });

  function openPanel(): void {
    open = true;
    // Re-fetch on open so the shown status is fresh, not last-seen.
    void chatMcpServers.refresh();
  }

  function closePanel(): void {
    open = false;
  }

  function toggle(): void {
    if (open) {
      closePanel();

      return;
    }

    openPanel();
  }

  function onToggle(serverId: string, enabled: boolean): void {
    void chatMcpServers.setEnabled(serverId, enabled);
  }

  function onWindowKeydown(event: KeyboardEvent): void {
    if (open && event.key === "Escape") {
      closePanel();
    }
  }

  // Close when a pointer press lands outside the picker.
  function onWindowPointerDown(event: PointerEvent): void {
    if (!open || rootEl === undefined) {
      return;
    }

    if (!rootEl.contains(event.target as Node)) {
      closePanel();
    }
  }
</script>

<svelte:window
  onpointerdown={onWindowPointerDown}
  onkeydown={onWindowKeydown}
/>

<div class="picker" bind:this={rootEl}>
  <button
    class="picker__trigger"
    type="button"
    aria-label={A11Y_CHAT_MCP}
    aria-haspopup="dialog"
    aria-expanded={open}
    onclick={toggle}
    data-testid={TESTID_CHAT_MCP_TOGGLE}
  >
    <Icon name="plug" size={15} />
    <span class="picker__value">{CHAT_MCP_TRIGGER}</span>
    <span class="picker__count">{chatMcpServers.enabledCount}</span>
  </button>

  {#if open}
    <div
      class="picker__panel"
      role="dialog"
      aria-label={CHAT_MCP_TITLE}
      data-testid={TESTID_CHAT_MCP_PANEL}
      use:clampToViewport
    >
      <div class="picker__head">
        <span class="picker__title">{CHAT_MCP_TITLE}</span>
        <button
          class="picker__close"
          type="button"
          onclick={closePanel}
          aria-label={LABEL_CLOSE}><Icon name="x" size={14} /></button
        >
      </div>

      <ul class="picker__list">
        {#each chatMcpServers.list as server (server.id)}
          <li class="picker__item" data-testid={TESTID_CHAT_MCP_ITEM}>
            <label class="picker__toggle">
              <input
                type="checkbox"
                checked={server.enabled}
                onchange={() => onToggle(server.id, !server.enabled)}
                data-testid={TESTID_CHAT_MCP_ITEM_TOGGLE}
              />
              <span class="picker__name">{server.name}</span>
            </label>
            <span class="picker__status">{mcpStatusLabel(server.status)}</span>
          </li>
        {:else}
          <li class="picker__empty">{CHAT_MCP_EMPTY}</li>
        {/each}
      </ul>

      {#if chatMcpServers.error !== null}
        <p class="picker__error">{chatMcpServers.error}</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .picker {
    position: relative;
    display: inline-flex;
  }

  .picker__trigger {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    height: var(--control-sm);
    padding: 0 var(--space-2);
    background: transparent;
    border: none;
    border-radius: var(--radius-pill);
    color: var(--muted);
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .picker__trigger:hover:not(:disabled),
  .picker__trigger[aria-expanded="true"] {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .picker__value {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .picker__count {
    min-width: 18px;
    padding: 0 5px;
    border-radius: var(--radius-pill);
    background: var(--panel-2);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 0.6875rem;
    line-height: 18px;
    text-align: center;
  }

  /* Opens upward, left-aligned under the composer's left controls. */
  .picker__panel {
    position: absolute;
    bottom: calc(100% + var(--space-2));
    left: 0;
    z-index: 20;
    width: 18rem;
    max-width: calc(100vw - 2rem);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    padding: var(--space-3);
    box-shadow: var(--shadow-lg);
    animation: fade-in var(--dur-fast) var(--ease);
  }

  .picker__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .picker__title {
    font-family: var(--font-display);
    font-weight: 600;
    font-size: var(--text-sm);
  }

  .picker__close {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    padding: 0;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--muted);
  }

  .picker__close:hover:not(:disabled) {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .picker__list {
    list-style: none;
    margin: 0;
    padding: 0;
    max-height: 16rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .picker__item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .picker__toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    cursor: pointer;
    overflow: hidden;
  }

  .picker__name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
  }

  .picker__status {
    flex-shrink: 0;
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .picker__empty {
    color: var(--muted);
    padding: var(--space-2);
    font-size: var(--text-xs);
    text-align: center;
  }

  .picker__error {
    margin: 0;
    color: var(--crit);
    font-size: var(--text-xs);
  }
</style>
