export interface XYPoint {
  x: number;
  y: number;
  label?: string | null;
}

export interface TimePoint {
  x: string;
  y: number;
}

export interface TimeSeries {
  name: string;
  points: TimePoint[];
}

export interface XYSeries {
  name: string;
  points: XYPoint[];
}

export interface CategorySeries {
  name: string;
  values: number[];
}

export interface LabelValue {
  label: string;
  value: number;
}

export interface BoxSummary {
  label: string;
  min: number;
  q1: number;
  median: number;
  q3: number;
  max: number;
}

export interface TreemapItem extends LabelValue {
  group?: string | null;
}

export interface NetworkNode {
  id: string;
  label: string;
  group?: string | null;
  value?: number | null;
}

export interface NetworkEdge {
  source: string;
  target: string;
  weight?: number | null;
}

export type LogLevel = "debug" | "info" | "warn" | "error";

export interface LogEntry {
  time: string;
  level: LogLevel;
  source?: string | null;
  message: string;
}
