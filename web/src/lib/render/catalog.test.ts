import { describe, it, expect } from "vitest";
import { parseMessageSegments, SEGMENT_SPEC, type SpecSegment } from "./fence";
import { stampSpecIds } from "./stamp";
import {
  COMP_BADGE,
  COMP_BOX_PLOT,
  COMP_CARD,
  COMP_GRID,
  COMP_LOG_VIEWER,
  COMP_STAT,
  COMP_TABLE,
  COMP_TIME_SERIES_CHART,
} from "$lib/common/render";
import { genUICatalog } from "./catalog";
import { ANALYTICAL_COMPONENTS, DEMO_MESSAGE } from "./__fixtures__/analytics";

function onlySpec(message: string): SpecSegment {
  const specs = parseMessageSegments(message).filter(
    (s): s is SpecSegment => s.kind === SEGMENT_SPEC,
  );
  expect(specs).toHaveLength(1);
  return specs[0];
}

describe("catalog demo spec — parse + assemble", () => {
  it("assembles the full Card/Grid/Stat/Badge/Table tree", () => {
    const seg = onlySpec(DEMO_MESSAGE);
    expect(seg.closed).toBe(true);
    expect(seg.spec.root).toBe("card");

    const { elements } = seg.spec;
    expect(Object.keys(elements).sort()).toEqual([
      "b1",
      "card",
      "grid",
      "s1",
      "s2",
      "tbl",
    ]);

    expect(elements.card.type).toBe(COMP_CARD);
    expect(elements.card.children).toEqual(["grid", "tbl"]);
    expect(elements.grid.type).toBe(COMP_GRID);
    expect(elements.grid.children).toEqual(["s1", "s2", "b1"]);
    expect(elements.s1.type).toBe(COMP_STAT);
    expect(elements.s2.type).toBe(COMP_STAT);
    expect(elements.b1.type).toBe(COMP_BADGE);
    expect(elements.tbl.type).toBe(COMP_TABLE);
  });

  it("carries typed props through assembly", () => {
    const { elements } = onlySpec(DEMO_MESSAGE).spec;
    expect(elements.s1.props).toMatchObject({
      label: "Occupancy",
      value: "92",
      unit: "%",
      delta: 4,
    });
    expect(elements.tbl.props).toMatchObject({
      columns: ["Unit", "Status"],
      rows: [
        ["A-101", "Booked"],
        ["A-102", "Open"],
      ],
    });
  });

  it("assembles the same tree from leaf-first and parent-first streams", () => {
    const patches = {
      root: '{"op":"add","path":"/root","value":"card"}',
      parent:
        '{"op":"add","path":"/elements/card","value":{"type":"Card","props":{"id":null,"title":"Ordered","description":null},"children":["child"]}}',
      child:
        '{"op":"add","path":"/elements/child","value":{"type":"Text","props":{"id":null,"content":"Ready"},"children":[]}}',
    };
    const message = (orderedPatches: string[]) =>
      ["```spec", ...orderedPatches, "```"].join("\n");
    const leafFirst = onlySpec(
      message([patches.child, patches.parent, patches.root]),
    ).spec;
    const parentFirst = onlySpec(
      message([patches.root, patches.parent, patches.child]),
    ).spec;

    expect(leafFirst).toEqual(parentFirst);
    expect(leafFirst.elements.card.children).toEqual(["child"]);
    expect(leafFirst.elements.child.props.content).toBe("Ready");
  });
});

describe("stampSpecIds", () => {
  it("stamps every null props.id with the element's map key", () => {
    const stamped = stampSpecIds(onlySpec(DEMO_MESSAGE).spec);
    for (const [key, el] of Object.entries(stamped.elements)) {
      expect(el.props.id).toBe(key);
    }
  });

  it("leaves a non-null id untouched and fills an absent one", () => {
    const spec = {
      root: "a",
      elements: {
        a: { type: COMP_CARD, props: { id: "custom-id" }, children: [] },
        b: { type: COMP_BADGE, props: {}, children: [] },
      },
    };

    const stamped = stampSpecIds(spec);
    expect(stamped.elements.a.props.id).toBe("custom-id");
    expect(stamped.elements.b.props.id).toBe("b");
  });

  it("does not mutate the input spec", () => {
    const spec = onlySpec(DEMO_MESSAGE).spec;
    stampSpecIds(spec);
    expect(spec.elements.card.props.id).toBeNull();
  });
});

describe("GenUI catalog contract", () => {
  it("registers every analytical component in the 26-component catalog", () => {
    expect(genUICatalog.componentNames).toHaveLength(26);
    expect(genUICatalog.componentNames).toEqual(
      expect.arrayContaining([...ANALYTICAL_COMPONENTS]),
    );
  });

  it("keeps every catalog example valid against its declared props schema", () => {
    for (const [name, definition] of Object.entries(
      genUICatalog.data.components,
    )) {
      expect(
        definition.props.safeParse(definition.example).success,
        `${name} example`,
      ).toBe(true);
    }
  });

  it("rejects malformed analytical props at the catalog boundary", () => {
    const components = genUICatalog.data.components;

    expect(
      components[COMP_TIME_SERIES_CHART].props.safeParse({
        id: null,
        title: null,
        xLabel: null,
        yLabel: null,
        height: null,
        series: [{ name: "bad", points: [{ x: "P1", y: "12" }] }],
      }).success,
    ).toBe(false);
    expect(
      components[COMP_BOX_PLOT].props.safeParse({
        id: null,
        title: null,
        yLabel: null,
        height: null,
        groups: [{ label: "bad", min: 1, q1: 2, median: 3, q3: 4 }],
      }).success,
    ).toBe(false);
    expect(
      components[COMP_LOG_VIEWER].props.safeParse({
        id: null,
        title: null,
        entries: [
          { time: "T+1", level: "fatal", source: null, message: "bad" },
        ],
        wrap: false,
        maxHeight: null,
      }).success,
    ).toBe(false);
  });
});
