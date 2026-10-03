import { beforeEach, describe, expect, it } from "vitest"

import type { Payload, SiteBasics } from "../shared/payload"
import {
  applySite,
  failSite,
  HTTPError,
  live,
  siteData,
  unavailable,
} from "./siteData"

const payload: Payload = {
  cursor: "boot.1",
  status: "operational",
  site:   {
    name:      "Acme",
    theme:     "system",
    legal:     {},
    languages: ["en"],
    timezone:  "UTC",
    panels:    [],
  },
  incidents: {
    ongoing:  [],
    upcoming: [],
    finished: [],
    spans:    [],
  },
  panels: {},
}
const basics: SiteBasics = {
  name:      "Acme",
  theme:     "dark",
  legal:     { imprint: { mode: "text" } },
  languages: ["en"],
}

describe("site state", () => {
  beforeEach(() => {
    siteData.value = null
    unavailable.value = null
  })

  it("keeps the last good state when a refresh fails", () => {
    applySite({
      lang: "en",
      payload,
    })
    failSite(new HTTPError(500, "internal"))
    expect(siteData.value?.site.name).toBe("Acme")
    expect(live.state).toBe("failed")
  })

  it("replaces the state while the site is unavailable", () => {
    applySite({
      lang: "en",
      payload,
    })
    failSite(new HTTPError(503, "site_unavailable", basics))
    expect(siteData.value).toBeNull()
    expect(unavailable.value).toEqual(basics)

    applySite({
      lang: "en",
      payload,
    })
    expect(unavailable.value).toBeNull()
    expect(siteData.value?.cursor).toBe("boot.1")
  })
})
