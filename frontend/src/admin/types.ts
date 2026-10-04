import type { IncidentStatus, Severity } from "../shared/payload"

export interface Languages {
  enabled: string[]
  primary: string
}

export type Text = Record<string, string>

/**
 * LegalPage is a legal page: none, a link to a page elsewhere, or a
 * Markdown text. Sites may inherit the instance's page.
 */
export interface LegalPage {
  mode:  "inherit" | "none" | "url" | "text"
  url?:  Text
  text?: Text
}

export interface Legal {
  imprint: LegalPage
  privacy: LegalPage
}

export interface Settings {
  languages:    Languages
  defaultTheme: "light" | "dark" | "system"
  legal:        Legal
  /** landing is the Markdown text of the landing page. */
  landing?:     Text
  /** landingSite is the ID of the site shown instead of the landing page. */
  landingSite?: string
}

export type Availability = "online" | "offline" | "paused"

export type State = "operational" | "unknown" | "degraded" | "down"

export interface Route {
  mode:    "path" | "subdomain" | "custom"
  slug?:   string
  domain?: string
}

export interface Site {
  id:                    string
  name:                  Text
  languages:             Languages
  timezone:              string
  route:                 Route
  theme:                 "inherit" | "light" | "dark" | "system"
  /** brandColor is "#rrggbb". */
  brandColor?:           string
  /** logo is the sanitized SVG source of the logo. */
  logo?:                 string
  legal:                 Legal
  allowedOrigins:        string[]
  /**
   * incidentRetentionDays keeps finished incidents that long; 0 keeps them
   * forever.
   */
  incidentRetentionDays: number
  availability:          Availability
}

/** Person is the admin who created or edited something. */
export interface Person {
  subject:     string
  displayName: string
}

export interface IncidentUpdate {
  id:           string
  at:           string
  status?:      IncidentStatus
  severity?:    Severity
  /** description is Markdown; html holds it rendered, per language. */
  description?: Text
  html?:        Text
  author:       Person
  editedBy?:    Person
  editedAt?:    string
}

/**
 * Incident is an incident as the console sees it; its updates are ordered
 * by time.
 */
export interface Incident {
  id:           string
  title:        Text
  updates:      IncidentUpdate[]
  author:       Person
  status:       IncidentStatus
  severity?:    Severity
  phase:        "upcoming" | "ongoing" | "finished"
  lastActivity: string
}

export interface SiteSummary {
  id:           string
  name:         Text
  languages:    Languages
  route:        Route
  availability: Availability
  status:       {
    overall:   State
    panels:    State
    incidents: State
  }
  /** missing counts the site and panels with missing translations. */
  missing:  number
  /** landing is whether the site is promoted to the base domains. */
  landing?: boolean
}

export type PanelType = "stat" | "status" | "timeseries"

export interface Threshold {
  op:    "<" | "<=" | ">" | ">=" | "==" | "!="
  value: number
  state: "operational" | "degraded" | "down"
}

export interface Panel {
  id?:          string
  type:         PanelType
  title:        Text
  description?: Text
  datasource:   string
  query:        string
  refresh?:     string
  reduce?:      string
  decimals?:    number
  unit?:        Text
  thresholds?:  Threshold[]
  range?:       string
  step?:        string
  style?:       "line" | "area"
  minZero?:     boolean
  legend?:      Text
}

export interface Field {
  name:      string
  kind:      "text" | "url" | "secret" | "duration" | "select" | "bool"
  required?: boolean
  default?:  string
  options?:  string[]
  min?:      string
  max?:      string
  when?:     {
    field: string
    value: string
  }
}

export interface DataSourceType {
  id:         string
  fields:     Field[]
  panelTypes: PanelType[]
  editor?:    string
}

export interface DataSource {
  id:      string
  name:    string
  type:    string
  config:  Record<string, string>
  /** secrets lists the secret fields that have a value. */
  secrets: string[]
  summary: string
  usable:  boolean
  panels:  number
}
