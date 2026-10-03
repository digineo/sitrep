import { describe, expect, it } from "vitest"

import type { PanelInfo } from "../shared/payload"
import { moveBy, moveTo, ordered } from "./order"

const p = (id: string, type: PanelInfo["type"]): PanelInfo => ({
  id,
  type,
  title: id,
})
const panels = [
  p("s1", "stat"),
  p("u1", "status"),
  p("s2", "stat"),
  p("s3", "stat"),
  p("t1", "timeseries"),
]

describe("panel order", () => {
  it("moves panels by places within their group", () => {
    expect(moveBy(panels, "s2", -1)).toEqual(["s2", "u1", "s1", "s3", "t1"])
    expect(moveBy(panels, "s1", 1)).toEqual(["s2", "u1", "s1", "s3", "t1"])
    expect(moveBy(panels, "s3", 1), "the last stays last")
      .toEqual(["s1", "u1", "s2", "s3", "t1"])
    expect(moveBy(panels, "u1", -1)).toEqual(["s1", "u1", "s2", "s3", "t1"])
  })

  it("moves panels onto others of their group", () => {
    expect(moveTo(panels, "s3", "s1")).toEqual(["s3", "u1", "s1", "s2", "t1"])
    expect(moveTo(panels, "s1", "s3")).toEqual(["s2", "u1", "s3", "s1", "t1"])
    expect(moveTo(panels, "s1", "t1"), "other groups are no targets")
      .toEqual(["s1", "u1", "s2", "s3", "t1"])
  })

  it("orders panels by IDs", () => {
    expect(ordered(panels, ["t1", "s1", "gone"]).map(x => x.id))
      .toEqual(["t1", "s1"])
  })
})
