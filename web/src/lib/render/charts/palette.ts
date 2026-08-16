export const CHART_PALETTE = [
  "var(--accent)",
  "var(--ok)",
  "var(--warn)",
  "var(--crit)",
  "var(--muted)",
  "var(--faint)",
] as const;

export const CHART_GRID_COLOR = "var(--border)";
export const CHART_TEXT_COLOR = "var(--muted)";
export const CHART_SURFACE_COLOR = "var(--panel-2)";

export function chartColor(index: number): string {
  const safeIndex = Number.isFinite(index) ? Math.abs(Math.trunc(index)) : 0;
  return CHART_PALETTE[safeIndex % CHART_PALETTE.length];
}
