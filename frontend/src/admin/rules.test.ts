import { describe, expect, it } from "vitest"

import {
  durationError,
  originError,
  parseDuration,
  resolveText,
  routeError,
  routeLabel,
  siteURL,
} from "./rules"

const bases = ["status.example.com", "sitrep.localhost"]

describe("resolveText", () => {
  const l = {
    enabled: ["de", "en", "fr"],
    primary: "en",
  }
  it("falls back like the server", () => {
    const deEn = {
      de: "Hallo",
      en: "Hello",
    }
    const enFr = {
      en: "Hello",
      fr: "Bonjour",
    }

    expect(resolveText(deEn, "de", l)).toBe("Hallo")
    expect(resolveText(enFr, "de", l)).toBe("Hello")
    expect(resolveText({ fr: "Bonjour" }, "de", l)).toBe("Bonjour")
    expect(resolveText({ it: "Ciao" }, "de", l)).toBe("")
    expect(resolveText(undefined, "de", l)).toBe("")
  })
})

describe("routes", () => {
  it("labels and links routes", () => {
    const location = {
      protocol: "https:",
      host:     "status.example.com:8443",
      port:     "8443",
    }
    const pathRoute = {
      mode: "path" as const,
      slug: "shop",
    }
    const subdomainRoute = {
      mode: "subdomain" as const,
      slug: "shop",
    }
    const customRoute = {
      mode:   "custom" as const,
      domain: "status.shop.com",
    }

    expect(routeLabel(pathRoute, bases)).toBe("/shop")
    expect(routeLabel(subdomainRoute, bases)).toBe("shop.status.example.com")
    expect(routeLabel(customRoute, bases)).toBe("status.shop.com")

    expect(siteURL(pathRoute, bases, location))
      .toBe("https://status.example.com:8443/shop/")
    expect(siteURL(subdomainRoute, bases, location))
      .toBe("https://shop.status.example.com:8443/")
    expect(siteURL(customRoute, bases, location)).toBe("https://status.shop.com/")
  })

  it.each([
    [{ mode: "path", slug: "shop" }, null],
    [{ mode: "path", slug: "my-shop2" }, null],
    [{ mode: "path", slug: "" }, "invalid_slug"],
    [{ mode: "path", slug: "Shop" }, "invalid_slug"],
    [{ mode: "path", slug: "a--b" }, "invalid_slug"],
    [{ mode: "path", slug: "a".repeat(64) }, "invalid_slug"],
    [{ mode: "path", slug: "admin" }, "slug_reserved"],
    [{ mode: "path", slug: "fr" }, "slug_reserved"],
    [{ mode: "path", slug: "pt-br" }, "slug_reserved"],
    [{ mode: "path", slug: "impressum" }, "slug_reserved"],
    [{ mode: "path", slug: "privacy" }, "slug_reserved"],
    [{ mode: "subdomain", slug: "admin" }, null],
    [{ mode: "custom", domain: "status.shop.com" }, null],
    [{ mode: "custom", domain: "a.shop.sitrep.localhost" }, null],
    [{ mode: "custom", domain: "localhost" }, "invalid_domain"],
    [{ mode: "custom", domain: "Status.shop.com" }, "invalid_domain"],
    [{ mode: "custom", domain: "-a.shop.com" }, "invalid_domain"],
    [{ mode: "custom", domain: "shop.com:443" }, "invalid_domain"],
    [{ mode: "custom", domain: "status.example.com" }, "domain_reserved"],
    [{ mode: "custom", domain: "shop.sitrep.localhost" }, "domain_reserved"],
  ] as const)("validates %o", (route, code) => {
    expect(routeError(route, bases)?.code ?? null).toBe(code)
  })
})

describe("durations", () => {
  it("parses the server's syntax", () => {
    expect(parseDuration("90s")).toBe(90)
    expect(parseDuration("1h30m")).toBe(5400)
    expect(parseDuration("7d")).toBe(604800)
    expect(parseDuration("1d2h3m4s")).toBe(93784)
    for (const bad of ["", "30", "1m1h", "1h1h", "1.5h", "1w", " 1h"]) {
      expect(parseDuration(bad), bad).toBeNull()
    }
  })

  it("checks bounds", () => {
    expect(durationError("5s", 5, 86400)).toBeNull()
    expect(durationError("4s", 5, 86400)).toBe("out_of_range")
    expect(durationError("1d1s", 5, 86400)).toBe("out_of_range")
    expect(durationError("1x", 5)).toBe("invalid_duration")
  })
})

describe("originError", () => {
  it("accepts origins like the server", () => {
    for (const origin of [
      "https://www.example.com",
      "HTTP://Example.com:8080/",
      "https://example.com:443",
      "http://[::1]:3000",
    ]) {
      expect(originError(origin), origin).toBeNull()
    }
  })

  it("rejects everything else", () => {
    for (const origin of [
      "example.com",
      "ftp://example.com",
      "https://example.com/path",
      "https://user@example.com",
      "https://example.com?q",
      "https://example.com#f",
      "https://*.example.com",
      "https://example.com:",
      "https://example.com//",
    ]) {
      expect(originError(origin), origin).toBe("invalid_origin")
    }
  })
})
