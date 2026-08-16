import { fireEvent, render, screen } from "@testing-library/svelte";
import { afterEach, beforeEach } from "vitest";
import { describe, expect, it } from "vitest";
import { conversation } from "$lib/stores/conversation.svelte";
import ChatSettings from "./ChatSettings.svelte";

const STORAGE_KEY = "chatz-composer-presets";

beforeEach(() => {
  conversation.settings = null;
});

afterEach(() => {
  localStorage.removeItem(STORAGE_KEY);
});

describe("ChatSettings", () => {
  it("explains why reasoning effort is unavailable for a model without support", () => {
    render(ChatSettings, {
      props: {
        model: {
          id: "general-model",
          upstream: "gateway",
          availability: "available",
          default: false,
          supportsReasoning: false,
        },
        onClose: () => undefined,
      },
    });

    expect(screen.getByLabelText("Reasoning effort")).toBeDisabled();
    expect(
      screen.getByText("This model does not advertise reasoning controls."),
    ).toBeInTheDocument();
  });

  it("keeps reasoning effort available when the selected model advertises it", () => {
    render(ChatSettings, {
      props: {
        model: {
          id: "reasoning-model",
          upstream: "gateway",
          availability: "available",
          default: false,
          supportsReasoning: true,
        },
        onClose: () => undefined,
      },
    });

    expect(screen.getByLabelText("Reasoning effort")).not.toBeDisabled();
    expect(
      screen.queryByText("This model does not advertise reasoning controls."),
    ).not.toBeInTheDocument();
  });

  it("applies a built-in preset to the open settings form", async () => {
    render(ChatSettings, {
      props: {
        model: {
          id: "general-model",
          upstream: "gateway",
          availability: "available",
          default: false,
          supportsReasoning: true,
        },
        onClose: () => undefined,
      },
    });

    await fireEvent.change(screen.getByLabelText("Preset"), {
      target: { value: "built-in-precise" },
    });

    expect(screen.getByLabelText("Temperature")).toHaveValue(0.2);
  });

  it("saves and removes a browser-local preset", async () => {
    render(ChatSettings, {
      props: {
        model: {
          id: "general-model",
          upstream: "gateway",
          availability: "available",
          default: false,
          supportsReasoning: true,
        },
        onClose: () => undefined,
      },
    });

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
