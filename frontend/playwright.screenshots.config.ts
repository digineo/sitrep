import { defineConfig } from "@playwright/test"

// Takes the README screenshots (make screenshots) with a testauth build and
// a fake Prometheus API that serves plausible data.
const tmp = "e2e/.tmp"

export default defineConfig({
  testDir:  "e2e/screenshots",
  reporter: "list",
  use:      {
    baseURL:  "http://sitrep.localhost:26080",
    viewport: {
      width:  1280,
      height: 800,
    },
    locale:     "en-US",
    timezoneId: "Europe/Berlin",
  },
  webServer: [
    {
      command:             "node e2e/screenshots/prometheus.ts",
      url:                 "http://127.0.0.1:26092/api/v1/status/flags",
      reuseExistingServer: false,
    },
    {
      command:             `mkdir -p ${tmp} && rm -f ${tmp}/screenshots.db && exec ../sitrep-e2e serve`,
      url:                 "http://127.0.0.1:26080/healthz",
      reuseExistingServer: false,
      env:                 {
        SITREP_LISTEN:          "127.0.0.1:26080",
        SITREP_DB:              `${tmp}/screenshots.db`,
        SITREP_BASE_DOMAINS:    "sitrep.localhost",
        SITREP_AUTH:            "bypass",
        SITREP_LOG_LEVEL:       "warn",
        SITREP_DEFAULT_REFRESH: "5s",
      },
    },
  ],
})
