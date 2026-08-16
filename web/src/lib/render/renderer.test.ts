import { describe, it, expect } from "vitest";
import { render } from "@testing-library/svelte";
import { parseMessageSegments, SEGMENT_SPEC, type SpecSegment } from "./fence";
import Renderer from "./Renderer.svelte";
import { DATA_JR_TYPE } from "$lib/common/render";
import { DEMO_MESSAGE_NO_INTRO_ONE_ROW } from "./__fixtures__/analytics";

// Same canned spec the backend showcase streams (see DEMO_MESSAGE in
// __fixtures__/analytics.ts, minus the intro line and with a single Table
// row — this test only cares about element counts, not row content): a Card
// wrapping a Grid of two Stats + a Badge, plus a Table — 6 catalog elements.
// After the refactor that renders the catalog Badge/Card/Table via the
// shared ui primitives, every catalog root MUST still carry data-jr-type
// (the e2e contract). This mounts the real Renderer and asserts the stamp
// survives the primitive-wrapping.
function demoSpec(): SpecSegment["spec"] {
  const spec = parseMessageSegments(DEMO_MESSAGE_NO_INTRO_ONE_ROW).find(
    (s): s is SpecSegment => s.kind === SEGMENT_SPEC,
  );
  expect(spec).toBeDefined();
  return (spec as SpecSegment).spec;
}

describe("Renderer — data-jr-type contract survives ui-primitive refactor", () => {
  it("stamps data-jr-type on every catalog root (6 elements)", () => {
    const { container } = render(Renderer, { props: { spec: demoSpec() } });

    const stamped = container.querySelectorAll(`[${DATA_JR_TYPE}]`);
    expect(stamped).toHaveLength(6);
  });

  it("carries the element-key id onto each stamped root", () => {
    const { container } = render(Renderer, { props: { spec: demoSpec() } });

    for (const key of ["card", "grid", "s1", "s2", "b1", "tbl"]) {
      const el = container.querySelector(`#${key}`);
      expect(el, `element #${key} rendered`).not.toBeNull();
      expect(el?.getAttribute(DATA_JR_TYPE)).not.toBeNull();
    }
  });
});
