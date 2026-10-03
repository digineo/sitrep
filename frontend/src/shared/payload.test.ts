import { describe, expect, it } from "vitest"

import { merge, type PanelData, type SiteSection } from "./payload"

const site: SiteSection = {
  name:      "Acme",
  theme:     "system",
  languages: ["en"],
  timezone:  "UTC",
  panels:    [
    { id: "a", type: "stat", title: "A" },
    { id: "b", type: "stat", title: "B" },
  ],
}
const fresh = (value: number): PanelData => ({
  state:     "fresh",
  fetchedAt: "2026-10-02T12:00:00Z",
  data:      { value },
})

describe("merge", () => {
  it("takes a full response as it is", () => {
    const state = merge(null, {
      cursor: "x.1",
      status: "operational",
      site,
      panels: {
        a: fresh(1),
        b: fresh(2),
      },
    })
    expect(state).toEqual({
      cursor: "x.1",
      status: "operational",
      site,
      panels: {
        a: fresh(1),
        b: fresh(2),
      },
    })
  })

  it("keeps sections that did not change", () => {
    const first = merge(null, {
      cursor: "x.1",
      status: "operational",
      site,
      panels: {
        a: fresh(1),
        b: fresh(2),
      },
    })
    const next = merge(first, {
      cursor: "x.2",
      status: "down",
    })
    expect(next).toEqual({
      cursor: "x.2",
      status: "down",
      site,
      panels: first.panels,
    })
  })

  it("replaces the data of changed panels as a whole", () => {
    const first = merge(null, {
      cursor: "x.1",
      status: "operational",
      site,
      panels: {
        a: fresh(1),
        b: fresh(2),
      },
    })
    const next = merge(first, {
      cursor: "x.2",
      status: "operational",
      panels: { b: { state: "pending" } },
    })
    expect(next.panels).toEqual({
      a: fresh(1),
      b: { state: "pending" },
    })
  })

  it("drops panels the site no longer lists", () => {
    const first = merge(null, {
      cursor: "x.1",
      status: "operational",
      site,
      panels: {
        a: fresh(1),
        b: fresh(2),
      },
    })
    const smaller = {
      ...site,
      panels: [site.panels[1]!],
    }
    const next = merge(first, {
      cursor: "x.2",
      status: "operational",
      site:   smaller,
    })
    expect(next.site).toBe(smaller)
    expect(next.panels).toEqual({ b: fresh(2) })
  })

  it("needs the site section first", () => {
    expect(() => merge(null, {
      cursor: "x.1",
      status: "operational",
    })).toThrow()
  })
})
