/** State is the status of a site or a status panel, by ascending severity. */
export type State = "operational" | "unknown" | "degraded" | "down"

export type PanelType = "stat" | "status" | "timeseries"

/** PanelInfo is a panel's presentation, in display order. */
export interface PanelInfo {
  id:           string
  type:         PanelType
  title:        string
  description?: string
  unit?:        string
  decimals?:    number
  style?:       "line" | "area"
  minZero?:     boolean
  /** range is the time range of a chart in seconds. */
  range?:       number
}

export interface SiteSection {
  name:      string
  theme:     "light" | "dark" | "system"
  languages: string[]
  timezone:  string
  panels:    PanelInfo[]
}

export interface StatData {
  value: number | null
}

export interface StatusData {
  state: State
}

export interface SeriesData {
  /** times are Unix seconds, shared by all series. */
  times:  number[]
  series: {
    name:   string
    values: (number | null)[]
  }[]
}

/** Warning is a hint about a panel's data, e.g. dropped series. */
export interface Warning {
  code:   "series_dropped" | "response_large"
  count?: number
  /** size is the size of the data source's response in bytes. */
  size?:  number
}

export interface PanelData {
  state:      "pending" | "fresh" | "stale"
  fetchedAt?: string
  data?:      StatData | StatusData | SeriesData
  /** error is the latest poll's error, in admin previews only. */
  error?:     string
  /** warnings are hints about the data, in admin previews only. */
  warnings?:  Warning[]
}

export type IncidentStatus = "planned"
  | "active"
  | "investigating"
  | "monitoring"
  | "resolved"

export type Severity = "minor" | "major" | "critical"

/** IncidentUpdate is one entry of an incident's timeline. */
export interface IncidentUpdate {
  id:        string
  at:        string
  status?:   IncidentStatus
  severity?: Severity
  /** html is the rendered description. */
  html:      string
}

/**
 * Incident is an incident as visitors see it; its updates are ordered by
 * time.
 */
export interface Incident {
  id:           string
  title:        string
  phase:        "upcoming" | "ongoing" | "finished"
  status:       IncidentStatus
  severity?:    Severity
  lastActivity: string
  updates:      IncidentUpdate[]
}

/**
 * Span is an incident's time range in charts; without until, the incident is
 * ongoing.
 */
export interface Span {
  id:        string
  title:     string
  severity?: Severity
  from:      string
  until?:    string
}

/**
 * IncidentsSection holds the public incidents by phase, most recent activity
 * first.
 */
export interface IncidentsSection {
  ongoing:  Incident[]
  upcoming: Incident[]
  finished: Incident[]
  spans:    Span[]
}

/** Payload is one response of a site payload request. */
export interface Payload {
  cursor:     string
  status:     State
  site?:      SiteSection
  incidents?: IncidentsSection
  panels?:    Record<string, PanelData>
}

/** SiteData is the state of a site page, merged from payload responses. */
export interface SiteData {
  cursor:    string
  status:    State
  site:      SiteSection
  incidents: IncidentsSection
  panels:    Record<string, PanelData>
}

/**
 * merge applies a response to the previous state. Responses only carry the
 * sections changed since the previous one; the site section's panel list
 * is authoritative, so data of panels no longer listed is dropped.
 */
export function merge(prev: SiteData | null, next: Payload): SiteData {
  const site = next.site ?? prev?.site
  const incidents = next.incidents ?? prev?.incidents
  if (!site || !incidents) {
    throw new Error("the first response must carry every section")
  }

  const panels: Record<string, PanelData> = {}
  for (const { id } of site.panels) {
    const data = next.panels?.[id] ?? prev?.panels[id]
    if (data) {
      panels[id] = data
    }
  }
  return {
    cursor: next.cursor,
    status: next.status,
    site,
    incidents,
    panels,
  }
}
