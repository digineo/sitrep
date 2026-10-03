import { describe, expect, it } from "vitest"

import {
  type IncidentsSection,
  merge,
  type PanelData,
  type SiteSection,
} from "./payload"

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
const incidents: IncidentsSection = {
  ongoing:  [],
  upcoming: [],
  finished: [],
  spans:    [],
}
const fresh = (value: number): PanelData => ({
  state:     "fresh",
  fetchedAt: "2026-10-02T12:00:00Z",
  data:      { value },
})
const full = () => merge(null, {
  cursor: "x.1",
  status: "operational",
  site,
  incidents,
  panels: {
    a: fresh(1),
    b: fresh(2),
  },
})

describe("merge", () => {
  it("takes a full response as it is", () => {
    expect(full()).toEqual({
      cursor: "x.1",
      status: "operational",
      site,
      incidents,
      panels: {
        a: fresh(1),
        b: fresh(2),
      },
    })
  })

  it("keeps sections that did not change", () => {
    const first = full()
    const next = merge(first, {
      cursor: "x.2",
      status: "down",
    })
    expect(next).toEqual({
      cursor: "x.2",
      status: "down",
      site,
      incidents,
      panels: first.panels,
    })
    expect(next.incidents).toBe(first.incidents)
  })

  it("replaces a changed incidents section", () => {
    const changed: IncidentsSection = {
      ...incidents,
      spans: [{ id: "i", title: "Outage", from: "2026-10-02T12:00:00Z" }],
    }
    const next = merge(full(), {
      cursor:    "x.2",
      status:    "degraded",
      incidents: changed,
    })
    expect(next.incidents).toBe(changed)
  })

  it("replaces the data of changed panels as a whole", () => {
    const next = merge(full(), {
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
    const smaller = {
      ...site,
      panels: [site.panels[1]!],
    }
    const next = merge(full(), {
      cursor: "x.2",
      status: "operational",
      site:   smaller,
    })
    expect(next.site).toBe(smaller)
    expect(next.panels).toEqual({ b: fresh(2) })
  })

  it("needs every section first", () => {
    expect(() => merge(null, {
      cursor: "x.1",
      status: "operational",
    })).toThrow()
    expect(() => merge(null, {
      cursor: "x.1",
      status: "operational",
      site,
    })).toThrow()
  })
})
