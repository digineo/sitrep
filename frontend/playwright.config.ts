import { defineConfig } from "@playwright/test"

// The servers run a testauth build (make test-e2e builds it). Each project
// gets its own server and database: "settings" and "landing" change the
// instance settings, which the other specs rely on. Data sources point at a
// stub of the Prometheus API.
const binary = "../sitrep-e2e"
const tmp = "e2e/.tmp"

function server(
  name: string,
  port: number,
  env: Record<string, string>,
  prepare = "",
) {
  return {
    command: `mkdir -p ${tmp} && rm -f ${tmp}/${name}.db && `
      + `${prepare} exec ${binary} serve`,
    url:                 `http://127.0.0.1:${port}/healthz`,
    reuseExistingServer: false,
    env:                 {
      SITREP_LISTEN:          `127.0.0.1:${port}`,
      SITREP_DB:              `${tmp}/${name}.db`,
      SITREP_BASE_DOMAINS:    "sitrep.localhost",
      SITREP_LOG_LEVEL:       "warn",
      SITREP_SECRET_KEY:      "ZTJlLXRlc3RzLW9ubHktMzItYnl0ZXMtc2VjcmV0LWs=",
      SITREP_DEFAULT_REFRESH: "5s",
      ...env,
    },
  }
}

const users = ["admin", "admin-de", "throttled", "throttled-de"]
  .map(user => "printf 'correct horse\\n' | "
    + `${binary} hash-password -user ${user} >> ${tmp}/users &&`)
  .join(" ")

const basicEnv = {
  SITREP_AUTH:             "basic",
  SITREP_BASIC_USERS_FILE: `${tmp}/users`,
  SITREP_TRUST_PROXY:      "true",
}

export default defineConfig({
  testDir:       "e2e",
  fullyParallel: true,
  forbidOnly:    !!process.env.CI,
  reporter:      process.env.CI ? "github" : "list",
  use:           { trace: "retain-on-failure" },
  projects:      [
    {
      name:      "bypass",
      testMatch: "bypass/*.spec.ts",
      use:       { baseURL: "http://sitrep.localhost:26071" },
    },
    {
      name:      "settings",
      testMatch: "settings/*.spec.ts",
      use:       { baseURL: "http://sitrep.localhost:26072" },
    },
    {
      name:      "basic",
      testMatch: "basic/*.spec.ts",
      use:       { baseURL: "http://sitrep.localhost:26073" },
    },
    {
      name:      "landing",
      testMatch: "landing/*.spec.ts",
      use:       { baseURL: "http://sitrep.localhost:26074" },
    },
  ],
  webServer: [
    {
      command:             "node e2e/prometheus.ts",
      url:                 "http://127.0.0.1:26090/api/v1/status/flags",
      reuseExistingServer: false,
    },
    server("bypass", 26071, { SITREP_AUTH: "bypass" }),
    server("settings", 26072, { SITREP_AUTH: "bypass" }),
    server("landing", 26074, { SITREP_AUTH: "bypass" }),
    // Behind a trusted proxy, each test signs in from its own X-Forwarded-For
    // address, so the per-address throttle does not couple the tests.
    server("basic", 26073, basicEnv, `rm -f ${tmp}/users && ${users}`),
  ],
})
