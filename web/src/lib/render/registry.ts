// The json-render registry: binds each catalog component type to its Svelte
// implementation. defineRegistry returns { registry } which the Renderer feeds
// to json-render's <Renderer>. Component keys MUST match the catalog keys (both
// come from $lib/common/render constants).
import { defineRegistry } from "@json-render/svelte";
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
import { genUICatalog } from "./catalog";

import TextComponent from "./components/Text.svelte";
import HeadingComponent from "./components/Heading.svelte";
import CardComponent from "./components/Card.svelte";
import StackComponent from "./components/Stack.svelte";
import GridComponent from "./components/Grid.svelte";
import StatComponent from "./components/Stat.svelte";
import BadgeComponent from "./components/Badge.svelte";
import TableComponent from "./components/Table.svelte";
import KeyValueComponent from "./components/KeyValue.svelte";
import CalloutComponent from "./components/Callout.svelte";
import TimelineComponent from "./components/Timeline.svelte";
import ProgressComponent from "./components/Progress.svelte";
import TimeSeriesChartComponent from "./components/TimeSeriesChart.svelte";
import AreaChartComponent from "./components/AreaChart.svelte";
import SparklineComponent from "./components/Sparkline.svelte";
import BarChartComponent from "./components/BarChart.svelte";
import DonutChartComponent from "./components/DonutChart.svelte";
import FunnelChartComponent from "./components/FunnelChart.svelte";
import GaugeComponent from "./components/Gauge.svelte";
import ScatterPlotComponent from "./components/ScatterPlot.svelte";
import HeatmapComponent from "./components/Heatmap.svelte";
import HistogramComponent from "./components/Histogram.svelte";
import BoxPlotComponent from "./components/BoxPlot.svelte";
import TreemapComponent from "./components/Treemap.svelte";
import NetworkGraphComponent from "./components/NetworkGraph.svelte";
import LogViewerComponent from "./components/LogViewer.svelte";

const components = {
  [COMP_TEXT]: TextComponent,
  [COMP_HEADING]: HeadingComponent,
  [COMP_CARD]: CardComponent,
  [COMP_STACK]: StackComponent,
  [COMP_GRID]: GridComponent,
  [COMP_STAT]: StatComponent,
  [COMP_BADGE]: BadgeComponent,
  [COMP_TABLE]: TableComponent,
  [COMP_KEY_VALUE]: KeyValueComponent,
  [COMP_CALLOUT]: CalloutComponent,
  [COMP_TIMELINE]: TimelineComponent,
  [COMP_PROGRESS]: ProgressComponent,
  [COMP_TIME_SERIES_CHART]: TimeSeriesChartComponent,
  [COMP_AREA_CHART]: AreaChartComponent,
  [COMP_SPARKLINE]: SparklineComponent,
  [COMP_BAR_CHART]: BarChartComponent,
  [COMP_DONUT_CHART]: DonutChartComponent,
  [COMP_FUNNEL_CHART]: FunnelChartComponent,
  [COMP_GAUGE]: GaugeComponent,
  [COMP_SCATTER_PLOT]: ScatterPlotComponent,
  [COMP_HEATMAP]: HeatmapComponent,
  [COMP_HISTOGRAM]: HistogramComponent,
  [COMP_BOX_PLOT]: BoxPlotComponent,
  [COMP_TREEMAP]: TreemapComponent,
  [COMP_NETWORK_GRAPH]: NetworkGraphComponent,
  [COMP_LOG_VIEWER]: LogViewerComponent,
};

export const { registry } = defineRegistry(genUICatalog, { components });
