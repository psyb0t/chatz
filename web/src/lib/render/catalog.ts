// The 100%-custom json-render catalog — the SHARED contract with the backend
// showcase turns and any LLM that emits a ```spec block. We author every
// component ourselves (no default-component dependency) so we control per-element
// attributes; specifically every component's props schema starts with
// `id: z.string().nullable()` and each component spreads props.id onto its root
// (id-on-everything). Props are an OPEN record at render time — unknown props are
// tolerated — so streaming half-built elements never throw.
//
// `.nullable()` (not `.optional()`) is used throughout because the LLM produces
// better output when it emits an explicit null than when it omits a key.
import { schema } from "@json-render/svelte/schema";
import { z } from "zod";
import {
  COMP_AREA_CHART,
  COMP_BADGE,
  COMP_BAR_CHART,
  COMP_BOX_PLOT,
  COMP_CALLOUT,
  COMP_CARD,
  COMP_DONUT_CHART,
  COMP_FUNNEL_CHART,
  COMP_GAUGE,
  COMP_GRID,
  COMP_HEADING,
  COMP_HEATMAP,
  COMP_HISTOGRAM,
  COMP_KEY_VALUE,
  COMP_LOG_VIEWER,
  COMP_NETWORK_GRAPH,
  COMP_PROGRESS,
  COMP_SCATTER_PLOT,
  COMP_SPARKLINE,
  COMP_STACK,
  COMP_STAT,
  COMP_TABLE,
  COMP_TEXT,
  COMP_TIME_SERIES_CHART,
  COMP_TIMELINE,
  COMP_TREEMAP,
} from "$lib/common/render";

const id = z.string().nullable();

export const genUICatalog = schema.createCatalog({
  components: {
    [COMP_TEXT]: {
      props: z.object({ id, content: z.string() }),
      slots: [],
      description: "A paragraph of plain body text.",
      example: { id: null, content: "Occupancy is trending up this week." },
    },

    [COMP_HEADING]: {
      props: z.object({
        id,
        level: z.union([z.literal(1), z.literal(2), z.literal(3)]),
        content: z.string(),
      }),
      slots: [],
      description: "A section heading. Level 1 is largest, 3 smallest.",
      example: { id: null, level: 2, content: "Portfolio Overview" },
    },

    [COMP_CARD]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        description: z.string().nullable(),
      }),
      slots: ["default"],
      description:
        "A bordered container with an optional title + description; holds children.",
      example: { id: null, title: "Listings", description: "Active only" },
    },

    [COMP_STACK]: {
      props: z.object({
        id,
        direction: z.enum(["vertical", "horizontal"]),
        gap: z.enum(["sm", "md", "lg"]),
      }),
      slots: ["default"],
      description:
        "A flex layout container that stacks its children vertically or horizontally.",
      example: { id: null, direction: "vertical", gap: "md" },
    },

    [COMP_GRID]: {
      props: z.object({ id, columns: z.number() }),
      slots: ["default"],
      description: "A responsive grid container with a fixed column count.",
      example: { id: null, columns: 3 },
    },

    [COMP_STAT]: {
      props: z.object({
        id,
        label: z.string(),
        value: z.string(),
        unit: z.string().nullable(),
        delta: z.number().nullable(),
      }),
      slots: [],
      description:
        "A single metric: label, value, optional unit, and an optional signed delta (positive renders ▲ ok, negative ▼ crit).",
      example: {
        id: null,
        label: "Occupancy",
        value: "92",
        unit: "%",
        delta: 4,
      },
    },

    [COMP_BADGE]: {
      props: z.object({
        id,
        label: z.string(),
        variant: z.enum(["ok", "warn", "crit", "info"]),
      }),
      slots: [],
      description: "A small status pill in one of four semantic variants.",
      example: { id: null, label: "LIVE", variant: "ok" },
    },

    [COMP_TABLE]: {
      props: z.object({
        id,
        columns: z.array(z.string()),
        rows: z.array(z.array(z.string())),
      }),
      slots: [],
      description:
        "A data table. `columns` are header labels; each row is an array of cell strings aligned to the columns.",
      example: {
        id: null,
        columns: ["Unit", "Status"],
        rows: [
          ["A-101", "Booked"],
          ["A-102", "Open"],
        ],
      },
    },

    [COMP_KEY_VALUE]: {
      props: z.object({
        id,
        items: z.array(z.object({ label: z.string(), value: z.string() })),
      }),
      slots: [],
      description: "A list of label/value pairs (a definition list).",
      example: {
        id: null,
        items: [
          { label: "Owner", value: "Acme LLC" },
          { label: "Region", value: "EU-West" },
        ],
      },
    },

    [COMP_CALLOUT]: {
      props: z.object({
        id,
        variant: z.enum(["info", "warn", "error"]),
        title: z.string().nullable(),
        text: z.string(),
      }),
      slots: [],
      description:
        "A highlighted callout box for a note, warning, or error, with optional title.",
      example: {
        id: null,
        variant: "warn",
        title: "Heads up",
        text: "Two units expire this month.",
      },
    },

    [COMP_TIMELINE]: {
      props: z.object({
        id,
        items: z.array(
          z.object({
            time: z.string(),
            label: z.string(),
            detail: z.string().nullable(),
          }),
        ),
      }),
      slots: [],
      description: "A vertical timeline of time-stamped events.",
      example: {
        id: null,
        items: [
          { time: "09:00", label: "Check-in", detail: "Unit A-101" },
          { time: "14:30", label: "Inspection", detail: null },
        ],
      },
    },

    [COMP_PROGRESS]: {
      props: z.object({
        id,
        label: z.string(),
        value: z.number(),
        max: z.number(),
        warn: z.number().nullable(),
        crit: z.number().nullable(),
      }),
      slots: [],
      description:
        "A labelled progress bar. The fill colors by threshold: at/above `crit` it is critical, at/above `warn` it is warning, otherwise ok.",
      example: {
        id: null,
        label: "Capacity",
        value: 82,
        max: 100,
        warn: 75,
        crit: 90,
      },
    },

    [COMP_TIME_SERIES_CHART]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        xLabel: z.string().nullable(),
        yLabel: z.string().nullable(),
        height: z.number().nullable(),
        series: z.array(
          z.object({
            name: z.string(),
            points: z.array(z.object({ x: z.string(), y: z.number() })),
          }),
        ),
      }),
      slots: [],
      description:
        "A responsive multi-series line chart for values observed across ordered labels or timestamps.",
      example: {
        id: null,
        title: "Usage trend",
        xLabel: "Period",
        yLabel: "Requests",
        height: 320,
        series: [
          {
            name: "Series A",
            points: [
              { x: "P1", y: 12 },
              { x: "P2", y: 18 },
            ],
          },
        ],
      },
    },

    [COMP_AREA_CHART]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        xLabel: z.string().nullable(),
        yLabel: z.string().nullable(),
        height: z.number().nullable(),
        series: z.array(
          z.object({
            name: z.string(),
            points: z.array(z.object({ x: z.string(), y: z.number() })),
          }),
        ),
      }),
      slots: [],
      description:
        "A filled multi-series trend chart for totals, volume, utilization, or cumulative values.",
      example: {
        id: null,
        title: "Volume over time",
        xLabel: "Period",
        yLabel: "Volume",
        height: 320,
        series: [
          {
            name: "Series A",
            points: [
              { x: "P1", y: 20 },
              { x: "P2", y: 28 },
            ],
          },
        ],
      },
    },

    [COMP_SPARKLINE]: {
      props: z.object({
        id,
        label: z.string(),
        value: z.string(),
        unit: z.string().nullable(),
        values: z.array(z.number()),
        trend: z.enum(["up", "down", "flat"]).nullable(),
      }),
      slots: [],
      description:
        "A compact metric with an inline trend line for dashboards and dense summaries.",
      example: {
        id: null,
        label: "Active records",
        value: "128",
        unit: null,
        values: [92, 104, 101, 128],
        trend: "up",
      },
    },

    [COMP_BAR_CHART]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        categories: z.array(z.string()),
        series: z.array(
          z.object({ name: z.string(), values: z.array(z.number()) }),
        ),
        stacked: z.boolean(),
        orientation: z.enum(["vertical", "horizontal"]),
        height: z.number().nullable(),
      }),
      slots: [],
      description:
        "A grouped or stacked bar chart with vertical or horizontal orientation for categorical comparisons.",
      example: {
        id: null,
        title: "Category comparison",
        categories: ["Group A", "Group B"],
        series: [{ name: "Count", values: [12, 18] }],
        stacked: false,
        orientation: "vertical",
        height: 320,
      },
    },

    [COMP_DONUT_CHART]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        centerLabel: z.string().nullable(),
        slices: z.array(z.object({ label: z.string(), value: z.number() })),
        height: z.number().nullable(),
      }),
      slots: [],
      description:
        "A proportional donut chart for composition or share-of-total data.",
      example: {
        id: null,
        title: "Composition",
        centerLabel: "Total",
        slices: [
          { label: "Group A", value: 62 },
          { label: "Group B", value: 38 },
        ],
        height: 320,
      },
    },

    [COMP_FUNNEL_CHART]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        stages: z.array(z.object({ label: z.string(), value: z.number() })),
        height: z.number().nullable(),
      }),
      slots: [],
      description:
        "A conversion funnel showing sequential stage volumes and retention percentages.",
      example: {
        id: null,
        title: "Adoption funnel",
        stages: [
          { label: "Started", value: 100 },
          { label: "Activated", value: 64 },
          { label: "Retained", value: 41 },
        ],
        height: 320,
      },
    },

    [COMP_GAUGE]: {
      props: z.object({
        id,
        label: z.string(),
        value: z.number(),
        min: z.number(),
        max: z.number(),
        unit: z.string().nullable(),
        warn: z.number().nullable(),
        crit: z.number().nullable(),
      }),
      slots: [],
      description:
        "A compact radial gauge for one bounded metric with optional warning and critical thresholds.",
      example: {
        id: null,
        label: "Utilization",
        value: 68,
        min: 0,
        max: 100,
        unit: "%",
        warn: 75,
        crit: 90,
      },
    },

    [COMP_SCATTER_PLOT]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        xLabel: z.string().nullable(),
        yLabel: z.string().nullable(),
        height: z.number().nullable(),
        series: z.array(
          z.object({
            name: z.string(),
            points: z.array(
              z.object({
                x: z.number(),
                y: z.number(),
                label: z.string().nullable(),
              }),
            ),
          }),
        ),
      }),
      slots: [],
      description:
        "A multi-series scatter plot for correlations, clusters, and outlier inspection.",
      example: {
        id: null,
        title: "Correlation",
        xLabel: "Input",
        yLabel: "Output",
        height: 320,
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
    },

    [COMP_HEATMAP]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        xLabels: z.array(z.string()),
        yLabels: z.array(z.string()),
        values: z.array(z.array(z.number())),
        height: z.number().nullable(),
      }),
      slots: [],
      description:
        "A matrix heatmap for intensity across two categorical dimensions.",
      example: {
        id: null,
        title: "Activity matrix",
        xLabels: ["P1", "P2"],
        yLabels: ["Group A", "Group B"],
        values: [
          [2, 8],
          [5, 3],
        ],
        height: 320,
      },
    },

    [COMP_HISTOGRAM]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        xLabel: z.string().nullable(),
        yLabel: z.string().nullable(),
        bins: z.array(z.object({ label: z.string(), value: z.number() })),
        height: z.number().nullable(),
      }),
      slots: [],
      description: "A histogram for an already-binned numeric distribution.",
      example: {
        id: null,
        title: "Distribution",
        xLabel: "Range",
        yLabel: "Count",
        bins: [
          { label: "0–10", value: 4 },
          { label: "10–20", value: 11 },
        ],
        height: 320,
      },
    },

    [COMP_BOX_PLOT]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        yLabel: z.string().nullable(),
        groups: z.array(
          z.object({
            label: z.string(),
            min: z.number(),
            q1: z.number(),
            median: z.number(),
            q3: z.number(),
            max: z.number(),
          }),
        ),
        height: z.number().nullable(),
      }),
      slots: [],
      description: "A box plot comparing five-number summaries across groups.",
      example: {
        id: null,
        title: "Latency spread",
        yLabel: "Milliseconds",
        groups: [
          { label: "Group A", min: 8, q1: 12, median: 16, q3: 22, max: 31 },
        ],
        height: 320,
      },
    },

    [COMP_TREEMAP]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        items: z.array(
          z.object({
            label: z.string(),
            value: z.number(),
            group: z.string().nullable(),
          }),
        ),
        height: z.number().nullable(),
      }),
      slots: [],
      description:
        "A proportional treemap for hierarchical-looking composition, ownership, storage, or grouped totals.",
      example: {
        id: null,
        title: "Share by category",
        items: [
          { label: "Item A", value: 55, group: "Group 1" },
          { label: "Item B", value: 45, group: "Group 2" },
        ],
        height: 320,
      },
    },

    [COMP_NETWORK_GRAPH]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        nodes: z.array(
          z.object({
            id: z.string(),
            label: z.string(),
            group: z.string().nullable(),
            value: z.number().nullable(),
          }),
        ),
        edges: z.array(
          z.object({
            source: z.string(),
            target: z.string(),
            weight: z.number().nullable(),
          }),
        ),
        height: z.number().nullable(),
      }),
      slots: [],
      description:
        "A deterministic node-link graph for relationships, dependencies, and cluster inspection.",
      example: {
        id: null,
        title: "Relationships",
        nodes: [
          { id: "a", label: "Node A", group: "Group 1", value: 8 },
          { id: "b", label: "Node B", group: "Group 1", value: 5 },
        ],
        edges: [{ source: "a", target: "b", weight: 2 }],
        height: 360,
      },
    },

    [COMP_LOG_VIEWER]: {
      props: z.object({
        id,
        title: z.string().nullable(),
        entries: z.array(
          z.object({
            time: z.string(),
            level: z.enum(["debug", "info", "warn", "error"]),
            source: z.string().nullable(),
            message: z.string(),
          }),
        ),
        wrap: z.boolean(),
        maxHeight: z.number().nullable(),
      }),
      slots: [],
      description:
        "A large scrollable monospace log viewer with time, severity, optional source, and message columns.",
      example: {
        id: null,
        title: "Recent logs",
        entries: [
          {
            time: "T+00:01",
            level: "info",
            source: "worker",
            message: "batch completed",
          },
        ],
        wrap: false,
        maxHeight: 420,
      },
    },
  },

  actions: {},
});
