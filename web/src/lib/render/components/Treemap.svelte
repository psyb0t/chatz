<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_TREEMAP, DATA_JR_TYPE } from "$lib/common/render";
  import {
    CHART_VIEWBOX_HEIGHT,
    CHART_VIEWBOX_WIDTH,
    chartHeight,
    finiteNumber,
    formatChartNumber,
  } from "$lib/render/charts/math";
  import { chartColor } from "$lib/render/charts/palette";
  import type { TreemapItem } from "$lib/render/charts/types";

  type TreemapProps = {
    id?: string | null;
    title?: string | null;
    items?: TreemapItem[] | null;
    height?: number | null;
  };
  interface SafeItem {
    label: string;
    value: number;
    group: string;
  }
  interface Tile extends SafeItem {
    x: number;
    y: number;
    width: number;
    height: number;
    colorIndex: number;
  }
  interface ItemGroup {
    items: SafeItem[];
    total: number;
  }

  const MAX_ITEMS = 200;
  const PADDING = 8;
  const MAP_WIDTH = CHART_VIEWBOX_WIDTH - PADDING * 2;
  const MAP_HEIGHT = CHART_VIEWBOX_HEIGHT - PADDING * 2;
  const { props }: BaseComponentProps<TreemapProps> = $props();
  const height = $derived(chartHeight(props.height));
  const items = $derived.by((): SafeItem[] => {
    if (!Array.isArray(props.items)) return [];
    return props.items.slice(0, MAX_ITEMS).flatMap((item, index) => {
      if (!item || typeof item !== "object") return [];
      const value = finiteNumber(item.value);
      if (value === null || value <= 0) return [];
      return [
        {
          label:
            typeof item.label === "string" && item.label
              ? item.label
              : String(index + 1),
          value,
          group: typeof item.group === "string" ? item.group : "",
        },
      ];
    });
  });
  const tiles = $derived.by((): Tile[] => {
    const total = items.reduce((sum, item) => sum + item.value, 0);
    if (!Number.isFinite(total) || total <= 0) return [];
    const grouped = new Map<string, ItemGroup>();
    for (const item of items) {
      const group = grouped.get(item.group) ?? { items: [], total: 0 };
      group.items.push(item);
      group.total += item.value;
      grouped.set(item.group, group);
    }

    const result: Tile[] = [];
    let groupOffset = 0;
    [...grouped.values()].forEach((group, colorIndex) => {
      const groupWidth = MAP_WIDTH * (group.total / total);
      let itemOffset = 0;
      for (const item of group.items) {
        const itemHeight = MAP_HEIGHT * (item.value / group.total);
        result.push({
          ...item,
          x: PADDING + groupOffset,
          y: PADDING + itemOffset,
          width: groupWidth,
          height: itemHeight,
          colorIndex,
        });
        itemOffset += itemHeight;
      }
      groupOffset += groupWidth;
    });
    return result;
  });
  const groupCount = $derived(new Set(tiles.map((tile) => tile.group)).size);
  const totalValue = $derived(tiles.reduce((sum, tile) => sum + tile.value, 0));
  const description = $derived(
    `${tiles.length} positive items across ${groupCount} groups, totaling ${formatChartNumber(totalValue)}. Groups are sliced horizontally and their items vertically by value.`,
  );
  function visibleLabel(tile: Tile): string {
    if (tile.width < 34 || tile.height < 22) return "";
    const maxCharacters = Math.max(1, Math.floor((tile.width - 12) / 7));
    return tile.label.length <= maxCharacters
      ? tile.label
      : `${tile.label.slice(0, Math.max(1, maxCharacters - 1))}…`;
  }
</script>

<section
  class="jr-chart"
  id={props.id ?? undefined}
  style:height={`${height}px`}
  {...{ [DATA_JR_TYPE]: COMP_TREEMAP }}
>
  {#if props.title}<h3>{props.title}</h3>{/if}
  {#if tiles.length === 0}
    <div class="jr-chart__empty">No treemap data</div>
  {:else}
    <svg
      viewBox={`0 0 ${CHART_VIEWBOX_WIDTH} ${CHART_VIEWBOX_HEIGHT}`}
      role="img"
      aria-label={props.title || "Treemap"}
    >
      <desc>{description}</desc>
      {#each tiles as tile}
        <rect
          x={tile.x}
          y={tile.y}
          width={Math.max(0, tile.width - 1)}
          height={Math.max(0, tile.height - 1)}
          fill={chartColor(tile.colorIndex)}
          fill-opacity="0.78"
          stroke="var(--panel)"
          stroke-width="2"
        >
          <title
            >{tile.label}{tile.group ? ` (${tile.group})` : ""}: {formatChartNumber(
              tile.value,
            )}</title
          >
        </rect>
        {#if visibleLabel(tile)}
          <text x={tile.x + 7} y={tile.y + 20}>{visibleLabel(tile)}</text>
          {#if tile.width >= 58 && tile.height >= 44}
            <text class="value" x={tile.x + 7} y={tile.y + 38}
              >{formatChartNumber(tile.value)}</text
            >
          {/if}
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
    fill: var(--on-accent);
    font-family: var(--font-display);
    font-size: 12px;
    font-weight: 600;
    pointer-events: none;
  }
  .value {
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 400;
  }
  .jr-chart__empty {
    flex: 1;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-size: var(--text-sm);
  }
</style>
