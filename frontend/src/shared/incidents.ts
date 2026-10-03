import type { Incident, IncidentStatus, Severity } from "./payload"

/** statusColors maps an incident status to its tag color. */
export const statusColors: Record<
  IncidentStatus,
  "info" | "danger" | "warning" | "link" | "success"
> = {
  planned:       "info",
  active:        "danger",
  investigating: "warning",
  monitoring:    "link",
  resolved:      "success",
}

/** severityColors maps a severity to its color in tags and charts. */
export const severityColors: Record<Severity, string> = {
  minor:    "#8892a0",
  major:    "#e0a52b",
  critical: "#e5484d",
}

/** plainText returns the text of rendered HTML with whitespace collapsed. */
export function plainText(html: string): string {
  const doc = new DOMParser().parseFromString(html, "text/html")
  const text = doc.body.textContent ?? ""
  return text.replace(/\s+/g, " ").trim()
}

/**
 * cut shortens text to limit code points, at a word boundary where
 * possible, and marks a cut with an ellipsis.
 */
export function cut(text: string, limit = 200): string {
  const chars = Array.from(text)
  if (chars.length <= limit) {
    return text
  }

  let end = limit
  if (chars[limit] !== " ") {
    const space = chars.lastIndexOf(" ", limit - 1)
    if (space > 0) {
      end = space
    }
  }
  return chars.slice(0, end).join("").trimEnd() + "…"
}

/** excerpt returns the cut plain text of the newest update with a description. */
export function excerpt(incident: Pick<Incident, "updates">): string {
  for (const u of incident.updates.toReversed()) {
    const text = plainText(u.html)
    if (text) {
      return cut(text)
    }
  }
  return ""
}
