// Shared render-layer constants — the CONTRACT with the backend showcase turns
// and any LLM that emits a ```spec block. The component `type` strings
// here MUST match the catalog keys in src/lib/render/catalog.ts exactly and the
// keys the backend emits in its JSONL patches. Kept out of the catalog/registry
// modules so tests and the fence parser can reference the same strings without
// pulling in zod / Svelte components.

// Component type names (the `type` field of every UIElement in a spec).
export const COMP_TEXT = "Text";
export const COMP_HEADING = "Heading";
export const COMP_CARD = "Card";
export const COMP_STACK = "Stack";
export const COMP_GRID = "Grid";
export const COMP_STAT = "Stat";
export const COMP_BADGE = "Badge";
export const COMP_TABLE = "Table";
export const COMP_KEY_VALUE = "KeyValue";
export const COMP_CALLOUT = "Callout";
export const COMP_TIMELINE = "Timeline";
export const COMP_PROGRESS = "Progress";
export const COMP_TIME_SERIES_CHART = "TimeSeriesChart";
export const COMP_AREA_CHART = "AreaChart";
export const COMP_SPARKLINE = "Sparkline";
export const COMP_BAR_CHART = "BarChart";
export const COMP_DONUT_CHART = "DonutChart";
export const COMP_FUNNEL_CHART = "FunnelChart";
export const COMP_GAUGE = "Gauge";
export const COMP_SCATTER_PLOT = "ScatterPlot";
export const COMP_HEATMAP = "Heatmap";
export const COMP_HISTOGRAM = "Histogram";
export const COMP_BOX_PLOT = "BoxPlot";
export const COMP_TREEMAP = "Treemap";
export const COMP_NETWORK_GRAPH = "NetworkGraph";
export const COMP_LOG_VIEWER = "LogViewer";

export const GENUI_COMPONENT_NAMES = [
  COMP_TEXT,
  COMP_HEADING,
  COMP_CARD,
  COMP_STACK,
  COMP_GRID,
  COMP_STAT,
  COMP_BADGE,
  COMP_TABLE,
  COMP_KEY_VALUE,
  COMP_CALLOUT,
  COMP_TIMELINE,
  COMP_PROGRESS,
  COMP_TIME_SERIES_CHART,
  COMP_AREA_CHART,
  COMP_SPARKLINE,
  COMP_BAR_CHART,
  COMP_DONUT_CHART,
  COMP_FUNNEL_CHART,
  COMP_GAUGE,
  COMP_SCATTER_PLOT,
  COMP_HEATMAP,
  COMP_HISTOGRAM,
  COMP_BOX_PLOT,
  COMP_TREEMAP,
  COMP_NETWORK_GRAPH,
  COMP_LOG_VIEWER,
] as const;

// Fence markers for the client-side ```spec detection. A line whose trimmed form
// starts with SPEC_FENCE_OPEN opens a spec block; a line whose trimmed form
// equals SPEC_FENCE_CLOSE ends it. Everything between is JSONL (RFC-6902 patches).
export const SPEC_FENCE_OPEN = "```spec";
export const SPEC_FENCE_CLOSE = "```";

// data-jr-type is stamped on every rendered component's root element so e2e can
// assert a component tree rendered (and identify which component).
export const DATA_JR_TYPE = "data-jr-type";
