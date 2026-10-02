import { defineConfig } from "@playwright/test"

// The servers run a testauth build (make test-e2e builds it). Each project
// gets its own server and database: "settings" changes the instance
// settings, which the other specs rely on.
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
      SITREP_LISTEN:       `127.0.0.1:${port}`,
      SITREP_DB:           `${tmp}/${name}.db`,
      SITREP_BASE_DOMAINS: "sitrep.localhost",
      SITREP_LOG_LEVEL:    "warn",
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
  ],
  webServer: [
    server("bypass", 26071, { SITREP_AUTH: "bypass" }),
    server("settings", 26072, { SITREP_AUTH: "bypass" }),
    server("basic", 26073, basicEnv, `rm -f ${tmp}/users && ${users}`),
  ],
})
