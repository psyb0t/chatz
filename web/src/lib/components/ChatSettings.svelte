<script lang="ts">
  import { conversation } from "$lib/stores/conversation.svelte";
  import type { ChatSettings, Model } from "$lib/api/client";
  import {
    deleteComposerPreset,
    COMPOSER_PRESET_NAME_MAX_LENGTH,
    isValidComposerPresetName,
    listComposerPresets,
    SAVED_COMPOSER_PRESET_ID_PREFIX,
    saveComposerPreset,
    type ComposerPreset,
  } from "$lib/common/composer-presets";
  import { clampToViewport } from "$lib/actions/clampToViewport";
  import Button from "$lib/components/ui/Button.svelte";
  import { BUTTON_PRIMARY } from "$lib/components/ui/variants";
  import {
    TESTID_CHAT_SETTINGS_PANEL,
    TESTID_CHAT_SETTINGS_PRESET,
    TESTID_CHAT_SETTINGS_PRESET_DELETE,
    TESTID_CHAT_SETTINGS_PRESET_NAME,
    TESTID_CHAT_SETTINGS_PRESET_SAVE,
  } from "$lib/common/test-ids";
  import {
    SETTINGS_TITLE,
    SETTINGS_REASONING,
    SETTINGS_REASONING_AUTO,
    SETTINGS_TEMPERATURE,
    SETTINGS_TOP_P,
    SETTINGS_MAX_OUTPUT,
    SETTINGS_MAX_HISTORY,
    SETTINGS_UNSET_HINT,
    SETTINGS_REASONING_UNSUPPORTED,
    SETTINGS_PRESET,
    SETTINGS_PRESET_DELETE,
    SETTINGS_PRESET_HINT,
    SETTINGS_PRESET_NAME,
    SETTINGS_PRESET_NONE,
    SETTINGS_PRESET_SAVE,
    SETTINGS_PRESET_STORAGE_ERROR,
    settingsPresetNameInvalid,
    LABEL_SAVE,
    LABEL_CLOSE,
  } from "$lib/common/labels";

  interface Props {
    model: Model | undefined;
    onClose: () => void;
  }

  const { model, onClose }: Props = $props();

  // The reasoning-effort tiers the API accepts (empty select value = unset).
  const REASONING_OPTIONS = ["minimal", "low", "medium", "high"] as const;
  type Reasoning = (typeof REASONING_OPTIONS)[number];

  function numToStr(n: number | undefined): string {
    return n === undefined ? "" : String(n);
  }

  // Seed the form from the chat's stored settings. The panel mounts fresh each
  // time it opens, so a one-time read is enough — no reactive resync needed.
  const initial: ChatSettings = conversation.settings ?? {};
  let reasoning = $state<Reasoning | "">(initial.reasoningEffort ?? "");
  let temperature = $state(numToStr(initial.temperature));
  let topP = $state(numToStr(initial.topP));
  let maxOutput = $state(numToStr(initial.maxOutputTokens));
  let maxHistory = $state(numToStr(initial.maxHistoryTokens));
  let presets = $state<readonly ComposerPreset[]>(listComposerPresets());
  let selectedPresetID = $state("");
  let presetName = $state("");

  let saving = $state(false);
  let errorMsg = $state<string | null>(null);
  const reasoningUnsupported = $derived(model?.supportsReasoning !== true);

  // Blank input → unset (undefined); an unparseable number is treated the same,
  // so a stray keystroke can't send NaN to the server.
  function parseFloatOrUnset(raw: string): number | undefined {
    const trimmed = raw.trim();
    if (trimmed === "") {
      return undefined;
    }

    const value = Number(trimmed);

    return Number.isFinite(value) ? value : undefined;
  }

  function parseIntOrUnset(raw: string): number | undefined {
    const value = parseFloatOrUnset(raw);

    return value === undefined ? undefined : Math.trunc(value);
  }

  function build(): ChatSettings {
    const out: ChatSettings = {};

    const t = parseFloatOrUnset(temperature);
    if (t !== undefined) {
      out.temperature = t;
    }

    const p = parseFloatOrUnset(topP);
    if (p !== undefined) {
      out.topP = p;
    }

    const mo = parseIntOrUnset(maxOutput);
    if (mo !== undefined) {
      out.maxOutputTokens = mo;
    }

    const mh = parseIntOrUnset(maxHistory);
    if (mh !== undefined) {
      out.maxHistoryTokens = mh;
    }

    if (!reasoningUnsupported && reasoning !== "") {
      out.reasoningEffort = reasoning;
    } else if (initial.reasoningEffort !== undefined) {
      out.reasoningEffort = initial.reasoningEffort;
    }

    return out;
  }

  function applyPreset(): void {
    const preset = presets.find(
      (candidate) => candidate.id === selectedPresetID,
    );
    if (preset === undefined) {
      return;
    }

    applySettings(preset.settings);
    errorMsg = null;
  }

  function savePreset(): void {
    if (!isValidComposerPresetName(presetName)) {
      errorMsg = settingsPresetNameInvalid(COMPOSER_PRESET_NAME_MAX_LENGTH);

      return;
    }

    presets = saveComposerPreset(presetName, build());
    const savedPreset = presets.find(
      (preset) =>
        !preset.builtIn &&
        preset.name.toLowerCase() === presetName.trim().toLowerCase(),
    );
    if (savedPreset === undefined) {
      errorMsg = SETTINGS_PRESET_STORAGE_ERROR;

      return;
    }

    selectedPresetID = savedPreset?.id ?? "";
    presetName = "";
    errorMsg = null;
  }

  function deletePreset(): void {
    const preset = presets.find(
      (candidate) => candidate.id === selectedPresetID,
    );
    if (preset === undefined || preset.builtIn) {
      return;
    }

    presets = deleteComposerPreset(preset.name);
    if (presets.some((candidate) => candidate.id === preset.id)) {
      errorMsg = SETTINGS_PRESET_STORAGE_ERROR;

      return;
    }

    selectedPresetID = "";
    errorMsg = null;
  }

  function applySettings(settings: ChatSettings): void {
    reasoning = settings.reasoningEffort ?? "";
    temperature = numToStr(settings.temperature);
    topP = numToStr(settings.topP);
    maxOutput = numToStr(settings.maxOutputTokens);
    maxHistory = numToStr(settings.maxHistoryTokens);
  }

  async function save(): Promise<void> {
    saving = true;
    errorMsg = null;
    try {
      await conversation.updateSettings(build());
      onClose();
    } catch (err) {
      errorMsg = err instanceof Error ? err.message : String(err);
    } finally {
      saving = false;
    }
  }
</script>

<div
  class="settings"
  data-testid={TESTID_CHAT_SETTINGS_PANEL}
  use:clampToViewport
>
  <div class="settings__head">
    <span class="settings__title">{SETTINGS_TITLE}</span>
    <button
      class="settings__close"
      type="button"
      onclick={onClose}
      aria-label={LABEL_CLOSE}>&times;</button
    >
  </div>

  <label class="settings__field">
    <span class="settings__label">{SETTINGS_PRESET}</span>
    <select
      class="settings__control"
      bind:value={selectedPresetID}
      onchange={applyPreset}
      data-testid={TESTID_CHAT_SETTINGS_PRESET}
    >
      <option value="">{SETTINGS_PRESET_NONE}</option>
      {#each presets as preset (preset.id)}
        <option value={preset.id}>{preset.name}</option>
      {/each}
    </select>
  </label>

  <div class="settings__preset-save">
    <input
      class="settings__control settings__preset-name"
      type="text"
      maxlength={COMPOSER_PRESET_NAME_MAX_LENGTH}
      placeholder={SETTINGS_PRESET_NAME}
      bind:value={presetName}
      data-testid={TESTID_CHAT_SETTINGS_PRESET_NAME}
    />
    <Button
      type="button"
      onclick={savePreset}
      testid={TESTID_CHAT_SETTINGS_PRESET_SAVE}>{SETTINGS_PRESET_SAVE}</Button
    >
    {#if selectedPresetID.startsWith(SAVED_COMPOSER_PRESET_ID_PREFIX)}
      <Button
        type="button"
        onclick={deletePreset}
        testid={TESTID_CHAT_SETTINGS_PRESET_DELETE}
        >{SETTINGS_PRESET_DELETE}</Button
      >
    {/if}
  </div>
  <p class="settings__hint">{SETTINGS_PRESET_HINT}</p>

  <label class="settings__field">
    <span class="settings__label">{SETTINGS_REASONING}</span>
    <select
      class="settings__control"
      bind:value={reasoning}
      disabled={reasoningUnsupported}
    >
      <option value="">{SETTINGS_REASONING_AUTO}</option>
      {#each REASONING_OPTIONS as option (option)}
        <option value={option}>{option}</option>
      {/each}
    </select>
  </label>

  {#if reasoningUnsupported}
    <p class="settings__hint">{SETTINGS_REASONING_UNSUPPORTED}</p>
  {/if}

  <label class="settings__field">
    <span class="settings__label">{SETTINGS_TEMPERATURE}</span>
    <input
      class="settings__control"
      type="number"
      min="0"
      max="2"
      step="0.1"
      bind:value={temperature}
    />
  </label>

  <label class="settings__field">
    <span class="settings__label">{SETTINGS_TOP_P}</span>
    <input
      class="settings__control"
      type="number"
      min="0"
      max="1"
      step="0.05"
      bind:value={topP}
    />
  </label>

  <label class="settings__field">
    <span class="settings__label">{SETTINGS_MAX_OUTPUT}</span>
    <input
      class="settings__control"
      type="number"
      min="1"
      step="1"
      bind:value={maxOutput}
    />
  </label>

  <label class="settings__field">
    <span class="settings__label">{SETTINGS_MAX_HISTORY}</span>
    <input
      class="settings__control"
      type="number"
      min="1"
      step="1"
      bind:value={maxHistory}
    />
  </label>

  <p class="settings__hint">{SETTINGS_UNSET_HINT}</p>

  {#if errorMsg !== null}
    <p class="settings__error">{errorMsg}</p>
  {/if}

  <div class="settings__actions">
    <Button
      variant={BUTTON_PRIMARY}
      type="button"
      onclick={save}
      disabled={saving}>{saving ? "…" : LABEL_SAVE}</Button
    >
  </div>
</div>

<style>
  .settings {
    position: absolute;
    bottom: calc(100% + var(--space-2));
    right: 0;
    left: auto;
    width: 22rem;
    max-width: min(24rem, calc(100vw - 2rem));
    z-index: 10;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-4);
    box-shadow: var(--shadow-lg);
  }

  .settings__head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .settings__title {
    font-family: var(--font-display);
    font-weight: 600;
    font-size: var(--text-sm);
  }

  .settings__close {
    background: transparent;
    border: none;
    color: var(--ink);
    cursor: pointer;
    font-size: var(--text-lg);
    line-height: 1;
    padding: 0 var(--space-1);
  }

  .settings__field {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .settings__preset-save {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .settings__preset-name {
    flex: 1;
    min-width: 0;
    width: auto;
  }

  .settings__label {
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .settings__control {
    width: 8rem;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .settings__hint {
    font-size: var(--text-xs);
    color: var(--muted);
  }

  .settings__error {
    font-size: var(--text-xs);
    color: var(--crit);
  }

  .settings__actions {
    display: flex;
    justify-content: flex-end;
  }
</style>
