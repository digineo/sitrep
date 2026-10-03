import { describe, expect, it } from "vitest"

import {
  fractionDigits,
  latest,
  palette,
  seriesColor,
  toggled,
  tooltipPosition,
  yRange,
} from "./chart"

describe("chart helpers", () => {
  it("starts the y-axis at zero unless values are negative", () => {
    expect(yRange(10, 20, true)).toEqual([0, 20.5])
    expect(yRange(10, 20, false)).toEqual([9.5, 20.5])
    expect(yRange(-10, 10, true)).toEqual([-11, 11])
    expect(yRange(5, 5, false)).toEqual([4.75, 5.25])
    expect(yRange(0, 0, true)).toEqual([0, 0.05])
    expect(yRange(null, null, true)).toEqual([0, 1])
  })

  it("uses as many fraction digits as the tick spacing needs", () => {
    expect(fractionDigits(1000)).toBe(0)
    expect(fractionDigits(1)).toBe(0)
    expect(fractionDigits(0.5)).toBe(1)
    expect(fractionDigits(0.25)).toBe(2)
    expect(fractionDigits(0.1 + 0.2)).toBe(1)
    expect(fractionDigits(1e-9)).toBe(6)
  })

  it("flips the tooltip at the edges", () => {
    const area = {
      width:  400,
      height: 200,
    }
    const box = {
      width:  100,
      height: 50,
    }
    expect(tooltipPosition(10, 10, box, area)).toEqual({
      left: 22,
      top:  22,
    })
    expect(tooltipPosition(350, 180, box, area)).toEqual({
      left: 238,
      top:  118,
    })

    const wide = {
      width:  500,
      height: 50,
    }
    expect(tooltipPosition(50, 10, wide, area)).toEqual({
      left: 0,
      top:  22,
    })
  })

  it("toggles hidden series by name", () => {
    expect(toggled([], "a")).toEqual(["a"])
    expect(toggled(["a", "b"], "a")).toEqual(["b"])
  })

  it("cycles through the palette", () => {
    expect(seriesColor(0)).toBe(palette[0])
    expect(seriesColor(palette.length)).toBe(palette[0])
  })

  it("finds the latest value of each series", () => {
    expect(latest({
      times:  [1, 2, 3],
      series: [
        { name: "a", values: [1, 2, null] },
        { name: "b", values: [null, null, null] },
      ],
    })).toEqual([2, null])
  })
})
