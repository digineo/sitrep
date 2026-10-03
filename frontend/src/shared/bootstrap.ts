/** Bootstrap is the data the server renders into every page shell. */
export interface Bootstrap {
  mode:            "site" | "landing" | "admin"
  siteId?:         string
  basePath:        string
  lang:            string
  languages:       string[]
  primary:         string
  /** baseDomains and defaultRefresh are given to the admin console. */
  baseDomains?:    string[]
  defaultRefresh?: string
}

export function readBootstrap(): Bootstrap {
  return JSON.parse(document.getElementById("bootstrap")!.textContent!)
}
