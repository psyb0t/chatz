import { describe, expect, it } from "vitest";
import {
  areaPath,
  chartHeight,
  clamp,
  extent,
  finiteNumber,
  finiteValues,
  linePath,
  scaleLinear,
  ticks,
} from "./math";

describe("chart math", () => {
  it("filters non-finite and non-numeric values", () => {
    expect(finiteNumber(3)).toBe(3);
    expect(finiteNumber(Number.NaN)).toBeNull();
    expect(finiteNumber(Number.POSITIVE_INFINITY)).toBeNull();
    expect(finiteNumber("3")).toBeNull();
    expect(finiteValues([1, Number.NaN, "2", 3])).toEqual([1, 3]);
  });

  it("clamps values and chart heights", () => {
    expect(clamp(9, 0, 5)).toBe(5);
    expect(chartHeight(undefined)).toBe(320);
    expect(chartHeight(10)).toBe(180);
    expect(chartHeight(900)).toBe(640);
  });

  it("creates useful extents for empty, constant, and mixed data", () => {
    expect(extent([])).toEqual([0, 1]);
    expect(extent([5])).toEqual([4.5, 5.5]);
    expect(extent([-2, 4], true)).toEqual([-2, 4]);
    expect(extent([2, 4], true)).toEqual([0, 4]);
  });

  it("scales constant and regular domains", () => {
    expect(scaleLinear(5, 0, 10, 0, 100)).toBe(50);
    expect(scaleLinear(5, 5, 5, 0, 100)).toBe(50);
  });

  it("creates ticks and SVG paths", () => {
    expect(ticks(0, 8, 3)).toEqual([0, 4, 8]);
    expect(
      linePath([
        { x: 1, y: 2 },
        { x: 3, y: 4 },
      ]),
    ).toBe("M1,2 L3,4");
    expect(
      areaPath(
        [
          { x: 1, y: 2 },
          { x: 3, y: 4 },
        ],
        10,
      ),
    ).toBe("M1,2 L3,4 L3,10 L1,10 Z");
  });
});
