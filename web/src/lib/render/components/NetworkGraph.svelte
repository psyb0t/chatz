<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_NETWORK_GRAPH, DATA_JR_TYPE } from "$lib/common/render";
  import {
    CHART_VIEWBOX_HEIGHT,
    CHART_VIEWBOX_WIDTH,
    chartHeight,
    clamp,
    finiteNumber,
    formatChartNumber,
  } from "$lib/render/charts/math";
  import { CHART_GRID_COLOR, chartColor } from "$lib/render/charts/palette";
  import type { NetworkEdge, NetworkNode } from "$lib/render/charts/types";

  type NetworkGraphProps = {
    id?: string | null;
    title?: string | null;
    nodes?: NetworkNode[] | null;
    edges?: NetworkEdge[] | null;
    height?: number | null;
  };
  interface PositionedNode {
    id: string;
    label: string;
    group: string;
    value: number | null;
    x: number;
    y: number;
    colorIndex: number;
    radius: number;
  }
  interface SafeEdge {
    source: PositionedNode;
    target: PositionedNode;
    weight: number;
  }

  const MAX_NODES = 100;
  const MAX_EDGES = 300;
  const CENTER_X = CHART_VIEWBOX_WIDTH / 2;
  const CENTER_Y = CHART_VIEWBOX_HEIGHT / 2;
  const GROUP_RADIUS_X = 220;
  const GROUP_RADIUS_Y = 92;
  const NODE_CLUSTER_RADIUS = 42;
  const { props }: BaseComponentProps<NetworkGraphProps> = $props();
  const height = $derived(chartHeight(props.height));
  const nodes = $derived.by((): PositionedNode[] => {
    if (!Array.isArray(props.nodes)) return [];
    const unique = new Map<
      string,
      { id: string; label: string; group: string; value: number | null }
    >();
    for (const candidate of props.nodes.slice(0, MAX_NODES)) {
      if (
        !candidate ||
        typeof candidate !== "object" ||
        typeof candidate.id !== "string" ||
        !candidate.id ||
        unique.has(candidate.id)
      )
        continue;
      unique.set(candidate.id, {
        id: candidate.id,
        label:
          typeof candidate.label === "string" && candidate.label
            ? candidate.label
            : candidate.id,
        group: typeof candidate.group === "string" ? candidate.group : "",
        value: finiteNumber(candidate.value),
      });
    }
    const grouped = new Map<
      string,
      Array<{ id: string; label: string; group: string; value: number | null }>
    >();
    for (const node of unique.values()) {
      const members = grouped.get(node.group) ?? [];
      members.push(node);
      grouped.set(node.group, members);
    }
    const result: PositionedNode[] = [];
    const groupEntries = [...grouped.entries()];
    groupEntries.forEach(([, members], groupIndex) => {
      const groupAngle =
        groupEntries.length === 1
          ? 0
          : (Math.PI * 2 * groupIndex) / groupEntries.length - Math.PI / 2;
      const groupX =
        groupEntries.length === 1
          ? CENTER_X
          : CENTER_X + Math.cos(groupAngle) * GROUP_RADIUS_X;
      const groupY =
        groupEntries.length === 1
          ? CENTER_Y
          : CENTER_Y + Math.sin(groupAngle) * GROUP_RADIUS_Y;
      members.forEach((node, nodeIndex) => {
        const nodeAngle =
          members.length === 1 ? 0 : (Math.PI * 2 * nodeIndex) / members.length;
        const spread = members.length === 1 ? 0 : NODE_CLUSTER_RADIUS;
        result.push({
          ...node,
          x: groupX + Math.cos(nodeAngle) * spread,
          y: groupY + Math.sin(nodeAngle) * spread,
          colorIndex: groupIndex,
          radius:
            node.value === null
              ? 7
              : clamp(6 + Math.sqrt(Math.max(0, node.value)), 6, 16),
        });
      });
    });
    return result;
  });
  const edges = $derived.by((): SafeEdge[] => {
    if (!Array.isArray(props.edges)) return [];
    const byID = new Map(nodes.map((node) => [node.id, node]));
    return props.edges.slice(0, MAX_EDGES).flatMap((edge): SafeEdge[] => {
      if (
        !edge ||
        typeof edge !== "object" ||
        typeof edge.source !== "string" ||
        typeof edge.target !== "string" ||
        edge.source === edge.target
      )
        return [];
      const source = byID.get(edge.source);
      const target = byID.get(edge.target);
      if (!source || !target) return [];
      const suppliedWeight =
        edge.weight === null || edge.weight === undefined
          ? 1
          : finiteNumber(edge.weight);
      if (suppliedWeight === null || suppliedWeight <= 0) return [];
      return [{ source, target, weight: suppliedWeight }];
    });
  });
  const labelStep = $derived(Math.max(1, Math.ceil(nodes.length / 24)));
  const groupCount = $derived(new Set(nodes.map((node) => node.group)).size);
  const description = $derived(
    `${nodes.length} unique nodes in ${groupCount} groups connected by ${edges.length} valid edges. Nodes are clustered deterministically by group.`,
  );

  function shortLabel(label: string): string {
    const maxCharacters = 14;
    return label.length <= maxCharacters
      ? label
      : `${label.slice(0, maxCharacters - 1)}…`;
  }
</script>

<section
  class="jr-chart"
  id={props.id ?? undefined}
  style:height={`${height}px`}
  {...{ [DATA_JR_TYPE]: COMP_NETWORK_GRAPH }}
>
  {#if props.title}<h3>{props.title}</h3>{/if}
  {#if nodes.length === 0}
    <div class="jr-chart__empty">No network data</div>
  {:else}
    <svg
      viewBox={`0 0 ${CHART_VIEWBOX_WIDTH} ${CHART_VIEWBOX_HEIGHT}`}
      role="img"
      aria-label={props.title || "Network graph"}
    >
      <desc>{description}</desc>
      {#each edges as edge}
        <line
          x1={edge.source.x}
          y1={edge.source.y}
          x2={edge.target.x}
          y2={edge.target.y}
          stroke={CHART_GRID_COLOR}
          stroke-width={clamp(Math.sqrt(edge.weight), 1, 6)}
          stroke-opacity="0.8"
        >
          <title
            >{edge.source.label} → {edge.target.label}, weight {formatChartNumber(
              edge.weight,
            )}</title
          >
        </line>
      {/each}
      {#each nodes as node, index}
        <circle
          cx={node.x}
          cy={node.y}
          r={node.radius}
          fill={chartColor(node.colorIndex)}
          stroke="var(--panel)"
          stroke-width="2"
        >
          <title
            >{node.label}{node.group ? ` (${node.group})` : ""}{node.value ===
            null
              ? ""
              : `: ${formatChartNumber(node.value)}`}</title
          >
        </circle>
        {#if index % labelStep === 0 || index === nodes.length - 1}
          <text x={node.x} y={node.y + node.radius + 13} text-anchor="middle"
            >{shortLabel(node.label)}</text
          >
        {/if}
      {/each}
    </svg>
  {/if}
</section>

<style>
  .jr-chart {
    min-width: 0;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    overflow: hidden;
  }
  h3 {
    font-size: var(--text-sm);
  }
  svg {
    width: 100%;
    min-height: 0;
    flex: 1;
  }
  text {
    fill: var(--muted);
    font-family: var(--font-display);
    font-size: 10px;
    paint-order: stroke;
    stroke: var(--panel);
    stroke-width: 3px;
    stroke-linejoin: round;
  }
  .jr-chart__empty {
    flex: 1;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-size: var(--text-sm);
  }
</style>
