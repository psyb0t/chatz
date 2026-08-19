import { readFileSync, readdirSync } from "node:fs";
import { describe, it, expect } from "vitest";
import { render } from "@testing-library/svelte";
import { parseMessageSegments, SEGMENT_SPEC, type SpecSegment } from "./fence";
import Renderer from "./Renderer.svelte";
import { DATA_JR_TYPE, GENUI_COMPONENT_NAMES } from "$lib/common/render";
import { TESTID_SPEC_ERROR } from "$lib/common/test-ids";

// The backend ships deterministic showcase responses (fixedresponses/*.txt),
// matched verbatim in showcase mode. Each holds a hand-authored ```spec block.
// This suite renders every one through the SAME fence parser + Renderer the
// browser uses and asserts (a) none surfaces the invalid-spec notice and (b)
// across all of them, every catalog component type actually draws. It is the
// deterministic, model-independent proof that the renderer draws all catalog
// element types, and a regression guard: break a showcase spec, the catalog, or
// the renderer, and this fails.
const SHOWCASE_DIR = `${process.cwd()}/../internal/pkg/core/chats/fixedresponses`;

function showcaseSpecs(): { file: string; spec: SpecSegment["spec"] }[] {
  const out: { file: string; spec: SpecSegment["spec"] }[] = [];
  for (const file of readdirSync(SHOWCASE_DIR)) {
    if (!file.endsWith(".txt")) {
      continue;
    }

    const text = readFileSync(`${SHOWCASE_DIR}/${file}`, "utf8");
    for (const segment of parseMessageSegments(text)) {
      if (segment.kind === SEGMENT_SPEC) {
        out.push({ file, spec: segment.spec });
      }
    }
  }

  return out;
}

// data-jr-type carries the catalog type name on every rendered node, so the
// attribute values across a render are exactly the component types that drew.
function renderedTypes(container: HTMLElement): string[] {
  return [...container.querySelectorAll(`[${DATA_JR_TYPE}]`)].map(
    (el) => el.getAttribute(DATA_JR_TYPE) ?? "",
  );
}

describe("showcase specs render every catalog component", () => {
  const specs = showcaseSpecs();

  it("finds a spec in every showcase file", () => {
    // Guards against a silently-empty sweep (wrong path, renamed fence) that
    // would make the coverage assertion below vacuously pass. There are seven
    // showcase files, each with one dashboard spec.
    expect(specs.length).toBeGreaterThanOrEqual(7);
  });

  it.each(specs)(
    "renders $file without the invalid-spec notice",
    ({ spec }) => {
      const { container } = render(Renderer, { props: { spec } });

      expect(
        container.querySelector(`[data-testid=${TESTID_SPEC_ERROR}]`),
      ).toBeNull();
      expect(renderedTypes(container).length).toBeGreaterThan(0);
    },
  );

  it("draws every catalog component type across the showcases", () => {
    const drawn = new Set<string>();
    for (const { spec } of specs) {
      const { container } = render(Renderer, { props: { spec } });
      for (const type of renderedTypes(container)) {
        drawn.add(type);
      }
    }

    const missing = GENUI_COMPONENT_NAMES.filter((name) => !drawn.has(name));
    expect(missing, `component types that never rendered: ${missing}`).toEqual(
      [],
    );
  });
});
