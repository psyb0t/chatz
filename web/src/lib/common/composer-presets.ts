import type { ChatSettings } from "#lib/api/client.js";
import { log } from "#lib/log.js";
import { EVENT_COMPOSER_PRESET_STORAGE_ERROR } from "#lib/common/log-events.js";

const STORAGE_KEY = "chatz-composer-presets";
const MAX_SAVED_PRESETS = 12;
const STORAGE_OPERATION_READ = "read";
const STORAGE_OPERATION_WRITE = "write";
const MIN_TEMPERATURE = 0;
const MAX_TEMPERATURE = 2;
const MIN_TOP_P = 0;
const MAX_TOP_P = 1;
const MIN_TOP_K = 1;
const MAX_TOP_K = 1000;
const MIN_PENALTY = -2;
const MAX_PENALTY = 2;
const MAX_STOP_SEQUENCES = 4;
const MAX_STOP_SEQUENCE_LENGTH = 64;
const MAX_SYSTEM_PROMPT_LENGTH = 16000;
export const COMPOSER_PRESET_NAME_MAX_LENGTH = 48;
export const MAX_SAVED_COMPOSER_PRESETS = MAX_SAVED_PRESETS;
export const SAVED_COMPOSER_PRESET_ID_PREFIX = "saved-";

const REASONING_EFFORTS = new Set<NonNullable<ChatSettings["reasoningEffort"]>>(
  ["none", "minimal", "low", "medium", "high", "xhigh", "max"],
);

export const PRESET_ID_PRECISE = "built-in-precise";
export const PRESET_ID_BALANCED = "built-in-balanced";
export const PRESET_ID_CREATIVE = "built-in-creative";

export interface ComposerPreset {
  id: string;
  name: string;
  settings: ChatSettings;
  builtIn: boolean;
}

const BUILT_IN_PRESETS: readonly ComposerPreset[] = [
  {
    id: PRESET_ID_PRECISE,
    name: "Precise",
    settings: { temperature: 0.2 },
    builtIn: true,
  },
  {
    id: PRESET_ID_BALANCED,
    name: "Balanced",
    settings: { temperature: 0.7 },
    builtIn: true,
  },
  {
    id: PRESET_ID_CREATIVE,
    name: "Creative",
    settings: { temperature: 1.2 },
    builtIn: true,
  },
];

interface StoredComposerPreset {
  name: string;
  settings: ChatSettings;
}

export function listComposerPresets(): readonly ComposerPreset[] {
  return [
    ...BUILT_IN_PRESETS,
    ...readStoredPresets().map((preset) => ({
      id: savedPresetID(preset.name),
      ...preset,
      builtIn: false,
    })),
  ];
}

export function saveComposerPreset(
  name: string,
  settings: ChatSettings,
): readonly ComposerPreset[] {
  const normalizedName = normalizeName(name);
  if (normalizedName === null) {
    return listComposerPresets();
  }

  const normalizedSettings = normalizeSettings(settings);
  const existing = readStoredPresets().filter(
    (preset) => preset.name.toLowerCase() !== normalizedName.toLowerCase(),
  );
  const next = [
    ...existing.slice(-(MAX_SAVED_PRESETS - 1)),
    { name: normalizedName, settings: normalizedSettings },
  ];
  writeStoredPresets(next);

  return listComposerPresets();
}

export function deleteComposerPreset(name: string): readonly ComposerPreset[] {
  const normalizedName = normalizeName(name);
  if (normalizedName === null) {
    return listComposerPresets();
  }

  const next = readStoredPresets().filter(
    (preset) => preset.name.toLowerCase() !== normalizedName.toLowerCase(),
  );
  writeStoredPresets(next);

  return listComposerPresets();
}

export function isValidComposerPresetName(name: string): boolean {
  return normalizeName(name) !== null;
}

function readStoredPresets(): StoredComposerPreset[] {
  if (typeof localStorage === "undefined") {
    return [];
  }

  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw === null) {
      return [];
    }

    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value)) {
      return [];
    }

    return value
      .map(normalizeStoredPreset)
      .filter((preset): preset is StoredComposerPreset => preset !== null)
      .slice(-MAX_SAVED_PRESETS);
  } catch {
    log.warn(EVENT_COMPOSER_PRESET_STORAGE_ERROR, {
      operation: STORAGE_OPERATION_READ,
    });

    return [];
  }
}

function writeStoredPresets(presets: StoredComposerPreset[]): void {
  if (typeof localStorage === "undefined") {
    return;
  }

  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(presets));
  } catch {
    log.warn(EVENT_COMPOSER_PRESET_STORAGE_ERROR, {
      operation: STORAGE_OPERATION_WRITE,
    });
  }
}

function normalizeStoredPreset(value: unknown): StoredComposerPreset | null {
  if (
    !isRecord(value) ||
    typeof value.name !== "string" ||
    !isRecord(value.settings)
  ) {
    return null;
  }

  const name = normalizeName(value.name);
  if (name === null) {
    return null;
  }

  return { name, settings: normalizeSettings(value.settings) };
}

function normalizeSettings(
  value: ChatSettings | Record<string, unknown>,
): ChatSettings {
  const raw = value as Record<string, unknown>;
  const settings: ChatSettings = {};

  if (isNumberWithin(raw.temperature, MIN_TEMPERATURE, MAX_TEMPERATURE)) {
    settings.temperature = raw.temperature;
  }

  if (isNumberWithin(raw.topP, MIN_TOP_P, MAX_TOP_P)) {
    settings.topP = raw.topP;
  }

  if (
    isPositiveInteger(raw.topK) &&
    isNumberWithin(raw.topK, MIN_TOP_K, MAX_TOP_K)
  ) {
    settings.topK = raw.topK;
  }

  if (isNumberWithin(raw.frequencyPenalty, MIN_PENALTY, MAX_PENALTY)) {
    settings.frequencyPenalty = raw.frequencyPenalty;
  }

  if (isNumberWithin(raw.presencePenalty, MIN_PENALTY, MAX_PENALTY)) {
    settings.presencePenalty = raw.presencePenalty;
  }

  if (
    isFiniteNumber(raw.seed) &&
    Number.isSafeInteger(raw.seed) &&
    raw.seed >= 0
  ) {
    settings.seed = raw.seed;
  }

  const stop = normalizeStop(raw.stop);
  if (stop !== undefined) {
    settings.stop = stop;
  }

  if (
    typeof raw.systemPrompt === "string" &&
    raw.systemPrompt.trim() !== "" &&
    !raw.systemPrompt.includes("\u0000") &&
    raw.systemPrompt.length <= MAX_SYSTEM_PROMPT_LENGTH
  ) {
    settings.systemPrompt = raw.systemPrompt;
  }

  if (isPositiveInteger(raw.maxOutputTokens)) {
    settings.maxOutputTokens = raw.maxOutputTokens;
  }

  if (isPositiveInteger(raw.maxHistoryTokens)) {
    settings.maxHistoryTokens = raw.maxHistoryTokens;
  }

  if (
    typeof raw.reasoningEffort === "string" &&
    REASONING_EFFORTS.has(
      raw.reasoningEffort as NonNullable<ChatSettings["reasoningEffort"]>,
    )
  ) {
    settings.reasoningEffort = raw.reasoningEffort as NonNullable<
      ChatSettings["reasoningEffort"]
    >;
  }

  return settings;
}

function normalizeStop(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }

  const stop = value.filter(
    (entry): entry is string =>
      typeof entry === "string" &&
      entry.trim() !== "" &&
      !entry.includes("\u0000") &&
      entry.length <= MAX_STOP_SEQUENCE_LENGTH,
  );

  return stop.length > 0 ? stop.slice(0, MAX_STOP_SEQUENCES) : undefined;
}

function normalizeName(value: string): string | null {
  const name = value.trim();
  if (name === "" || name.length > COMPOSER_PRESET_NAME_MAX_LENGTH) {
    return null;
  }

  return name;
}

function savedPresetID(name: string): string {
  return `${SAVED_COMPOSER_PRESET_ID_PREFIX}${name.toLowerCase()}`;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function isNumberWithin(
  value: unknown,
  minimum: number,
  maximum: number,
): value is number {
  return isFiniteNumber(value) && value >= minimum && value <= maximum;
}

function isPositiveInteger(value: unknown): value is number {
  return isFiniteNumber(value) && Number.isInteger(value) && value > 0;
}
