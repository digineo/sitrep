import type { IncidentStatus, Severity } from "../shared/payload"

export interface Languages {
  enabled: string[]
  primary: string
}

export interface Settings {
  languages:    Languages
  defaultTheme: "light" | "dark" | "system"
}

export type Text = Record<string, string>

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
  allowedOrigins:        string[]
  /**
   * incidentRetentionDays keeps finished incidents that long; 0 keeps them
   * forever.
   */
  incidentRetentionDays: number
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
  id:        string
  name:      Text
  languages: Languages
  route:     Route
  status:       {
    overall:   State
    panels:    State
    incidents: State
  }
  /** missing counts the site and panels with missing translations. */
  missing: number
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
