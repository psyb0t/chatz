<script lang="ts">
  import type {
    ChatSettings,
    Model,
    ReasoningEffort,
  } from "#lib/api/client.js";
  import { conversation } from "#lib/stores/conversation.svelte.js";
  import { clampToViewport } from "#lib/actions/clampToViewport.js";
  import Icon from "#lib/components/ui/Icon.svelte";
  import {
    A11Y_REASONING,
    SETTINGS_REASONING_AUTO,
    SETTINGS_REASONING_HINT,
    effortLabel,
  } from "#lib/common/labels.js";
  import {
    TESTID_REASONING_PICKER,
    TESTID_REASONING_OPTION,
  } from "#lib/common/test-ids.js";

  interface Props {
    model: Model | undefined;
    disabled?: boolean;
  }

  const { model, disabled = false }: Props = $props();

  const AUTO = "";

  let open = $state(false);
  let saving = $state(false);
  let rootEl: HTMLDivElement | undefined = $state();

  const efforts = $derived<readonly ReasoningEffort[]>(
    model?.controls?.reasoningEfforts ?? [],
  );
  const effortOptions = $derived<(ReasoningEffort | typeof AUTO)[]>([
    AUTO,
    ...efforts,
  ]);
  const current = $derived(conversation.settings?.reasoningEffort ?? AUTO);
  // A stored effort this model does not take is not sent, so show it as Auto.
  const shown = $derived(
    current !== AUTO && efforts.includes(current) ? current : AUTO,
  );
  const thinkingOff = $derived(shown === "none");

  async function pick(effort: ReasoningEffort | typeof AUTO): Promise<void> {
    open = false;
    if (effort === shown) {
      return;
    }

    const next: ChatSettings = { ...(conversation.settings ?? {}) };
    if (effort === AUTO) {
      delete next.reasoningEffort;
    } else {
      next.reasoningEffort = effort;
    }

    saving = true;
    try {
      await conversation.updateSettings(next);
    } finally {
      saving = false;
    }
  }

  function onWindowPointerDown(event: PointerEvent): void {
    if (
      open &&
      rootEl !== undefined &&
      !rootEl.contains(event.target as Node)
    ) {
      open = false;
    }
  }

  function onWindowKeydown(event: KeyboardEvent): void {
    if (open && event.key === "Escape") {
      open = false;
    }
  }
</script>

<svelte:window
  onpointerdown={onWindowPointerDown}
  onkeydown={onWindowKeydown}
/>

{#if efforts.length > 0}
  <div class="effort" bind:this={rootEl}>
    <button
      class="effort__trigger"
      class:effort__trigger--off={thinkingOff}
      class:effort__trigger--set={shown !== AUTO && !thinkingOff}
      type="button"
      aria-label={A11Y_REASONING}
      aria-haspopup="listbox"
      aria-expanded={open}
      title={SETTINGS_REASONING_HINT}
      disabled={disabled || saving}
      onclick={() => (open = !open)}
      data-testid={TESTID_REASONING_PICKER}
    >
      <Icon name="brain" size={15} />
      <span
        >{shown === AUTO ? SETTINGS_REASONING_AUTO : effortLabel(shown)}</span
      >
    </button>

    {#if open}
      <div
        class="effort__panel"
        role="listbox"
        aria-label={A11Y_REASONING}
        use:clampToViewport
      >
        <div class="effort__title">{A11Y_REASONING}</div>
        {#each effortOptions as effort (effort)}
          <button
            class="effort__option"
            class:effort__option--selected={effort === shown}
            type="button"
            role="option"
            aria-selected={effort === shown}
            onclick={() => pick(effort)}
            data-testid={TESTID_REASONING_OPTION}
            data-effort={effort}
          >
            <span
              >{effort === AUTO
                ? SETTINGS_REASONING_AUTO
                : effortLabel(effort)}</span
            >
            {#if effort === shown}
              <Icon name="check" size={14} />
            {/if}
          </button>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .effort {
    position: relative;
    display: inline-flex;
  }

  .effort__trigger {
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

  .effort__trigger:hover:not(:disabled),
  .effort__trigger[aria-expanded="true"] {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .effort__trigger--set {
    background: var(--accent-soft);
    color: var(--accent);
  }

  .effort__trigger--set:hover:not(:disabled) {
    background: var(--accent-soft);
    color: var(--accent-hover);
  }

  .effort__trigger--off {
    color: var(--faint);
  }

  .effort__panel {
    position: absolute;
    bottom: calc(100% + var(--space-2));
    left: 0;
    z-index: 20;
    width: 12rem;
    display: flex;
    flex-direction: column;
    padding: var(--space-1);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-lg);
    animation: fade-in var(--dur-fast) var(--ease);
  }

  .effort__title {
    padding: var(--space-2) var(--space-2) var(--space-1);
    color: var(--faint);
    font-size: var(--text-xs);
    font-weight: 500;
  }

  .effort__option {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
    padding: var(--space-2);
    background: transparent;
    border: none;
    border-radius: var(--radius);
    color: var(--ink);
    font-size: var(--text-sm);
    font-weight: 400;
    text-align: left;
  }

  .effort__option:hover:not(:disabled) {
    background: var(--fill-hover);
  }

  .effort__option--selected {
    font-weight: 600;
  }
</style>
