import { afterEach, describe, expect, it } from "vitest";
import {
  COMPOSER_PRESET_NAME_MAX_LENGTH,
  deleteComposerPreset,
  listComposerPresets,
  MAX_SAVED_COMPOSER_PRESETS,
  saveComposerPreset,
} from "./composer-presets";

const STORAGE_KEY = "chatz-composer-presets";
const SAVED_PRESET_NAME = "Long reports";
const REPLACEMENT_PRESET_NAME = "long reports";

describe("composer presets", () => {
  afterEach(() => {
    localStorage.removeItem(STORAGE_KEY);
  });

  it("offers sensible built-in settings", () => {
    expect(listComposerPresets().map((preset) => preset.name)).toEqual([
      "Precise",
      "Balanced",
      "Creative",
    ]);
  });

  it("persists and replaces a saved preset by name", () => {
    saveComposerPreset(SAVED_PRESET_NAME, {
      temperature: 0.4,
      maxOutputTokens: 4_000,
    });
    saveComposerPreset(REPLACEMENT_PRESET_NAME, { temperature: 0.8 });

    expect(listComposerPresets()).toContainEqual({
      id: "saved-long reports",
      name: REPLACEMENT_PRESET_NAME,
      settings: { temperature: 0.8 },
      builtIn: false,
    });
  });

  it("drops malformed persisted settings instead of trusting local storage", () => {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify([
        {
          name: "Review",
          settings: {
            temperature: "hot",
            topP: 4,
            maxHistoryTokens: 9,
            maxOutputTokens: -1,
            reasoningEffort: "invalid",
          },
        },
      ]),
    );

    expect(listComposerPresets()).toContainEqual({
      id: "saved-review",
      name: "Review",
      settings: { maxHistoryTokens: 9 },
      builtIn: false,
    });
  });

  it("deletes only the named saved preset", () => {
    saveComposerPreset("A", { temperature: 0.2 });
    saveComposerPreset("B", { temperature: 0.8 });
    deleteComposerPreset("A");

    expect(listComposerPresets().map((preset) => preset.name)).toEqual([
      "Precise",
      "Balanced",
      "Creative",
      "B",
    ]);
  });

  it("rejects an empty or oversized preset name", () => {
    saveComposerPreset("", { temperature: 0.2 });
    saveComposerPreset("x".repeat(COMPOSER_PRESET_NAME_MAX_LENGTH + 1), {
      temperature: 0.2,
    });

    expect(listComposerPresets()).toHaveLength(3);
  });

  it("keeps the most recent saved presets within the storage bound", () => {
    for (let index = 0; index <= MAX_SAVED_COMPOSER_PRESETS; index += 1) {
      saveComposerPreset(`Preset ${index}`, { temperature: 0.2 });
    }

    const saved = listComposerPresets().filter((preset) => !preset.builtIn);
    expect(saved).toHaveLength(MAX_SAVED_COMPOSER_PRESETS);
    expect(saved.map((preset) => preset.name)).not.toContain("Preset 0");
    expect(saved.map((preset) => preset.name)).toContain(
      `Preset ${MAX_SAVED_COMPOSER_PRESETS}`,
    );
  });
});
