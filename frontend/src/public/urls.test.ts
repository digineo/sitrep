import { describe, expect, it } from "vitest"

import { pagePath, pageRoutes } from "./urls"

describe("pagePath", () => {
  const incident = {
    kind: "incident" as const,
    id:   "0192",
  }

  it("omits the language with one enabled language", () => {
    expect(pagePath({ kind: "overview" }, "de", false)).toBe("/")
    expect(pagePath({ kind: "archive" }, "de", false)).toBe("/incidents")
    expect(pagePath(incident, "de", false)).toBe("/incidents/0192")
  })

  it("starts with the language with several enabled languages", () => {
    expect(pagePath({ kind: "overview" }, "en", true)).toBe("/en/")
    expect(pagePath(incident, "en", true)).toBe("/en/incidents/0192")
  })

  it("maps legal pages to the slugs of the language", () => {
    expect(pagePath({ kind: "imprint" }, "de", false)).toBe("/impressum")
    expect(pagePath({ kind: "privacy" }, "de", true)).toBe("/de/datenschutz")
    expect(pagePath({ kind: "imprint" }, "en", true)).toBe("/en/imprint")
    expect(pagePath({ kind: "privacy" }, "en", false)).toBe("/privacy")
  })
})

describe("pageRoutes", () => {
  const view = {}

  it("creates a route per view and language", () => {
    const routes = pageRoutes(["de", "en"], {
      overview: view,
      imprint:  view,
      incident: view,
    })
    expect(routes.map(r => [r.path, r.meta])).toEqual([
      ["/de/", { lang: "de", page: "overview" }],
      ["/de/impressum", { lang: "de", page: "imprint" }],
      ["/de/incidents/:id", { lang: "de", page: "incident" }],
      ["/en/", { lang: "en", page: "overview" }],
      ["/en/imprint", { lang: "en", page: "imprint" }],
      ["/en/incidents/:id", { lang: "en", page: "incident" }],
    ])
  })

  it("has no language segments with one language", () => {
    const routes = pageRoutes(["de"], {
      overview: view,
      privacy:  view,
    })
    expect(routes.map(r => r.path)).toEqual(["/", "/datenschutz"])
  })
})
