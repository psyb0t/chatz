export const CHART_VIEWBOX_WIDTH = 720;
export const CHART_VIEWBOX_HEIGHT = 320;
export const CHART_MARGIN_TOP = 24;
export const CHART_MARGIN_RIGHT = 24;
export const CHART_MARGIN_BOTTOM = 48;
export const CHART_MARGIN_LEFT = 64;
export const CHART_DEFAULT_HEIGHT = 320;
export const CHART_MIN_HEIGHT = 180;
export const CHART_MAX_HEIGHT = 640;
export const CHART_TICK_COUNT = 5;

const compactNumber = new Intl.NumberFormat("en", {
  notation: "compact",
  maximumFractionDigits: 2,
});

export interface NumericPoint {
  x: number;
  y: number;
}

export function finiteNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

export function finiteValues(values: unknown): number[] {
  if (!Array.isArray(values)) {
    return [];
  }

  return values.flatMap((value) => {
    const finite = finiteNumber(value);
    return finite === null ? [] : [finite];
  });
}

export function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

export function chartHeight(value: unknown): number {
  const finite = finiteNumber(value);
  if (finite === null) {
    return CHART_DEFAULT_HEIGHT;
  }

  return clamp(Math.round(finite), CHART_MIN_HEIGHT, CHART_MAX_HEIGHT);
}

export function extent(
  values: readonly number[],
  includeZero = false,
): [number, number] {
  const finite = values.filter(Number.isFinite);
  if (finite.length === 0) {
    return [0, 1];
  }

  let min = Math.min(...finite);
  let max = Math.max(...finite);

  if (includeZero) {
    min = Math.min(min, 0);
    max = Math.max(max, 0);
  }

  if (min === max) {
    const padding = Math.abs(min) * 0.1 || 1;
    return [min - padding, max + padding];
  }

  return [min, max];
}

export function scaleLinear(
  value: number,
  domainMin: number,
  domainMax: number,
  rangeMin: number,
  rangeMax: number,
): number {
  if (domainMin === domainMax) {
    return (rangeMin + rangeMax) / 2;
  }

  const ratio = (value - domainMin) / (domainMax - domainMin);
  return rangeMin + ratio * (rangeMax - rangeMin);
}

export function ticks(
  min: number,
  max: number,
  count = CHART_TICK_COUNT,
): number[] {
  const safeCount = Math.max(2, Math.floor(count));
  const step = (max - min) / (safeCount - 1);
  return Array.from({ length: safeCount }, (_, index) => min + step * index);
}

export function linePath(points: readonly NumericPoint[]): string {
  return points
    .map((point, index) => `${index === 0 ? "M" : "L"}${point.x},${point.y}`)
    .join(" ");
}

export function areaPath(
  points: readonly NumericPoint[],
  baseline: number,
): string {
  if (points.length === 0) {
    return "";
  }

  const line = linePath(points);
  const first = points[0];
  const last = points[points.length - 1];
  return `${line} L${last.x},${baseline} L${first.x},${baseline} Z`;
}

export function formatChartNumber(value: number): string {
  return compactNumber.format(value);
}
