import { describe, it, expect } from "vitest";
import { render } from "@testing-library/svelte";
import { parseMessageSegments, SEGMENT_SPEC, type SpecSegment } from "./fence";
import Renderer from "./Renderer.svelte";
import { DATA_JR_TYPE } from "$lib/common/render";
import { TESTID_SPEC_ERROR } from "$lib/common/test-ids";
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

// The production failure: the model set /root to the prompt-example literal
// "main" while keying its actual top-level container "stackMain", so /root
// dangled and json-render drew nothing — a permanently blank block, on first
// paint and every reload. The real root is recoverable (the one element no
// other element lists among its children), so the Renderer rewires it and the
// tree renders instead of vanishing.
describe("Renderer — a dangling root is recovered when unambiguous", () => {
  // Structurally the production dashboard: /root names "main" (undefined), but
  // "stackMain" is the true top element — nothing lists it as a child.
  const recoverableSpec = {
    root: "main",
    elements: {
      stackMain: {
        type: "Stack",
        props: { id: null, direction: "vertical", gap: "md" },
        children: ["heading1", "gridStats"],
      },
      heading1: {
        type: "Heading",
        props: { id: null, level: 1, content: "System Status" },
        children: [],
      },
      gridStats: {
        type: "Grid",
        props: { id: null, columns: 2 },
        children: ["statA", "statB"],
      },
      statA: {
        type: "Stat",
        props: { id: null, label: "Pending", value: "0" },
        children: [],
      },
      statB: {
        type: "Stat",
        props: { id: null, label: "Processed", value: "1735" },
        children: [],
      },
    },
  } as unknown as SpecSegment["spec"];

  it("renders the tree instead of the invalid-spec notice", () => {
    const { container } = render(Renderer, {
      props: { spec: recoverableSpec },
    });

    expect(
      container.querySelector(`[data-testid=${TESTID_SPEC_ERROR}]`),
    ).toBeNull();
    expect(container.querySelectorAll(`[${DATA_JR_TYPE}]`)).toHaveLength(5);
  });
});

// Recovery only fires when the real root is unambiguous. A spec with two
// unreferenced elements (disconnected trees) and a dangling /root cannot be
// resolved without guessing, so the Renderer leaves it to validateSpec and
// surfaces the notice rather than rendering an arbitrary half of the UI.
describe("Renderer — a genuinely unresolvable spec is surfaced, not swallowed", () => {
  const ambiguousSpec = {
    root: "main",
    elements: {
      cardA: {
        type: "Card",
        props: { id: null, title: "One" },
        children: [],
      },
      cardB: {
        type: "Card",
        props: { id: null, title: "Two" },
        children: [],
      },
    },
  } as unknown as SpecSegment["spec"];

  it("shows the invalid-spec notice instead of rendering nothing", () => {
    const { container } = render(Renderer, { props: { spec: ambiguousSpec } });

    expect(
      container.querySelector(`[data-testid=${TESTID_SPEC_ERROR}]`),
    ).not.toBeNull();
    expect(container.querySelectorAll(`[${DATA_JR_TYPE}]`)).toHaveLength(0);
  });

  it("stays quiet while the block is still streaming", () => {
    // Mid-stream /root legitimately arrives before the element it points at
    // (rule 19 has leaves emitted before their parents), so an incomplete tree
    // must neither flash an error nor be recovered against a transient shape.
    const { container } = render(Renderer, {
      props: { spec: ambiguousSpec, loading: true },
    });

    expect(
      container.querySelector(`[data-testid=${TESTID_SPEC_ERROR}]`),
    ).toBeNull();
  });

  it("does not fire on a well-formed spec", () => {
    const { container } = render(Renderer, { props: { spec: demoSpec() } });

    expect(
      container.querySelector(`[data-testid=${TESTID_SPEC_ERROR}]`),
    ).toBeNull();
  });
});
