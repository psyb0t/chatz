<script lang="ts">
  import {
    A11Y_MODEL,
    A11Y_MODEL_SEARCH,
    MODEL_PICKER_SEARCH_PLACEHOLDER,
    MODEL_PICKER_EMPTY,
    MODEL_PICKER_DEFAULT,
  } from "#lib/common/labels.js";
  import {
    TESTID_MODEL_PICKER,
    TESTID_MODEL_PICKER_SEARCH,
  } from "#lib/common/test-ids.js";
  import { clampToViewport } from "#lib/actions/clampToViewport.js";
  import Icon from "#lib/components/ui/Icon.svelte";

  interface ModelOption {
    id: string;
    alias?: string;
    default: boolean;
    contextWindow?: number;
    controls?: { reasoningEfforts: readonly string[] };
  }

  const TOKENS_PER_K = 1000;

  function contextLabel(tokens: number | undefined): string {
    if (tokens === undefined) {
      return "";
    }

    return tokens >= TOKENS_PER_K * TOKENS_PER_K
      ? `${(tokens / (TOKENS_PER_K * TOKENS_PER_K)).toFixed(1).replace(/\.0$/, "")}M`
      : `${Math.round(tokens / TOKENS_PER_K)}k`;
  }

  function thinks(option: ModelOption): boolean {
    return (option.controls?.reasoningEfforts ?? []).some(
      (effort) => effort !== "none",
    );
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
    <span class="picker__caret"><Icon name="chevron-down" size={14} /></span>
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
              <span class="picker__option-tags">
                {#if thinks(option)}
                  <span class="picker__tag" title="Reasoning">
                    <Icon name="brain" size={12} />
                  </span>
                {/if}
                {#if option.contextWindow !== undefined}
                  <span class="picker__tag">
                    {contextLabel(option.contextWindow)}
                  </span>
                {/if}
                {#if option.default}
                  <span class="picker__tag picker__tag--default"
                    >{MODEL_PICKER_DEFAULT}</span
                  >
                {/if}
                {#if option.id === value}
                  <span class="picker__check"
                    ><Icon name="check" size={14} /></span
                  >
                {/if}
              </span>
              {#if option.alias !== undefined}
                <span class="picker__option-id">{option.id}</span>
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
    min-width: 0;
  }

  .picker__trigger {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
    max-width: 14rem;
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
    display: inline-flex;
    color: var(--faint);
  }

  /* Opens upward and right-aligned: the picker sits at the bottom right of the
     composer. */
  .picker__panel {
    position: absolute;
    bottom: calc(100% + var(--space-2));
    right: 0;
    z-index: 20;
    width: 20rem;
    max-width: calc(100vw - 2rem);
    display: flex;
    flex-direction: column;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-lg);
    overflow: hidden;
    animation: fade-in var(--dur-fast) var(--ease);
  }

  .picker__search {
    margin: var(--space-2);
    border-radius: var(--radius);
    font-size: var(--text-sm);
  }

  .picker__list {
    list-style: none;
    max-height: 18rem;
    overflow-y: auto;
    padding: 0 var(--space-1) var(--space-1);
  }

  .picker__option {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    column-gap: var(--space-2);
    width: 100%;
    text-align: left;
    background: transparent;
    border: none;
    border-radius: var(--radius);
    color: var(--ink);
    padding: var(--space-2);
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .picker__option:hover:not(:disabled) {
    background: transparent;
  }

  .picker__option-label,
  .picker__option-id {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .picker__option-id {
    grid-column: 1;
    color: var(--faint);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 400;
  }

  .picker__option-tags {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    grid-column: 2;
    grid-row: 1 / span 2;
  }

  .picker__tag {
    display: inline-flex;
    align-items: center;
    padding: 1px 6px;
    border-radius: var(--radius-pill);
    background: var(--panel-2);
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 0.6875rem;
    font-weight: 400;
  }

  .picker__tag--default {
    background: var(--accent-soft);
    color: var(--accent);
  }

  .picker__check {
    display: inline-flex;
    color: var(--ink);
  }

  .picker__option--active {
    background: var(--fill-hover) !important;
  }

  .picker__empty {
    color: var(--muted);
    padding: var(--space-3);
    font-size: var(--text-sm);
    text-align: center;
  }
</style>
