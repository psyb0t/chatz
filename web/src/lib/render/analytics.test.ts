import { expect, describe, it } from "vitest";
import { render } from "@testing-library/svelte";
import type { Spec } from "@json-render/svelte";
import {
  COMP_AREA_CHART,
  COMP_BAR_CHART,
  COMP_BOX_PLOT,
  COMP_DONUT_CHART,
  COMP_FUNNEL_CHART,
  COMP_GAUGE,
  COMP_HEATMAP,
  COMP_HISTOGRAM,
  COMP_LOG_VIEWER,
  COMP_NETWORK_GRAPH,
  COMP_SCATTER_PLOT,
  COMP_SPARKLINE,
  COMP_STACK,
  COMP_TIME_SERIES_CHART,
  COMP_TREEMAP,
  DATA_JR_TYPE,
} from "$lib/common/render";
import Renderer from "./Renderer.svelte";
import { ANALYTICAL_COMPONENTS } from "./__fixtures__/analytics";

// Every analytical component renders exactly one <svg> root except
// LogViewer, which renders a plain list — hence -1. Tied to
// ANALYTICAL_COMPONENTS.length so this can't silently drift if a component
// is added/removed from the fixture.
const SVG_COMPONENT_COUNT = ANALYTICAL_COMPONENTS.length - 1;

const GEOMETRY_BY_ID = {
  "item-0": "path",
  "item-1": "path",
  "item-2": "path",
  "item-3": "rect",
  "item-4": "circle",
  "item-5": "polygon",
  "item-6": "path",
  "item-7": "circle",
  "item-8": "rect",
  "item-9": "rect",
  "item-10": "rect",
  "item-11": "rect",
  "item-12": "circle",
} as const;

function analyticalSpec(): Spec {
  return {
    root: "root",
    elements: {
      root: {
        type: COMP_STACK,
        props: { id: null, direction: "vertical", gap: "md" },
        children: ANALYTICAL_COMPONENTS.map((_, index) => `item-${index}`),
      },
      "item-0": {
        type: COMP_TIME_SERIES_CHART,
        props: {
          id: null,
          title: "Request trend",
          xLabel: "Period",
          yLabel: "Count",
          height: 280,
          series: [
            {
              name: "Processed",
              points: [
                { x: "P1", y: 12 },
                { x: "P2", y: 18 },
              ],
            },
          ],
        },
        children: [],
      },
      "item-1": {
        type: COMP_AREA_CHART,
        props: {
          id: null,
          title: "Volume",
          xLabel: "Period",
          yLabel: "Items",
          height: 280,
          series: [
            {
              name: "Queued",
              points: [
                { x: "P1", y: 8 },
                { x: "P2", y: 15 },
              ],
            },
          ],
        },
        children: [],
      },
      "item-2": {
        type: COMP_SPARKLINE,
        props: {
          id: null,
          label: "Active records",
          value: "18",
          unit: null,
          values: [8, 12, 11, 18],
          trend: "up",
        },
        children: [],
      },
      "item-3": {
        type: COMP_BAR_CHART,
        props: {
          id: null,
          title: "Category comparison",
          categories: ["Group A", "Group B"],
          series: [{ name: "Count", values: [12, 18] }],
          stacked: false,
          orientation: "vertical",
          height: 280,
        },
        children: [],
      },
      "item-4": {
        type: COMP_DONUT_CHART,
        props: {
          id: null,
          title: "Composition",
          centerLabel: "Total",
          slices: [
            { label: "Group A", value: 62 },
            { label: "Group B", value: 38 },
          ],
          height: 280,
        },
        children: [],
      },
      "item-5": {
        type: COMP_FUNNEL_CHART,
        props: {
          id: null,
          title: "Adoption funnel",
          stages: [
            { label: "Started", value: 100 },
            { label: "Activated", value: 64 },
            { label: "Retained", value: 41 },
          ],
          height: 280,
        },
        children: [],
      },
      "item-6": {
        type: COMP_GAUGE,
        props: {
          id: null,
          label: "Utilization",
          value: 68,
          min: 0,
          max: 100,
          unit: "%",
          warn: 75,
          crit: 90,
        },
        children: [],
      },
      "item-7": {
        type: COMP_SCATTER_PLOT,
        props: {
          id: null,
          title: "Correlation",
          xLabel: "Input",
          yLabel: "Output",
          height: 280,
          series: [
            {
              name: "Cluster A",
              points: [
                { x: 2, y: 4, label: "Point 1" },
                { x: 3, y: 7, label: "Point 2" },
              ],
            },
          ],
        },
        children: [],
      },
      "item-8": {
        type: COMP_HEATMAP,
        props: {
          id: null,
          title: "Activity matrix",
          xLabels: ["P1", "P2"],
          yLabels: ["Group A", "Group B"],
          values: [
            [2, 8],
            [5, 3],
          ],
          height: 280,
        },
        children: [],
      },
      "item-9": {
        type: COMP_HISTOGRAM,
        props: {
          id: null,
          title: "Distribution",
          xLabel: "Range",
          yLabel: "Count",
          bins: [
            { label: "0–10", value: 4 },
            { label: "10–20", value: 11 },
          ],
          height: 280,
        },
        children: [],
      },
      "item-10": {
        type: COMP_BOX_PLOT,
        props: {
          id: null,
          title: "Latency spread",
          yLabel: "Milliseconds",
          groups: [
            { label: "Group A", min: 8, q1: 12, median: 16, q3: 22, max: 31 },
          ],
          height: 280,
        },
        children: [],
      },
      "item-11": {
        type: COMP_TREEMAP,
        props: {
          id: null,
          title: "Share by category",
          items: [
            { label: "Item A", value: 55, group: "Group 1" },
            { label: "Item B", value: 45, group: "Group 2" },
          ],
          height: 280,
        },
        children: [],
      },
      "item-12": {
        type: COMP_NETWORK_GRAPH,
        props: {
          id: null,
          title: "Relationships",
          nodes: [
            { id: "a", label: "Node A", group: "Group 1", value: 8 },
            { id: "b", label: "Node B", group: "Group 1", value: 5 },
          ],
          edges: [{ source: "a", target: "b", weight: 2 }],
          height: 300,
        },
        children: [],
      },
      "item-13": {
        type: COMP_LOG_VIEWER,
        props: {
          id: null,
          title: "Recent logs",
          entries: [
            {
              time: "T+00:01",
              level: "info",
              source: "worker",
              message: '<img src=x onerror="globalThis.pwned=true">',
            },
          ],
          wrap: false,
          maxHeight: 320,
        },
        children: [],
      },
    },
  };
}

function partialSpec(): Spec {
  const elements: Spec["elements"] = {
    root: {
      type: COMP_STACK,
      props: { id: null, direction: "vertical", gap: "sm" },
      children: ANALYTICAL_COMPONENTS.map((_, index) => `partial-${index}`),
    },
  };

  ANALYTICAL_COMPONENTS.forEach((type, index) => {
    elements[`partial-${index}`] = { type, props: {}, children: [] };
  });

  return { root: "root", elements };
}

describe("analytical GenUI components", () => {
  it("mounts every analytical component through the real registry", () => {
    const { container } = render(Renderer, {
      props: { spec: analyticalSpec() },
    });

    for (const type of ANALYTICAL_COMPONENTS) {
      expect(
        container.querySelectorAll(`[${DATA_JR_TYPE}="${type}"]`),
        `${type} root`,
      ).toHaveLength(1);
    }

    expect(container.querySelectorAll("svg")).toHaveLength(SVG_COMPONENT_COUNT);
    expect(
      container.querySelectorAll("svg title").length,
    ).toBeGreaterThanOrEqual(SVG_COMPONENT_COUNT);
    for (const [id, selector] of Object.entries(GEOMETRY_BY_ID)) {
      expect(
        container.querySelector(`#${id} ${selector}`),
        `${id} geometry`,
      ).not.toBeNull();
    }
  });

  it("provides dataset-level accessible summaries for every analytical view", () => {
    const { container } = render(Renderer, {
      props: { spec: analyticalSpec() },
    });

    expect(
      container.querySelector("#item-0 .jr-chart__sr")?.textContent,
    ).toContain("Processed");
    expect(
      container.querySelector("#item-1 .jr-area__sr")?.textContent,
    ).toContain("Queued");
    expect(
      container.querySelector("#item-2 .jr-sparkline__sr")?.textContent,
    ).toContain("Active records");
    expect(
      container.querySelector("#item-3 table.sr-only")?.textContent,
    ).toContain("Group A");
    for (const id of ["item-4", "item-5", "item-6"]) {
      expect(
        container.querySelector(`#${id} .sr-only`)?.textContent?.trim(),
      ).toBeTruthy();
    }
    for (const id of [
      "item-7",
      "item-8",
      "item-9",
      "item-10",
      "item-11",
      "item-12",
    ]) {
      expect(
        container.querySelector(`#${id} svg desc`)?.textContent?.trim(),
      ).toBeTruthy();
    }
    expect(
      container.querySelector("#item-13 .jr-logs__sr")?.textContent,
    ).toContain("1 log entry");
  });

  it("renders hostile log text as text rather than markup", () => {
    const { container } = render(Renderer, {
      props: { spec: analyticalSpec() },
    });

    expect(container.querySelector("img")).toBeNull();
    expect(container.textContent).toContain(
      '<img src=x onerror="globalThis.pwned=true">',
    );
  });

  it("survives partial streamed props for every analytical component", () => {
    const { container } = render(Renderer, { props: { spec: partialSpec() } });

    for (const type of ANALYTICAL_COMPONENTS) {
      expect(
        container.querySelector(`[${DATA_JR_TYPE}="${type}"]`),
      ).not.toBeNull();
    }
  });

  it("bounds an oversized time-series to at most 500 rendered points and ignores non-finite values", () => {
    const spec = analyticalSpec();
    spec.elements["item-0"].props.series = [
      {
        name: "Oversized",
        points: Array.from({ length: 2_500 }, (_, index) => ({
          x: `P${index}`,
          y: index === 12 ? Number.POSITIVE_INFINITY : index,
        })),
      },
    ];

    const { container } = render(Renderer, { props: { spec } });

    expect(
      container.querySelectorAll("#item-0 circle").length,
    ).toBeLessThanOrEqual(500);
  });

  it("bounds an oversized log stream to 2,000 rendered entries", () => {
    const spec = analyticalSpec();
    spec.elements["item-13"].props.entries = Array.from(
      { length: 10_000 },
      (_, index) => ({
        time: `T+${index}`,
        level: "debug",
        source: null,
        message: `entry ${index}`,
      }),
    );

    const { container } = render(Renderer, { props: { spec } });

    expect(container.querySelectorAll("#item-13 li")).toHaveLength(2_000);
  }, 20_000);

  it("fails safely for malformed analytical domains", () => {
    const spec = analyticalSpec();
    spec.elements["item-4"].props.slices = [
      { label: "zero", value: 0 },
      { label: "negative", value: -2 },
    ];
    spec.elements["item-6"].props.value = Number.POSITIVE_INFINITY;
    spec.elements["item-10"].props.groups = [
      { label: "bad", min: 5, q1: 4, median: 3, q3: 2, max: 1 },
    ];
    spec.elements["item-12"].props.nodes = [
      { id: "duplicate", label: "First", group: null, value: null },
      { id: "duplicate", label: "Second", group: null, value: null },
    ];
    spec.elements["item-12"].props.edges = [
      { source: "duplicate", target: "missing", weight: null },
    ];

    const { container } = render(Renderer, { props: { spec } });

    expect(container.querySelector("#item-4")?.textContent).toContain(
      "No positive slice data",
    );
    expect(container.querySelector("#item-6")?.textContent).toContain(
      "No finite gauge value",
    );
    expect(container.querySelector("#item-10")?.textContent).toContain(
      "No valid box plot data",
    );
    expect(container.querySelectorAll("#item-12 circle")).toHaveLength(1);
    expect(container.querySelectorAll("#item-12 line")).toHaveLength(0);
  });

  it.each([
    [false, "vertical"],
    [false, "horizontal"],
    [true, "vertical"],
    [true, "horizontal"],
  ] as const)(
    "renders bar mode stacked=%s orientation=%s without invalid geometry",
    (stacked, orientation) => {
      const spec = analyticalSpec();
      spec.elements["item-3"].props = {
        ...spec.elements["item-3"].props,
        categories: ["负值", "Constant", "Positive"],
        series: [
          { name: "Series A", values: [-10, 4, 12] },
          { name: "Series B", values: [-3, 4, 7] },
        ],
        stacked,
        orientation,
      };

      const { container } = render(Renderer, { props: { spec } });
      const chart = container.querySelector("#item-3");

      expect(chart?.querySelectorAll("rect").length).toBeGreaterThan(0);
      expect(chart?.innerHTML).not.toMatch(/NaN|Infinity/);
      expect(chart?.textContent).toContain("负值");
    },
  );

  // Shared across the dense-label subtests below: 80 long, non-ASCII labels
  // fed to each component's respective label-bearing prop, so every
  // component independently proves it samples its label ticks/text down to
  // a readable count while keeping one geometry element per data point.
  function denseLabels(): string[] {
    return Array.from(
      { length: 80 },
      (_, index) => `Long category ${index} 分析`,
    );
  }

  it("samples dense bar-chart labels while retaining one rect per category", () => {
    const spec = analyticalSpec();
    const labels = denseLabels();
    spec.elements["item-3"].props.categories = labels;
    spec.elements["item-3"].props.series = [
      { name: "Dense", values: labels.map((_, index) => index - 40) },
    ];

    const { container } = render(Renderer, { props: { spec } });

    expect(container.querySelectorAll("#item-3 rect")).toHaveLength(80);
    expect(container.querySelectorAll("#item-3 svg text").length).toBeLessThan(
      30,
    );
    expect(
      container.querySelector("#item-3 table.sr-only")?.textContent,
    ).toContain("Long category 79 分析");
  });

  // 80x80 = 6,400 rects is the heaviest single render in this file; give it
  // headroom over vitest's 5000ms default so full-suite CPU contention can't
  // intermittently time it out (see project history for the flake this fixes).
  it("samples dense heatmap labels while retaining one rect per cell", () => {
    const spec = analyticalSpec();
    const labels = denseLabels();
    spec.elements["item-8"].props.xLabels = labels;
    spec.elements["item-8"].props.yLabels = labels;
    spec.elements["item-8"].props.values = labels.map((_, row) =>
      labels.map((__, column) => row + column),
    );

    const { container } = render(Renderer, { props: { spec } });

    expect(container.querySelectorAll("#item-8 rect")).toHaveLength(6_400);
    expect(container.querySelectorAll("#item-8 svg text").length).toBeLessThan(
      35,
    );
  }, 20_000);

  it("samples dense box-plot labels while retaining one rect per group", () => {
    const spec = analyticalSpec();
    const labels = denseLabels();
    spec.elements["item-10"].props.groups = labels.map((label, index) => ({
      label,
      min: index,
      q1: index + 1,
      median: index + 2,
      q3: index + 3,
      max: index + 4,
    }));

    const { container } = render(Renderer, { props: { spec } });

    expect(container.querySelectorAll("#item-10 rect")).toHaveLength(80);
    expect(container.querySelectorAll("#item-10 svg text").length).toBeLessThan(
      25,
    );
  });

  it("samples dense network-graph labels while retaining one circle per node", () => {
    const spec = analyticalSpec();
    const labels = denseLabels();
    spec.elements["item-12"].props.nodes = labels.map((label, index) => ({
      id: `node-${index}`,
      label,
      group: `group-${index % 8}`,
      value: index,
    }));
    spec.elements["item-12"].props.edges = [];

    const { container } = render(Renderer, { props: { spec } });

    expect(container.querySelectorAll("#item-12 circle")).toHaveLength(80);
    expect(container.querySelectorAll("#item-12 svg text").length).toBeLessThan(
      30,
    );
  });

  it("renders single-point and constant domains without invalid coordinates", () => {
    const spec = analyticalSpec();
    spec.elements["item-0"].props.series = [
      { name: "Constant", points: [{ x: "Only", y: 7 }] },
    ];
    spec.elements["item-7"].props.series = [
      { name: "Single", points: [{ x: -3, y: -3, label: "唯一" }] },
    ];
    spec.elements["item-9"].props.bins = [{ label: "Constant", value: 7 }];

    const { container } = render(Renderer, { props: { spec } });

    for (const id of ["item-0", "item-7", "item-9"]) {
      expect(container.querySelector(`#${id}`)?.innerHTML).not.toMatch(
        /NaN|Infinity/,
      );
    }
    expect(container.querySelector("#item-0 circle")).not.toBeNull();
    expect(container.querySelector("#item-7 circle")).not.toBeNull();
    expect(container.querySelector("#item-9 rect")).not.toBeNull();
    expect(container.querySelector("#item-7")?.textContent).toContain("唯一");
  });

  // Shared across the six-color-palette-repeat subtests below: 7 series/
  // slices/stages — one more than the 6-color palette — so every consuming
  // component independently proves it falls back to a non-color encoding
  // (dasharray or opacity) once the palette wraps around.
  function sevenSeries(): Array<{
    name: string;
    points: Array<{ x: string; y: number }>;
  }> {
    return Array.from({ length: 7 }, (_, index) => ({
      name: `Series ${index + 1}`,
      points: [
        { x: "P1", y: index + 1 },
        { x: "P2", y: index + 2 },
      ],
    }));
  }

  it("adds a dasharray encoding to the time-series chart when the palette repeats", () => {
    const spec = analyticalSpec();
    spec.elements["item-0"].props.series = sevenSeries();

    const { container } = render(Renderer, { props: { spec } });

    expect(
      container.querySelector("#item-0 path[stroke-dasharray]"),
    ).not.toBeNull();
  });

  it("adds a dasharray encoding to the area chart when the palette repeats", () => {
    const spec = analyticalSpec();
    spec.elements["item-1"].props.series = sevenSeries();

    const { container } = render(Renderer, { props: { spec } });

    expect(
      container.querySelector("#item-1 path[stroke-dasharray]"),
    ).not.toBeNull();
  });

  it("adds a fill-opacity encoding to the bar chart when the palette repeats", () => {
    const spec = analyticalSpec();
    const series = sevenSeries();
    spec.elements["item-3"].props.categories = ["Only"];
    spec.elements["item-3"].props.series = series.map((entry, index) => ({
      name: entry.name,
      values: [index + 1],
    }));

    const { container } = render(Renderer, { props: { spec } });
    const barOpacities = new Set(
      [...container.querySelectorAll("#item-3 rect")].map((element) =>
        element.getAttribute("fill-opacity"),
      ),
    );

    expect(barOpacities.size).toBeGreaterThan(1);
  });

  it("adds a stroke-opacity encoding to the donut chart when the palette repeats", () => {
    const spec = analyticalSpec();
    const series = sevenSeries();
    spec.elements["item-4"].props.slices = series.map((entry, index) => ({
      label: entry.name,
      value: index + 1,
    }));

    const { container } = render(Renderer, { props: { spec } });
    const donutOpacities = new Set(
      [...container.querySelectorAll("#item-4 circle")].map((element) =>
        element.getAttribute("stroke-opacity"),
      ),
    );

    expect(donutOpacities.size).toBeGreaterThan(1);
  });

  it("adds a fill-opacity encoding to the funnel chart when the palette repeats", () => {
    const spec = analyticalSpec();
    const series = sevenSeries();
    spec.elements["item-5"].props.stages = series.map((entry, index) => ({
      label: entry.name,
      value: 7 - index,
    }));

    const { container } = render(Renderer, { props: { spec } });
    const funnelOpacities = new Set(
      [...container.querySelectorAll("#item-5 polygon")].map((element) =>
        element.getAttribute("fill-opacity"),
      ),
    );

    expect(funnelOpacities.size).toBeGreaterThan(1);
  });
});
