<script lang="ts">
  import {
    A11Y_MODEL,
    A11Y_MODEL_SEARCH,
    MODEL_PICKER_SEARCH_PLACEHOLDER,
    MODEL_PICKER_EMPTY,
    MODEL_PICKER_DEFAULT,
  } from "$lib/common/labels";
  import {
    TESTID_MODEL_PICKER,
    TESTID_MODEL_PICKER_SEARCH,
  } from "$lib/common/test-ids";
  import { clampToViewport } from "$lib/actions/clampToViewport";

  interface ModelOption {
    id: string;
    alias?: string;
    default: boolean;
  }

  interface Props {
    models: ModelOption[];
    value: string;
    disabled?: boolean;
  }

  let { models, value = $bindable(""), disabled = false }: Props = $props();

  let open = $state(false);
  let query = $state("");
  let activeIndex = $state(0);
  let rootEl: HTMLDivElement | undefined = $state();
  let searchEl: HTMLInputElement | undefined = $state();

  const filtered = $derived.by(() => {
    const needle = query.trim().toLowerCase();
    if (needle === "") {
      return models;
    }

    return models.filter(
      (model) =>
        model.id.toLowerCase().includes(needle) ||
        model.alias?.toLowerCase().includes(needle) === true,
    );
  });

  const selected = $derived(models.find((model) => model.id === value));
  const selectedLabel = $derived(selected?.alias ?? (value || A11Y_MODEL));

  function optionLabel(option: ModelOption): string {
    return option.alias ?? option.id;
  }

  function openPanel(): void {
    if (disabled) {
      return;
    }

    query = "";
    activeIndex = 0;
    open = true;
  }

  function closePanel(): void {
    open = false;
  }

  function toggle(): void {
    if (open) {
      closePanel();
    } else {
      openPanel();
    }
  }

  function pick(id: string): void {
    value = id;
    closePanel();
  }

  // Focus the search field as the panel opens so typing filters immediately.
  $effect(() => {
    if (open && searchEl !== undefined) {
      searchEl.focus();
    }
  });

  // Keep the highlighted row within the current filtered range.
  $effect(() => {
    if (activeIndex > filtered.length - 1) {
      activeIndex = Math.max(0, filtered.length - 1);
    }
  });

  function onSearchKeydown(event: KeyboardEvent): void {
    if (event.key === "Escape") {
      event.preventDefault();
      closePanel();

      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      activeIndex = Math.min(activeIndex + 1, filtered.length - 1);

      return;
    }

    if (event.key === "ArrowUp") {
      event.preventDefault();
      activeIndex = Math.max(activeIndex - 1, 0);

      return;
    }

    if (event.key === "Enter") {
      event.preventDefault();
      const option = filtered[activeIndex];
      if (option !== undefined) {
        pick(option.id);
      }
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

<svelte:window onpointerdown={onWindowPointerDown} />

<div class="picker" bind:this={rootEl}>
  <button
    class="picker__trigger"
    type="button"
    id="composer-model"
    aria-label={A11Y_MODEL}
    aria-haspopup="listbox"
    aria-expanded={open}
    {disabled}
    onclick={toggle}
    data-testid={TESTID_MODEL_PICKER}
  >
    <span class="picker__value">{selectedLabel}</span>
    <span class="picker__caret" aria-hidden="true">▾</span>
  </button>

  {#if open}
    <div
      class="picker__panel"
      role="listbox"
      aria-label={A11Y_MODEL}
      use:clampToViewport
    >
      <input
        class="picker__search"
        type="text"
        bind:this={searchEl}
        bind:value={query}
        placeholder={MODEL_PICKER_SEARCH_PLACEHOLDER}
        aria-label={A11Y_MODEL_SEARCH}
        onkeydown={onSearchKeydown}
        data-testid={TESTID_MODEL_PICKER_SEARCH}
      />

      <ul class="picker__list">
        {#each filtered as option, i (option.id)}
          <li>
            <button
              type="button"
              class="picker__option"
              class:picker__option--active={i === activeIndex}
              class:picker__option--selected={option.id === value}
              role="option"
              aria-selected={option.id === value}
              onclick={() => pick(option.id)}
              onmouseenter={() => (activeIndex = i)}
            >
              <span class="picker__option-label">{optionLabel(option)}</span>
              {#if option.alias !== undefined}
                <span class="picker__option-id">{option.id}</span>
              {/if}
              {#if option.default}
                <span class="picker__option-default"
                  >{MODEL_PICKER_DEFAULT}</span
                >
              {/if}
            </button>
          </li>
        {:else}
          <li class="picker__empty">{MODEL_PICKER_EMPTY}</li>
        {/each}
      </ul>
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
    gap: var(--space-1);
    max-width: 12rem;
    background: transparent;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    color: var(--ink);
    cursor: pointer;
    padding: var(--space-1) var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .picker__trigger:hover:not(:disabled) {
    background: var(--panel-2);
  }

  .picker__trigger:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .picker__value {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .picker__caret {
    color: var(--muted);
  }

  /* Opens upward — the composer sits at the bottom of the viewport. */
  .picker__panel {
    position: absolute;
    bottom: calc(100% + var(--space-1));
    left: 0;
    z-index: 20;
    width: 16rem;
    max-width: min(20rem, calc(100vw - 2rem));
    display: flex;
    flex-direction: column;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    box-shadow: var(--shadow-lg);
    overflow: hidden;
  }

  .picker__search {
    margin: var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .picker__list {
    list-style: none;
    max-height: 16rem;
    overflow-y: auto;
    padding: 0 var(--space-1) var(--space-1);
  }

  .picker__option {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    gap: var(--space-1);
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--ink);
    cursor: pointer;
    padding: var(--space-2);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .picker__option-label,
  .picker__option-id {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .picker__option-id {
    grid-column: 1;
    color: var(--muted);
    font-size: var(--text-xs);
  }

  .picker__option-default {
    grid-column: 2;
    grid-row: 1 / span 2;
    color: var(--accent);
    font-size: var(--text-xs);
  }

  .picker__option--active {
    background: var(--panel-2);
  }

  .picker__option--selected {
    color: var(--accent);
    font-weight: 600;
  }

  .picker__empty {
    color: var(--muted);
    padding: var(--space-2);
    font-size: var(--text-xs);
    text-align: center;
  }
</style>
