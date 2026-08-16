import { fireEvent, render, screen } from "@testing-library/svelte";
import { describe, expect, it } from "vitest";
import ModelPicker from "./ModelPicker.svelte";

describe("ModelPicker", () => {
  it("uses aliases for display while retaining model ids and the default marker", async () => {
    render(ModelPicker, {
      props: {
        models: [
          { id: "fast-model", alias: "Fast", default: true },
          { id: "deep-model", alias: "Deep analysis", default: false },
        ],
        value: "fast-model",
      },
    });

    const trigger = screen.getByRole("button", { name: "Model" });
    expect(trigger).toHaveTextContent("Fast");

    await fireEvent.click(trigger);

    expect(screen.getByText("fast-model")).toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();
    expect(screen.getByText("Deep analysis")).toBeInTheDocument();
  });

  it("falls back to the executable id when no alias is configured", () => {
    render(ModelPicker, {
      props: {
        models: [{ id: "base-model", default: false }],
        value: "base-model",
      },
    });

    expect(screen.getByRole("button", { name: "Model" })).toHaveTextContent(
      "base-model",
    );
  });
});
