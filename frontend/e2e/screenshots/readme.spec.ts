import { expect, type Page, test } from "@playwright/test"

import { admin, createPanel, signIn } from "../helpers"

const dir = "../docs/screenshots"
const hour = 3600 * 1000

/** ago returns the time hours ago. */
const ago = (hours: number) => new Date(Date.now() - hours * hour).toISOString()

/** tomorrow returns the next day's hour in UTC. */
const tomorrow = (h: number) =>
  new Date((Math.floor(Date.now() / (24 * hour)) + 1) * 24 * hour + h * hour).toISOString()

const logo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" `
  + `stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`
  + `<path d="M17.5 19H9a7 7 0 1 1 6.71-9h1.79a4.5 4.5 0 1 1 0 9Z"/></svg>`

const probe = [{ op: "<", value: 1, state: "down" }]

/** incident opens an incident with the first update and adds the others. */
async function incident(
  page: Page,
  site: string,
  title: string,
  ...updates: Record<string, unknown>[]
): Promise<string> {
  const [first, ...rest] = updates.map(u => ({ ...u, description: { en: u.description } }))
  const { id } = await admin<{ id: string }>(page, "POST", `/api/admin/sites/${site}/incidents`, {
    title:  { en: title },
    update: first,
  })
  for (const u of rest) {
    await admin(page, "POST", `/api/admin/sites/${site}/incidents/${id}/updates`, u)
  }
  return id
}

test("README screenshots", async({ page }) => {
  await signIn(page)

  const ds = await admin<{ id: string }>(page, "POST", "/api/admin/datasources", {
    name:   "Prometheus",
    type:   "prometheus",
    config: { url: "http://127.0.0.1:26092" },
  })
  const { id: site } = await admin<{ id: string }>(page, "POST", "/api/admin/sites", {
    name:       { en: "Acme Cloud" },
    languages:  { enabled: ["en"], primary: "en" },
    timezone:   "Europe/Berlin",
    route:      { mode: "path", slug: "acme" },
    brandColor: "#1d3557",
    logo,
  })

  const panels = [
    { type: "status", title: "Website", query: 'probe_success{job="website"}', thresholds: probe },
    { type: "status", title: "API", query: 'probe_success{job="api"}', thresholds: probe },
    { type: "status", title: "Customer dashboard", query: 'probe_success{job="dashboard"}', thresholds: probe },
    { type: "status", title: "Payments", query: 'probe_success{job="payments"}', thresholds: probe },
    {
      type:       "status",
      title:      "Email delivery",
      query:      'sum(mail_queue_size{queue="deferred"})',
      thresholds: [{ op: ">", value: 2000, state: "down" }, { op: ">", value: 500, state: "degraded" }],
    },
    {
      type:     "stat",
      title:    "Uptime, last 30 days",
      query:    'avg_over_time(probe_success{job="api"}[30d]) * 100',
      decimals: 2,
      unit:     "%",
    },
    {
      type:  "stat",
      title: "Response time (p95)",
      query: "histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[5m]))) * 1000",
      unit:  "ms",
    },
    { type: "stat", title: "Requests", query: "sum(rate(http_requests_total[5m]))", unit: "req/s" },
    {
      type:     "timeseries",
      title:    "Response time (p95)",
      query:    "histogram_quantile(0.95, sum by (le, region) (rate(http_request_duration_seconds_bucket[5m]))) * 1000",
      range:    "24h",
      decimals: 0,
      unit:     "ms",
      legend:   "{{region}}",
    },
    {
      type:     "timeseries",
      title:    "Requests",
      query:    "sum(rate(http_requests_total[5m]))",
      range:    "24h",
      style:    "area",
      minZero:  true,
      decimals: 0,
      unit:     "req/s",
    },
    {
      type:    "timeseries",
      title:   "Error rate",
      query:   'sum(rate(http_requests_total{code=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100',
      range:   "24h",
      minZero: true,
      unit:    "%",
    },
    {
      type:     "timeseries",
      title:    "Deferred emails",
      query:    'sum(mail_queue_size{queue="deferred"})',
      range:    "24h",
      style:    "area",
      minZero:  true,
      decimals: 0,
    },
  ]
  const ids: string[] = []
  for (const p of panels) {
    const { id } = await createPanel(page, site, {
      ...p,
      title:      { en: p.title },
      unit:       p.unit && { en: p.unit },
      legend:     p.legend && { en: p.legend },
      datasource: ds.id,
    })
    ids.push(id)
  }

  // The anomalies of the fake Prometheus data match these times.
  await incident(
    page,
    site,
    "Elevated API error rates",
    {
      at:          ago(14),
      status:      "active",
      severity:    "major",
      description: "Some API requests fail with HTTP 500 errors.",
    },
    {
      at:          ago(13.75),
      status:      "investigating",
      description: "A database node failed. We are promoting a replica.",
    },
    {
      at:          ago(13.25),
      status:      "resolved",
      description: "The replica took over and error rates are back to normal.",
    },
  )
  await incident(
    page,
    site,
    "Database maintenance",
    {
      at:          tomorrow(20),
      status:      "planned",
      severity:    "minor",
      description: "We upgrade the database cluster. The API may respond slowly for up to **15 minutes**.",
    },
  )
  const mail = await incident(
    page,
    site,
    "Delayed email delivery",
    {
      at:          ago(2),
      status:      "active",
      severity:    "minor",
      description: "Some outgoing emails are delayed, including invoices and password resets.",
    },
    {
      at:          ago(1.5),
      status:      "investigating",
      description: "Our mail provider throttles our servers. We are routing mail "
        + "through a second provider. **No emails are lost.**",
    },
  )

  // The console's sidebar is as high as the viewport, so the viewport grows
  // to the page instead of taking a full-page screenshot.
  const shot = async(name: string) => {
    const height = await page.evaluate(() => document.documentElement.scrollHeight)
    await page.setViewportSize({ width: 1280, height })
    await page.screenshot({
      path:       `${dir}/${name}.png`,
      animations: "disabled",
      caret:      "hide",
    })
    await page.setViewportSize({ width: 1280, height: 800 })
  }

  await page.goto("/acme/")
  await expect(page.locator("[data-panel-id]")).toHaveCount(panels.length)
  await expect(page.locator(".u-over")).toHaveCount(4)
  await expect(page.getByText("No data yet")).toHaveCount(0)
  await shot("status-page")
  await page.emulateMedia({ colorScheme: "dark" })
  await shot("status-page-dark")
  await page.emulateMedia({ colorScheme: "light" })

  await page.goto(`/admin/sites/${site}/incidents/${mail}`)
  await expect(page.getByRole("heading", { level: 1, name: "Delayed email delivery" })).toBeVisible()
  await page.getByText("Monitoring", { exact: true }).click()
  await page.getByRole("textbox", { name: "Description" }).fill(
    "The second provider delivers the delayed emails. We **monitor** the queue until it is empty.",
  )
  await expect(page.getByRole("region", { name: "Preview" })).toContainText("We monitor the queue")
  await shot("console-incident")

  const chart = ids[panels.findIndex(p => p.type === "timeseries")]
  await page.goto(`/admin/sites/${site}/panels/${chart}`)
  await page.getByRole("button", { name: "Widget preview" }).click()
  await expect(page.locator(".u-over")).toBeVisible()
  await shot("console-panel")
})
