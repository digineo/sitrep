export interface Languages {
  enabled: string[]
  primary: string
}

export interface Settings {
  languages:    Languages
  defaultTheme: "light" | "dark" | "system"
}
