import { describe, expect, it } from "vitest"

import {
  bands,
  bandsAt,
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

describe("bands", () => {
  // The plot spans 100 to 400 device pixels for the times 1000 to 4000.
  const toPos = (t: number) => 100 + (t - 1000) / 10
  const plot = {
    left:  100,
    width: 300,
  }
  const iso = (t: number) => new Date(t * 1000).toISOString()

  it("places closed and open spans", () => {
    expect(bands(
      [
        { id: "a", title: "A", severity: "critical", from: iso(2000), until: iso(2500) },
        { id: "b", title: "B", severity: "major", from: iso(3000) },
      ],
      toPos,
      plot,
    )).toEqual([
      { id: "a", title: "A", left: 200, width: 50, color: "#e5484d" },
      { id: "b", title: "B", left: 300, width: 100, color: "#e0a52b" },
    ])
  })

  it("clips to the plot and drops spans outside", () => {
    expect(bands(
      [
        { id: "a", title: "A", from: iso(0), until: iso(1500) },
        { id: "b", title: "B", from: iso(0), until: iso(900) },
        { id: "c", title: "C", from: iso(5000) },
      ],
      toPos,
      plot,
    )).toEqual([
      { id: "a", title: "A", left: 100, width: 50, color: "#8892a0" },
    ])
  })

  it("keeps short spans at least 2 pixels wide, inside the plot", () => {
    const short = bands(
      [{ id: "a", title: "A", from: iso(2000), until: iso(2001) }],
      toPos,
      plot,
    )
    expect(short[0]).toMatchObject({
      left:  200,
      width: 2,
    })

    const atEdge = bands(
      [{ id: "a", title: "A", from: iso(4000), until: iso(4000) }],
      toPos,
      plot,
    )
    expect(atEdge[0]).toMatchObject({
      left:  398,
      width: 2,
    })
  })

  it("finds the bands at a pixel", () => {
    const list = bands(
      [
        { id: "a", title: "A", from: iso(2000), until: iso(3000) },
        { id: "b", title: "B", from: iso(2500) },
      ],
      toPos,
      plot,
    )
    expect(bandsAt(list, 260).map(b => b.id)).toEqual(["a", "b"])
    expect(bandsAt(list, 150)).toEqual([])
  })
})
