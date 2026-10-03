// Rules the admin console mirrors from the server, so that forms can
// validate and preview without a request.
import { catalogs } from "../shared/i18n"
import type { Languages, Route, Text } from "./types"

/**
 * resolveText returns a localized text in lang, else in the primary
 * language, else in the first enabled language that has a value.
 */
export function resolveText(
  text: Text | undefined,
  lang: string,
  l: Languages,
): string {
  return text?.[lang] || text?.[l.primary]
    || l.enabled.map(e => text?.[e]).find(Boolean) || ""
}

/** routeLabel returns how a site is reached, e.g. "/shop" or "shop.example.com". */
export function routeLabel(route: Route, baseDomains: string[]): string {
  switch (route.mode) {
    case "path":
      return `/${route.slug ?? ""}`
    case "subdomain":
      return `${route.slug ?? ""}.${baseDomains[0] ?? ""}`
    case "custom":
      return route.domain ?? ""
  }
}

/** siteURL returns the public URL of a site, as seen from location. */
export function siteURL(
  route: Route,
  baseDomains: string[],
  location: {
    protocol: string
    host:     string
    port:     string
  },
): string {
  const port = location.port ? `:${location.port}` : ""
  switch (route.mode) {
    case "path":
      return `${location.protocol}//${location.host}/${route.slug ?? ""}/`
    case "subdomain":
      return `${location.protocol}//${route.slug ?? ""}.${baseDomains[0] ?? ""}${port}/`
    case "custom":
      return `${location.protocol}//${route.domain ?? ""}/`
  }
}

const slugPattern = /^[a-z0-9]+(-[a-z0-9]+)*$/
const languagePattern = /^[a-z]{2}(-[a-z]{2})?$/
const labelPattern = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/
const reserved = ["admin", "api", "auth", "assets", "healthz", "tls"]

/** routeError returns the failing field and error code of a route, or null. */
export function routeError(
  route: Route,
  baseDomains: string[],
): {
  field: "slug" | "domain"
  code:  string
} | null {
  if (route.mode === "custom") {
    const domain = route.domain ?? ""
    const labels = domain.split(".")
    if (domain.length > 253 || labels.length < 2
      || !labels.every(l => l.length <= 63 && labelPattern.test(l))) {
      return {
        field: "domain",
        code:  "invalid_domain",
      }
    }

    const shadowed = baseDomains.some(base => domain === base
      || (domain.endsWith(`.${base}`)
        && !domain.slice(0, -base.length - 1).includes(".")))
    return shadowed
      ? {
        field: "domain",
        code:  "domain_reserved",
      }
      : null
  }

  const slug = route.slug ?? ""
  if (slug.length > 63 || !slugPattern.test(slug)) {
    return {
      field: "slug",
      code:  "invalid_slug",
    }
  }

  const legal = Object.values(catalogs)
    .flatMap(c => [c.legal.imprint.slug, c.legal.privacy.slug])
  if (route.mode === "path" && (reserved.includes(slug)
    || languagePattern.test(slug) || legal.includes(slug))) {
    return {
      field: "slug",
      code:  "slug_reserved",
    }
  }
  return null
}

const durationUnits = {
  d: 86400,
  h: 3600,
  m: 60,
  s: 1,
} as const

/**
 * parseDuration returns the seconds of a duration like "1h30m", or null.
 * Units may appear once each, in descending order.
 */
export function parseDuration(s: string): number | null {
  const m = /^(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/.exec(s)
  if (!s || !m) {
    return null
  }
  return Object.values(durationUnits).reduce(
    (sum, size, i) => sum + Number(m[i + 1] ?? 0) * size,
    0,
  )
}

/**
 * durationError returns the error code of a duration outside [min, max]
 * seconds, or null.
 */
export function durationError(s: string, min: number, max?: number): string | null {
  const d = parseDuration(s)
  if (d === null) {
    return "invalid_duration"
  }
  return d < min || (max !== undefined && d > max) ? "out_of_range" : null
}

/**
 * originError returns the error code of an origin "scheme://host[:port]"
 * with the scheme http or https, or null if it is valid. Paths, userinfo,
 * queries, fragments and wildcards are invalid.
 */
export function originError(origin: string): string | null {
  let url: URL
  try {
    url = new URL(origin.trim())
  } catch {
    return "invalid_origin"
  }

  const valid = (url.protocol === "http:" || url.protocol === "https:")
    && url.host !== "" && url.pathname === "/"
    && !/[@?#*\\]/.test(origin) && !/:\/?$/.test(origin.trim())
  return valid ? null : "invalid_origin"
}
