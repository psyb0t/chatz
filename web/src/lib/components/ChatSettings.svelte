<script lang="ts">
  import { untrack } from "svelte";
  import { conversation } from "#lib/stores/conversation.svelte.js";
  import type {
    ChatSettings,
    Model,
    ReasoningEffort,
  } from "#lib/api/client.js";
  import {
    deleteComposerPreset,
    COMPOSER_PRESET_NAME_MAX_LENGTH,
    isValidComposerPresetName,
    listComposerPresets,
    SAVED_COMPOSER_PRESET_ID_PREFIX,
    saveComposerPreset,
    type ComposerPreset,
  } from "#lib/common/composer-presets.js";
  import { clampToViewport } from "#lib/actions/clampToViewport.js";
  import ParamRow from "#lib/components/ParamRow.svelte";
  import Icon from "#lib/components/ui/Icon.svelte";
  import {
    TESTID_CHAT_SETTINGS_PANEL,
    TESTID_CHAT_SETTINGS_PRESET,
    TESTID_CHAT_SETTINGS_PRESET_DELETE,
    TESTID_CHAT_SETTINGS_PRESET_NAME,
    TESTID_CHAT_SETTINGS_PRESET_SAVE,
    TESTID_CHAT_SETTINGS_SYSTEM_PROMPT,
    TESTID_CHAT_SETTINGS_RESET,
    TESTID_CHAT_SETTINGS_STATUS,
  } from "#lib/common/test-ids.js";
  import {
    SETTINGS_TITLE,
    SETTINGS_SECTION_REASONING,
    SETTINGS_SECTION_INSTRUCTIONS,
    SETTINGS_SECTION_SAMPLING,
    SETTINGS_SECTION_LENGTH,
    SETTINGS_SECTION_ADVANCED,
    SETTINGS_SECTION_PRESETS,
    SETTINGS_REASONING,
    SETTINGS_REASONING_AUTO,
    SETTINGS_REASONING_HINT,
    SETTINGS_REASONING_UNSUPPORTED,
    SETTINGS_SYSTEM_PROMPT,
    SETTINGS_SYSTEM_PROMPT_PLACEHOLDER,
    SETTINGS_TEMPERATURE,
    SETTINGS_TEMPERATURE_HINT,
    SETTINGS_TOP_P,
    SETTINGS_TOP_P_HINT,
    SETTINGS_TOP_K,
    SETTINGS_TOP_K_HINT,
    SETTINGS_FREQUENCY_PENALTY,
    SETTINGS_FREQUENCY_PENALTY_HINT,
    SETTINGS_PRESENCE_PENALTY,
    SETTINGS_PRESENCE_PENALTY_HINT,
    SETTINGS_MAX_OUTPUT,
    SETTINGS_MAX_OUTPUT_HINT,
    SETTINGS_MAX_HISTORY,
    SETTINGS_MAX_HISTORY_HINT,
    SETTINGS_SEED,
    SETTINGS_SEED_HINT,
    SETTINGS_STOP,
    SETTINGS_STOP_HINT,
    SETTINGS_STOP_PLACEHOLDER,
    SETTINGS_UNSUPPORTED,
    SETTINGS_RESET,
    SETTINGS_SAVING,
    SETTINGS_SAVED,
    SETTINGS_PRESET,
    SETTINGS_PRESET_DELETE,
    SETTINGS_PRESET_HINT,
    SETTINGS_PRESET_NAME,
    SETTINGS_PRESET_NONE,
    SETTINGS_PRESET_SAVE,
    SETTINGS_PRESET_STORAGE_ERROR,
    settingsPresetNameInvalid,
    effortLabel,
    LABEL_CLOSE,
  } from "#lib/common/labels.js";

  interface Props {
    model: Model | undefined;
    onClose: () => void;
  }

  const { model, onClose }: Props = $props();

  // Bounds mirror api/api.yml; the server enforces them again.
  const TEMPERATURE = { min: 0, max: 2, step: 0.05, start: 0.7 };
  const TOP_P = { min: 0, max: 1, step: 0.05, start: 0.9 };
  const TOP_K = { min: 1, max: 1000, step: 1, start: 40 };
  const PENALTY = { min: -2, max: 2, step: 0.1, start: 0 };
  const MAX_OUTPUT = { min: 1, max: 1_000_000, step: 1, start: 4096 };
  const MAX_HISTORY = { min: 1, max: 10_000_000, step: 1000, start: 100_000 };
  const SEED = { min: 0, max: Number.MAX_SAFE_INTEGER, step: 1, start: 42 };
  const MAX_STOP_SEQUENCES = 4;
  const MAX_STOP_LENGTH = 64;
  const SYSTEM_PROMPT_MAX_LENGTH = 16000;
  const SAVE_DEBOUNCE_MS = 500;
  const SAVED_BADGE_MS = 1500;
  const AUTO = "";

  // One numeric setting's form state: whether it is sent, and its value. The
  // value survives switching off so switching back on restores it.
  interface NumberSetting {
    on: boolean;
    value: number;
  }

  function numberSetting(
    stored: number | undefined,
    start: number,
  ): NumberSetting {
    return { on: stored !== undefined, value: stored ?? start };
  }

  // The panel mounts fresh each time it opens, so a one-time read of the
  // chat's stored settings is enough.
  const initial: ChatSettings = untrack(() => conversation.settings ?? {});

  let reasoning = $state<ReasoningEffort | typeof AUTO>(
    initial.reasoningEffort ?? AUTO,
  );
  let systemPrompt = $state(initial.systemPrompt ?? "");
  let temperature = $state<NumberSetting>(
    numberSetting(initial.temperature, TEMPERATURE.start),
  );
  let topP = $state<NumberSetting>(numberSetting(initial.topP, TOP_P.start));
  let topK = $state<NumberSetting>(numberSetting(initial.topK, TOP_K.start));
  let frequencyPenalty = $state<NumberSetting>(
    numberSetting(initial.frequencyPenalty, PENALTY.start),
  );
  let presencePenalty = $state<NumberSetting>(
    numberSetting(initial.presencePenalty, PENALTY.start),
  );
  let maxOutput = $state<NumberSetting>(
    numberSetting(initial.maxOutputTokens, MAX_OUTPUT.start),
  );
  let maxHistory = $state<NumberSetting>(
    numberSetting(initial.maxHistoryTokens, MAX_HISTORY.start),
  );
  let seed = $state<NumberSetting>(numberSetting(initial.seed, SEED.start));
  let stop = $state<string[]>([...(initial.stop ?? [])]);
  let stopDraft = $state("");

  let presets = $state<readonly ComposerPreset[]>(listComposerPresets());
  let selectedPresetID = $state("");
  let presetName = $state("");

  let saving = $state(false);
  let savedVisible = $state(false);
  let errorMsg = $state<string | null>(null);

  const controls = $derived(model?.controls);
  const efforts = $derived<readonly ReasoningEffort[]>(
    controls?.reasoningEfforts ?? [],
  );
  const effortOptions = $derived<(ReasoningEffort | typeof AUTO)[]>([
    AUTO,
    ...efforts,
  ]);
  // Without a controls block (an older server) nothing is known, so nothing
  // is marked unsupported.
  const sampling = $derived(controls?.supportsSampling ?? true);
  const penalties = $derived(controls?.supportsPenalties ?? true);
  const seedOK = $derived(controls?.supportsSeed ?? true);
  const topKOK = $derived(controls?.supportsTopK ?? true);
  const stopOK = $derived(controls?.supportsStop ?? true);
  const maxOutputCap = $derived(model?.maxOutputTokens ?? MAX_OUTPUT.max);

  function build(): ChatSettings {
    const out: ChatSettings = {};

    if (reasoning !== AUTO) {
      out.reasoningEffort = reasoning;
    }

    if (systemPrompt.trim() !== "") {
      out.systemPrompt = systemPrompt;
    }

    if (temperature.on) out.temperature = temperature.value;
    if (topP.on) out.topP = topP.value;
    if (topK.on) out.topK = topK.value;
    if (frequencyPenalty.on) out.frequencyPenalty = frequencyPenalty.value;
    if (presencePenalty.on) out.presencePenalty = presencePenalty.value;
    if (maxOutput.on) out.maxOutputTokens = maxOutput.value;
    if (maxHistory.on) out.maxHistoryTokens = maxHistory.value;
    if (seed.on) out.seed = seed.value;
    if (stop.length > 0) out.stop = [...stop];

    return out;
  }

  function applySettings(settings: ChatSettings): void {
    reasoning = settings.reasoningEffort ?? AUTO;
    systemPrompt = settings.systemPrompt ?? "";
    temperature = numberSetting(settings.temperature, temperature.value);
    topP = numberSetting(settings.topP, topP.value);
    topK = numberSetting(settings.topK, topK.value);
    frequencyPenalty = numberSetting(
      settings.frequencyPenalty,
      frequencyPenalty.value,
    );
    presencePenalty = numberSetting(
      settings.presencePenalty,
      presencePenalty.value,
    );
    maxOutput = numberSetting(settings.maxOutputTokens, maxOutput.value);
    maxHistory = numberSetting(settings.maxHistoryTokens, maxHistory.value);
    seed = numberSetting(settings.seed, seed.value);
    stop = [...(settings.stop ?? [])];
  }

  // Autosave: every change is written to the chat after a short pause, so the
  // panel needs no Save button and closing it never loses an edit.
  let lastSaved = untrack(() => JSON.stringify(build()));
  let saveTimer: ReturnType<typeof setTimeout> | null = null;
  let savedTimer: ReturnType<typeof setTimeout> | null = null;

  async function persist(
    next: ChatSettings,
    serialized: string,
  ): Promise<void> {
    saving = true;
    errorMsg = null;
    try {
      await conversation.updateSettings(next);
      lastSaved = serialized;
      savedVisible = true;
      if (savedTimer !== null) {
        clearTimeout(savedTimer);
      }

      savedTimer = setTimeout(() => (savedVisible = false), SAVED_BADGE_MS);
    } catch (err) {
      errorMsg = err instanceof Error ? err.message : String(err);
    } finally {
      saving = false;
    }
  }

  $effect(() => {
    const next = build();
    const serialized = JSON.stringify(next);
    if (serialized === lastSaved) {
      return;
    }

    if (saveTimer !== null) {
      clearTimeout(saveTimer);
    }

    saveTimer = setTimeout(() => {
      saveTimer = null;
      void persist(next, serialized);
    }, SAVE_DEBOUNCE_MS);
  });

  // Closing flushes a pending edit instead of dropping it.
  $effect(() => () => {
    if (saveTimer !== null) {
      clearTimeout(saveTimer);
      const next = untrack(build);
      if (JSON.stringify(next) !== lastSaved) {
        void conversation.updateSettings(next).catch(() => undefined);
      }
    }

    if (savedTimer !== null) {
      clearTimeout(savedTimer);
    }
  });

  function addStop(): void {
    const value = stopDraft;
    if (
      value.trim() === "" ||
      value.includes("\u0000") ||
      value.length > MAX_STOP_LENGTH ||
      stop.length >= MAX_STOP_SEQUENCES ||
      stop.includes(value)
    ) {
      return;
    }

    stop = [...stop, value];
    stopDraft = "";
  }

  function onStopKeydown(event: KeyboardEvent): void {
    if (event.key === "Enter" && !event.isComposing) {
      event.preventDefault();
      addStop();
    }
  }

  function removeStop(value: string): void {
    stop = stop.filter((entry) => entry !== value);
  }

  // A visible escape for stop sequences that are whitespace, such as "\n".
  function showStop(value: string): string {
    return JSON.stringify(value).slice(1, -1);
  }

  function resetAll(): void {
    applySettings({});
    selectedPresetID = "";
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

    selectedPresetID = savedPreset.id;
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

  function onKeydown(event: KeyboardEvent): void {
    if (event.key === "Escape") {
      onClose();
    }
  }
</script>

<div
  class="settings"
  role="dialog"
  aria-label={SETTINGS_TITLE}
  tabindex="-1"
  data-testid={TESTID_CHAT_SETTINGS_PANEL}
  onkeydown={onKeydown}
  use:clampToViewport
>
  <div class="settings__head">
    <span class="settings__title">{SETTINGS_TITLE}</span>
    <span
      class="settings__status"
      data-testid={TESTID_CHAT_SETTINGS_STATUS}
      role="status"
    >
      {#if saving}
        {SETTINGS_SAVING}
      {:else if savedVisible}
        <Icon name="check" size={12} />
        {SETTINGS_SAVED}
      {/if}
    </span>
    <button
      class="settings__icon-btn"
      type="button"
      onclick={resetAll}
      title={SETTINGS_RESET}
      aria-label={SETTINGS_RESET}
      data-testid={TESTID_CHAT_SETTINGS_RESET}
    >
      <Icon name="reset" size={14} />
    </button>
    <button
      class="settings__icon-btn"
      type="button"
      onclick={onClose}
      aria-label={LABEL_CLOSE}
    >
      <Icon name="x" size={14} />
    </button>
  </div>

  <div class="settings__body">
    {#if errorMsg !== null}
      <p class="settings__error">{errorMsg}</p>
    {/if}

    <section class="settings__section">
      <h3 class="settings__section-title">{SETTINGS_SECTION_REASONING}</h3>
      {#if efforts.length > 0}
        <div
          class="settings__segmented"
          role="radiogroup"
          aria-label={SETTINGS_REASONING}
        >
          {#each effortOptions as effort (effort)}
            <button
              type="button"
              role="radio"
              aria-checked={reasoning === effort}
              class="settings__segment"
              class:settings__segment--on={reasoning === effort}
              onclick={() => (reasoning = effort)}
              >{effort === AUTO
                ? SETTINGS_REASONING_AUTO
                : effortLabel(effort)}</button
            >
          {/each}
        </div>
        <p class="settings__hint">{SETTINGS_REASONING_HINT}</p>
        {#if reasoning !== AUTO && !efforts.includes(reasoning)}
          <p class="settings__hint">{SETTINGS_UNSUPPORTED}</p>
        {/if}
      {:else}
        <p class="settings__hint">{SETTINGS_REASONING_UNSUPPORTED}</p>
      {/if}
    </section>

    <section class="settings__section">
      <h3 class="settings__section-title">{SETTINGS_SECTION_INSTRUCTIONS}</h3>
      <label class="sr-only" for="settings-system-prompt"
        >{SETTINGS_SYSTEM_PROMPT}</label
      >
      <textarea
        id="settings-system-prompt"
        class="settings__textarea"
        rows="3"
        maxlength={SYSTEM_PROMPT_MAX_LENGTH}
        placeholder={SETTINGS_SYSTEM_PROMPT_PLACEHOLDER}
        bind:value={systemPrompt}
        data-testid={TESTID_CHAT_SETTINGS_SYSTEM_PROMPT}></textarea>
    </section>

    <section class="settings__section">
      <h3 class="settings__section-title">{SETTINGS_SECTION_SAMPLING}</h3>
      <ParamRow
        label={SETTINGS_TEMPERATURE}
        hint={SETTINGS_TEMPERATURE_HINT}
        bind:enabled={temperature.on}
        bind:value={temperature.value}
        min={TEMPERATURE.min}
        max={TEMPERATURE.max}
        step={TEMPERATURE.step}
        supported={sampling}
      />
      <ParamRow
        label={SETTINGS_TOP_P}
        hint={SETTINGS_TOP_P_HINT}
        bind:enabled={topP.on}
        bind:value={topP.value}
        min={TOP_P.min}
        max={TOP_P.max}
        step={TOP_P.step}
        supported={sampling}
      />
      <ParamRow
        label={SETTINGS_TOP_K}
        hint={SETTINGS_TOP_K_HINT}
        bind:enabled={topK.on}
        bind:value={topK.value}
        min={TOP_K.min}
        max={TOP_K.max}
        step={TOP_K.step}
        integer
        supported={topKOK}
      />
      <ParamRow
        label={SETTINGS_FREQUENCY_PENALTY}
        hint={SETTINGS_FREQUENCY_PENALTY_HINT}
        bind:enabled={frequencyPenalty.on}
        bind:value={frequencyPenalty.value}
        min={PENALTY.min}
        max={PENALTY.max}
        step={PENALTY.step}
        supported={penalties}
      />
      <ParamRow
        label={SETTINGS_PRESENCE_PENALTY}
        hint={SETTINGS_PRESENCE_PENALTY_HINT}
        bind:enabled={presencePenalty.on}
        bind:value={presencePenalty.value}
        min={PENALTY.min}
        max={PENALTY.max}
        step={PENALTY.step}
        supported={penalties}
      />
    </section>

    <section class="settings__section">
      <h3 class="settings__section-title">{SETTINGS_SECTION_LENGTH}</h3>
      <ParamRow
        label={SETTINGS_MAX_OUTPUT}
        hint={SETTINGS_MAX_OUTPUT_HINT}
        bind:enabled={maxOutput.on}
        bind:value={maxOutput.value}
        min={MAX_OUTPUT.min}
        max={maxOutputCap}
        step={MAX_OUTPUT.step}
        integer
        slider={false}
      />
      <ParamRow
        label={SETTINGS_MAX_HISTORY}
        hint={SETTINGS_MAX_HISTORY_HINT}
        bind:enabled={maxHistory.on}
        bind:value={maxHistory.value}
        min={MAX_HISTORY.min}
        max={MAX_HISTORY.max}
        step={MAX_HISTORY.step}
        integer
        slider={false}
      />
    </section>

    <section class="settings__section">
      <h3 class="settings__section-title">{SETTINGS_SECTION_ADVANCED}</h3>
      <ParamRow
        label={SETTINGS_SEED}
        hint={SETTINGS_SEED_HINT}
        bind:enabled={seed.on}
        bind:value={seed.value}
        min={SEED.min}
        max={SEED.max}
        step={SEED.step}
        integer
        slider={false}
        supported={seedOK}
      />

      <div class="settings__stop" class:settings__stop--unsupported={!stopOK}>
        <label class="settings__label" for="settings-stop"
          >{SETTINGS_STOP}</label
        >
        <p class="settings__hint">
          {stopOK ? SETTINGS_STOP_HINT : SETTINGS_UNSUPPORTED}
        </p>
        {#if stop.length > 0}
          <div class="settings__chips">
            {#each stop as entry (entry)}
              <span class="settings__chip">
                <code>{showStop(entry)}</code>
                <button
                  type="button"
                  aria-label="Remove {showStop(entry)}"
                  onclick={() => removeStop(entry)}
                >
                  <Icon name="x" size={11} />
                </button>
              </span>
            {/each}
          </div>
        {/if}
        {#if stop.length < MAX_STOP_SEQUENCES}
          <input
            id="settings-stop"
            class="settings__input"
            type="text"
            maxlength={MAX_STOP_LENGTH}
            placeholder={SETTINGS_STOP_PLACEHOLDER}
            bind:value={stopDraft}
            onkeydown={onStopKeydown}
          />
        {/if}
      </div>
    </section>

    <section class="settings__section">
      <h3 class="settings__section-title">{SETTINGS_SECTION_PRESETS}</h3>
      <label class="settings__label" for="settings-preset"
        >{SETTINGS_PRESET}</label
      >
      <select
        id="settings-preset"
        class="settings__input"
        bind:value={selectedPresetID}
        onchange={applyPreset}
        data-testid={TESTID_CHAT_SETTINGS_PRESET}
      >
        <option value="">{SETTINGS_PRESET_NONE}</option>
        {#each presets as preset (preset.id)}
          <option value={preset.id}>{preset.name}</option>
        {/each}
      </select>
      <div class="settings__preset-save">
        <input
          class="settings__input settings__preset-name"
          type="text"
          maxlength={COMPOSER_PRESET_NAME_MAX_LENGTH}
          placeholder={SETTINGS_PRESET_NAME}
          bind:value={presetName}
          data-testid={TESTID_CHAT_SETTINGS_PRESET_NAME}
        />
        <button
          class="settings__btn"
          type="button"
          onclick={savePreset}
          data-testid={TESTID_CHAT_SETTINGS_PRESET_SAVE}
          >{SETTINGS_PRESET_SAVE}</button
        >
        {#if selectedPresetID.startsWith(SAVED_COMPOSER_PRESET_ID_PREFIX)}
          <button
            class="settings__btn"
            type="button"
            onclick={deletePreset}
            data-testid={TESTID_CHAT_SETTINGS_PRESET_DELETE}
            >{SETTINGS_PRESET_DELETE}</button
          >
        {/if}
      </div>
      <p class="settings__hint">{SETTINGS_PRESET_HINT}</p>
    </section>
  </div>
</div>

<style>
  /* Opens upward from the knob at the composer's left edge. */
  .settings {
    position: absolute;
    bottom: calc(100% + var(--space-2));
    left: 0;
    z-index: 30;
    width: 24rem;
    max-width: calc(100vw - 1rem);
    max-height: min(70vh, 40rem);
    display: flex;
    flex-direction: column;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
    box-shadow: var(--shadow-lg);
    overflow: hidden;
    animation: fade-in var(--dur-fast) var(--ease);
  }

  .settings:focus {
    outline: none;
  }

  .settings__head {
    display: flex;
    align-items: center;
    gap: var(--space-1);
    padding: var(--space-3) var(--space-3) var(--space-3) var(--space-4);
    border-bottom: var(--border-width) solid var(--border);
  }

  .settings__title {
    font-size: var(--text-sm);
    font-weight: 600;
  }

  .settings__status {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;
    color: var(--faint);
    font-size: var(--text-xs);
  }

  .settings__icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 26px;
    height: 26px;
    padding: 0;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--muted);
  }

  .settings__icon-btn:hover:not(:disabled) {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .settings__body {
    overflow-y: auto;
    padding: 0 var(--space-4) var(--space-4);
  }

  .settings__section {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3) 0;
    border-bottom: var(--border-width) solid var(--border);
  }

  .settings__section:last-child {
    border-bottom: none;
  }

  .settings__section-title {
    margin-bottom: var(--space-1);
    color: var(--faint);
    font-size: 0.6875rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .settings__segmented {
    display: flex;
    flex-wrap: wrap;
    gap: 2px;
    padding: 3px;
    border-radius: var(--radius);
    background: var(--panel-2);
  }

  .settings__segment {
    flex: 1 0 auto;
    padding: 4px var(--space-2);
    border: none;
    border-radius: 7px;
    background: transparent;
    color: var(--muted);
    font-size: var(--text-xs);
    font-weight: 500;
  }

  .settings__segment:hover:not(:disabled) {
    background: var(--fill-hover);
    color: var(--ink);
  }

  .settings__segment--on,
  .settings__segment--on:hover:not(:disabled) {
    background: var(--surface);
    color: var(--ink);
    box-shadow: var(--shadow-sm);
  }

  .settings__hint {
    color: var(--faint);
    font-size: var(--text-xs);
    line-height: 1.4;
  }

  .settings__label {
    font-size: var(--text-sm);
    font-weight: 500;
  }

  .settings__textarea {
    width: 100%;
    min-height: 4.5rem;
    max-height: 14rem;
    resize: vertical;
    border-radius: var(--radius);
    font-size: var(--text-sm);
    line-height: 1.5;
  }

  .settings__input {
    width: 100%;
    padding: 6px var(--space-2);
    border-radius: var(--radius-sm);
    font-size: var(--text-sm);
  }

  .settings__stop {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-2) 0;
  }

  .settings__stop--unsupported .settings__label {
    color: var(--disabled);
  }

  .settings__chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  .settings__chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 4px 2px var(--space-2);
    border-radius: var(--radius-pill);
    background: var(--panel-2);
    font-size: var(--text-xs);
  }

  .settings__chip button {
    display: inline-flex;
    padding: 2px;
    border: none;
    border-radius: 50%;
    background: transparent;
    color: var(--muted);
  }

  .settings__chip button:hover:not(:disabled) {
    background: var(--fill-active);
    color: var(--ink);
  }

  .settings__preset-save {
    display: flex;
    gap: var(--space-2);
    margin-top: var(--space-1);
  }

  .settings__preset-name {
    flex: 1;
    min-width: 0;
  }

  .settings__btn {
    flex-shrink: 0;
    padding: 4px var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--text-xs);
  }

  .settings__error {
    margin-top: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius);
    background: var(--crit-soft);
    color: var(--crit);
    font-size: var(--text-xs);
  }

  @media (max-width: 640px) {
    .settings {
      position: fixed;
      left: var(--space-2);
      right: var(--space-2);
      bottom: 6.5rem;
      width: auto;
    }
  }
</style>
