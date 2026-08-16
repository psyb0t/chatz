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
  COMP_TIME_SERIES_CHART,
  COMP_TREEMAP,
} from "$lib/common/render";

// Every analytical GenUI component, in catalog registration order. Shared by
// analytics.test.ts (mounts + mutates each through the real registry) and
// catalog.test.ts (asserts catalog membership) so the two lists can't drift.
export const ANALYTICAL_COMPONENTS = [
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

// The canned spec mirroring what the backend showcase streams: a Card
// wrapping a Grid of two Stats + a Badge, plus a Table. Emitted as ```spec
// fenced JSONL with every props.id null (the frontend stamps ids from map
// keys). Shared by catalog.test.ts (parse + assemble) and renderer.test.ts
// (data-jr-type contract), which mounts a one-table-row variant — see
// DEMO_MESSAGE_NO_INTRO_ONE_ROW below.
export const DEMO_MESSAGE = [
  "Here's the snapshot:",
  "```spec",
  '{"op":"add","path":"/root","value":"card"}',
  '{"op":"add","path":"/elements/card","value":{"type":"Card","props":{"id":null,"title":"Portfolio"},"children":["grid","tbl"]}}',
  '{"op":"add","path":"/elements/grid","value":{"type":"Grid","props":{"id":null,"columns":3},"children":["s1","s2","b1"]}}',
  '{"op":"add","path":"/elements/s1","value":{"type":"Stat","props":{"id":null,"label":"Occupancy","value":"92","unit":"%","delta":4},"children":[]}}',
  '{"op":"add","path":"/elements/s2","value":{"type":"Stat","props":{"id":null,"label":"Vacancy","value":"8","unit":"%","delta":-2},"children":[]}}',
  '{"op":"add","path":"/elements/b1","value":{"type":"Badge","props":{"id":null,"label":"LIVE","variant":"ok"},"children":[]}}',
  '{"op":"add","path":"/elements/tbl","value":{"type":"Table","props":{"id":null,"columns":["Unit","Status"],"rows":[["A-101","Booked"],["A-102","Open"]]},"children":[]}}',
  "```",
].join("\n");

// renderer.test.ts's variant: drops the leading prose line (it only cares
// about the spec segment) and trims the Table to one row (it only asserts
// element counts, not row content). Expressed as an explicit diff against
// DEMO_MESSAGE so the two fixtures can't silently drift apart.
export const DEMO_MESSAGE_NO_INTRO_ONE_ROW = DEMO_MESSAGE.replace(
  "Here's the snapshot:\n",
  "",
).replace('["A-101","Booked"],["A-102","Open"]', '["A-101","Booked"]');
