<script lang="ts">
  // One generation setting: a label with its hint, a switch, and while the
  // switch is on, a slider plus an exact number field. Off means the setting
  // is not sent and the provider default applies; the last value is kept so
  // switching back on restores it.
  import { SETTINGS_UNSUPPORTED } from "#lib/common/labels.js";

  interface Props {
    label: string;
    hint: string;
    enabled: boolean;
    value: number;
    min: number;
    max: number;
    step: number;
    integer?: boolean;
    // A slider is pointless for wide ranges like token counts; the number
    // field alone is shown then.
    slider?: boolean;
    supported?: boolean;
  }

  let {
    label,
    hint,
    enabled = $bindable(),
    value = $bindable(),
    min,
    max,
    step,
    integer = false,
    slider = true,
    supported = true,
  }: Props = $props();

  const id = $props.id();

  function clamp(raw: number): number {
    const bounded = Math.min(Math.max(raw, min), max);

    return integer ? Math.round(bounded) : bounded;
  }

  function onNumber(event: Event): void {
    const raw = Number((event.currentTarget as HTMLInputElement).value);
    if (Number.isFinite(raw)) {
      value = clamp(raw);
    }
  }

  function onSlider(event: Event): void {
    value = clamp(Number((event.currentTarget as HTMLInputElement).value));
  }
</script>

<div
  class="param"
  class:param--off={!enabled}
  class:param--unsupported={!supported}
>
  <div class="param__head">
    <span class="param__label" title={hint}>{label}</span>
    {#if !supported}
      <span class="param__note">{SETTINGS_UNSUPPORTED}</span>
    {/if}
    <button
      id="{id}-switch"
      class="param__switch"
      type="button"
      role="switch"
      aria-checked={enabled}
      aria-label="Set {label}"
      onclick={() => (enabled = !enabled)}
    >
      <span class="param__knob"></span>
    </button>
  </div>
  <p class="param__hint">{hint}</p>

  {#if enabled}
    <div class="param__controls">
      {#if slider}
        <input
          class="param__slider"
          type="range"
          {min}
          {max}
          {step}
          {value}
          aria-label="{label} slider"
          oninput={onSlider}
        />
      {/if}
      <input
        class="param__number"
        class:param__number--wide={!slider}
        type="number"
        {min}
        {max}
        {step}
        {value}
        aria-label={label}
        onchange={onNumber}
      />
    </div>
  {/if}
</div>

<style>
  .param {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--space-2) 0;
  }

  .param__head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .param__label {
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--ink);
    cursor: help;
  }

  .param--off .param__label {
    color: var(--muted);
  }

  .param--unsupported .param__label {
    color: var(--disabled);
  }

  .param__note {
    color: var(--faint);
    font-size: 0.6875rem;
  }

  .param__hint {
    color: var(--faint);
    font-size: var(--text-xs);
    line-height: 1.4;
  }

  .param__switch {
    position: relative;
    flex-shrink: 0;
    width: 30px;
    height: 18px;
    margin-left: auto;
    padding: 0;
    border: none;
    border-radius: var(--radius-pill);
    background: var(--border-strong);
    transition: background-color var(--dur-fast) var(--ease);
  }

  .param__switch:hover:not(:disabled) {
    background: var(--faint);
  }

  .param__switch[aria-checked="true"],
  .param__switch[aria-checked="true"]:hover:not(:disabled) {
    background: var(--primary);
  }

  .param__knob {
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--surface);
    box-shadow: var(--shadow-sm);
    transition: transform var(--dur-fast) var(--ease);
  }

  .param__switch[aria-checked="true"] .param__knob {
    transform: translateX(12px);
  }

  .param__controls {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }

  .param__slider {
    flex: 1;
    min-width: 0;
    height: 4px;
    padding: 0;
    border: none;
    border-radius: var(--radius-pill);
    accent-color: var(--primary);
    cursor: pointer;
  }

  .param__slider:focus {
    box-shadow: none;
  }

  .param__number {
    width: 5.5rem;
    padding: 4px var(--space-2);
    border-radius: var(--radius-sm);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  .param__number--wide {
    width: 9rem;
  }
</style>
