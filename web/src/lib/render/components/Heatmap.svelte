<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_HEATMAP, DATA_JR_TYPE } from "$lib/common/render";
  import {
    CHART_VIEWBOX_HEIGHT,
    CHART_VIEWBOX_WIDTH,
    chartHeight,
    extent,
    finiteNumber,
    formatChartNumber,
    scaleLinear,
  } from "$lib/render/charts/math";

  type HeatmapProps = {
    id?: string | null;
    title?: string | null;
    xLabels?: string[] | null;
    yLabels?: string[] | null;
    values?: number[][] | null;
    height?: number | null;
  };

  interface HeatCell {
    column: number;
    row: number;
    value: number;
  }

  const MAX_ROWS = 80;
  const MAX_COLUMNS = 80;
  const LABEL_WIDTH = 104;
  const LABEL_HEIGHT = 38;
  const PLOT_WIDTH = CHART_VIEWBOX_WIDTH - LABEL_WIDTH - 12;
  const PLOT_HEIGHT = CHART_VIEWBOX_HEIGHT - LABEL_HEIGHT - 12;

  const { props }: BaseComponentProps<HeatmapProps> = $props();

  const height = $derived(chartHeight(props.height));
  const rawRows = $derived(
    Array.isArray(props.values) ? props.values.slice(0, MAX_ROWS) : [],
  );
  const rowCount = $derived(
    Math.min(
      rawRows.length,
      Array.isArray(props.yLabels) ? props.yLabels.length : rawRows.length,
    ),
  );
  const widestRow = $derived(
    rawRows.reduce(
      (max, row) => (Array.isArray(row) ? Math.max(max, row.length) : max),
      0,
    ),
  );
  const columnCount = $derived(
    Math.min(
      MAX_COLUMNS,
      widestRow,
      Array.isArray(props.xLabels) ? props.xLabels.length : widestRow,
    ),
  );
  const xLabels = $derived(
    Array.from({ length: columnCount }, (_, index) =>
      Array.isArray(props.xLabels) && typeof props.xLabels[index] === "string"
        ? props.xLabels[index]
        : String(index + 1),
    ),
  );
  const yLabels = $derived(
    Array.from({ length: rowCount }, (_, index) =>
      Array.isArray(props.yLabels) && typeof props.yLabels[index] === "string"
        ? props.yLabels[index]
        : String(index + 1),
    ),
  );
  const cells = $derived.by((): HeatCell[] => {
    const result: HeatCell[] = [];
    for (let row = 0; row < rowCount; row += 1) {
      const values = Array.isArray(rawRows[row]) ? rawRows[row] : [];
      for (let column = 0; column < columnCount; column += 1) {
        const value = finiteNumber(values[column]);
        if (value === null) {
          continue;
        }
        result.push({ column, row, value });
      }
    }
    return result;
  });
  const domain = $derived(extent(cells.map((cell) => cell.value)));
  const cellWidth = $derived(
    columnCount > 0 ? PLOT_WIDTH / columnCount : PLOT_WIDTH,
  );
  const cellHeight = $derived(
    rowCount > 0 ? PLOT_HEIGHT / rowCount : PLOT_HEIGHT,
  );
  const xLabelStep = $derived(Math.max(1, Math.ceil(columnCount / 12)));
  const yLabelStep = $derived(Math.max(1, Math.ceil(rowCount / 14)));
  const description = $derived(
    `${rowCount} rows by ${columnCount} columns with ${cells.length} finite cells. Values range from ${formatChartNumber(domain[0])} to ${formatChartNumber(domain[1])}.`,
  );

  function shortLabel(label: string, maxCharacters: number): string {
    return label.length <= maxCharacters
      ? label
      : `${label.slice(0, maxCharacters - 1)}…`;
  }
</script>

<section
  class="jr-chart"
  id={props.id ?? undefined}
  style:height={`${height}px`}
  {...{ [DATA_JR_TYPE]: COMP_HEATMAP }}
>
  {#if props.title}<h3>{props.title}</h3>{/if}
  {#if cells.length === 0}
    <div class="jr-chart__empty">No heatmap data</div>
  {:else}
    <svg
      viewBox={`0 0 ${CHART_VIEWBOX_WIDTH} ${CHART_VIEWBOX_HEIGHT}`}
      role="img"
      aria-label={props.title || "Heatmap"}
    >
      <desc>{description}</desc>
      {#each xLabels as label, column}
        {#if column % xLabelStep === 0 || column === columnCount - 1}
          <text
            x={LABEL_WIDTH + column * cellWidth + cellWidth / 2}
            y={CHART_VIEWBOX_HEIGHT - 10}
            text-anchor="middle">{shortLabel(label, 8)}</text
          >
        {/if}
      {/each}
      {#each yLabels as label, row}
        {#if row % yLabelStep === 0 || row === rowCount - 1}
          <text
            x={LABEL_WIDTH - 8}
            y={12 + row * cellHeight + cellHeight / 2 + 4}
            text-anchor="end">{shortLabel(label, 14)}</text
          >
        {/if}
      {/each}
      {#each cells as cell}
        {@const opacity = scaleLinear(
          cell.value,
          domain[0],
          domain[1],
          0.18,
          1,
        )}
        <rect
          x={LABEL_WIDTH + cell.column * cellWidth}
          y={12 + cell.row * cellHeight}
          width={Math.max(0, cellWidth - 1)}
          height={Math.max(0, cellHeight - 1)}
          rx={Math.min(3, cellWidth / 6, cellHeight / 6)}
          fill="var(--accent)"
          fill-opacity={opacity}
        >
          <title
            >{yLabels[cell.row]} × {xLabels[cell.column]}: {formatChartNumber(
              cell.value,
            )}</title
          >
        </rect>
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
    font-family: var(--font-mono);
    font-size: 10px;
  }
  .jr-chart__empty {
    flex: 1;
    display: grid;
    place-items: center;
    color: var(--muted);
    font-size: var(--text-sm);
  }
</style>
