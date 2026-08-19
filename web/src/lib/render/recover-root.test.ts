import { describe, it, expect } from "vitest";
import type { Spec } from "@json-render/svelte";
import { recoverRoot } from "./recover-root";

// Build a flat spec with childless leaf elements plus explicit container links,
// so each case controls exactly which keys are referenced as children.
function spec(root: string, links: Record<string, string[]>): Spec {
  const elements: Record<string, unknown> = {};
  for (const [key, children] of Object.entries(links)) {
    elements[key] = { type: "Card", props: { id: null }, children };
  }

  return { root, elements } as unknown as Spec;
}

describe("recoverRoot", () => {
  it("rewires a dangling root to the sole unreferenced element", () => {
    // The production shape: /root names "main" (undefined); "stackMain" is the
    // real top, no element lists it among its children.
    const out = recoverRoot(
      spec("main", {
        stackMain: ["heading1", "gridStats"],
        heading1: [],
        gridStats: ["statA"],
        statA: [],
      }),
    );

    expect(out.root).toBe("stackMain");
  });

  it("recovers a single-element spec whose root names nothing", () => {
    const out = recoverRoot(spec("main", { cardMain: [] }));

    expect(out.root).toBe("cardMain");
  });

  it("leaves a spec whose root already resolves untouched", () => {
    const input = spec("stackMain", { stackMain: ["leaf"], leaf: [] });
    const out = recoverRoot(input);

    expect(out).toBe(input);
    expect(out.root).toBe("stackMain");
  });

  it("does not guess when two elements are unreferenced (ambiguous)", () => {
    const input = spec("main", { cardA: [], cardB: [] });
    const out = recoverRoot(input);

    expect(out).toBe(input);
    expect(out.root).toBe("main");
  });

  it("does not guess when every element is referenced (a cycle)", () => {
    const input = spec("main", { a: ["b"], b: ["a"] });
    const out = recoverRoot(input);

    expect(out.root).toBe("main");
  });

  it("returns an empty-element spec unchanged", () => {
    const input = spec("main", {});
    const out = recoverRoot(input);

    expect(out).toBe(input);
  });
});
