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

// A spec whose /root names an element that was never defined resolves to no
// tree at all. json-render draws nothing for it, which is indistinguishable
// from "the assistant chose not to draw" — that is how a real model's typo
// ("main" as root, "cardMain" as the element) produced a permanently blank
// block in production, on first paint and on every reload.
describe("Renderer — an unresolvable spec is surfaced, not swallowed", () => {
  const danglingRootSpec = {
    root: "main",
    elements: {
      cardMain: {
        type: "Card",
        props: { id: null, title: "Service Status" },
        children: [],
      },
    },
  } as unknown as SpecSegment["spec"];

  it("shows the invalid-spec notice instead of rendering nothing", () => {
    const { container } = render(Renderer, {
      props: { spec: danglingRootSpec },
    });

    expect(
      container.querySelector(`[data-testid=${TESTID_SPEC_ERROR}]`),
    ).not.toBeNull();
    expect(container.querySelectorAll(`[${DATA_JR_TYPE}]`)).toHaveLength(0);
  });

  it("stays quiet while the block is still streaming", () => {
    // Mid-stream /root legitimately arrives before the element it points at
    // (rule 19 has leaves emitted before their parents), so an incomplete tree
    // must not flash an error at the reader.
    const { container } = render(Renderer, {
      props: { spec: danglingRootSpec, loading: true },
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
