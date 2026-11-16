import { fireEvent, render, screen } from "@testing-library/svelte";
import { afterEach, beforeEach, vi } from "vitest";
import { describe, expect, it } from "vitest";
import type { ChatSettings as Settings, Model } from "#lib/api/client.js";
import { conversation } from "#lib/stores/conversation.svelte.js";
import ChatSettings from "./ChatSettings.svelte";

const STORAGE_KEY = "chatz-composer-presets";
const SAVE_DEBOUNCE_MS = 500;

function model(controls?: Model["controls"]): Model {
  return {
    id: "some-model",
    upstream: "gateway",
    availability: "available",
    default: false,
    controls,
  };
}

const THINKING_MODEL = model({
  reasoningEfforts: ["none", "low", "medium", "high"],
  supportsSampling: true,
  supportsPenalties: true,
  supportsSeed: true,
  supportsTopK: false,
  supportsStop: true,
});

const PLAIN_MODEL = model({
  reasoningEfforts: [],
  supportsSampling: false,
  supportsPenalties: false,
  supportsSeed: false,
  supportsTopK: false,
  supportsStop: true,
});

let saved: Settings[] = [];

beforeEach(() => {
  saved = [];
  conversation.settings = null;
  vi.spyOn(conversation, "updateSettings").mockImplementation(
    async (next: Settings) => {
      saved.push(next);
      conversation.settings = next;
    },
  );
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
  localStorage.removeItem(STORAGE_KEY);
});

function renderPanel(selected: Model): void {
  render(ChatSettings, {
    props: { model: selected, onClose: () => undefined },
  });
}

describe("ChatSettings", () => {
  it("offers the model's reasoning levels, with Off for thinking off", () => {
    renderPanel(THINKING_MODEL);

    const group = screen.getByRole("radiogroup", { name: "Reasoning effort" });
    const labels = Array.from(group.querySelectorAll("button")).map((button) =>
      button.textContent?.trim(),
    );
    expect(labels).toEqual(["Auto", "Off", "Low", "Medium", "High"]);
  });

  it("explains a model without reasoning control and marks unsupported knobs", () => {
    renderPanel(PLAIN_MODEL);

    expect(
      screen.queryByRole("radiogroup", { name: "Reasoning effort" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("This model has no reasoning controls."),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Not supported by this model").length,
    ).toBeGreaterThan(0);
  });

  it("autosaves only the settings that are switched on", async () => {
    vi.useFakeTimers();
    renderPanel(THINKING_MODEL);

    await fireEvent.click(screen.getByRole("radio", { name: "High" }));
    await fireEvent.click(
      screen.getByRole("switch", { name: "Set Temperature" }),
    );
    await fireEvent.input(
      screen.getByPlaceholderText(
        "Instructions for this chat, added after chatz's own",
      ),
      { target: { value: "Answer in French." } },
    );

    await vi.advanceTimersByTimeAsync(SAVE_DEBOUNCE_MS + 50);

    expect(saved.at(-1)).toEqual({
      reasoningEffort: "high",
      temperature: 0.7,
      systemPrompt: "Answer in French.",
    });
  });

  it("applies a built-in preset to the open settings form", async () => {
    renderPanel(THINKING_MODEL);

    await fireEvent.change(screen.getByLabelText("Preset"), {
      target: { value: "built-in-precise" },
    });

    expect(screen.getByLabelText("Temperature")).toHaveValue(0.2);
  });

  it("saves and removes a browser-local preset", async () => {
    renderPanel(THINKING_MODEL);

    await fireEvent.input(screen.getByPlaceholderText("Preset name"), {
      target: { value: "Reports" },
    });
    await fireEvent.click(screen.getByText("Save preset"));

    expect(screen.getByLabelText("Preset")).toHaveValue("saved-reports");
    expect(screen.getByText("Delete preset")).toBeInTheDocument();

    await fireEvent.click(screen.getByText("Delete preset"));

    expect(screen.getByLabelText("Preset")).toHaveValue("");
    expect(screen.queryByText("Delete preset")).not.toBeInTheDocument();
  });
});
