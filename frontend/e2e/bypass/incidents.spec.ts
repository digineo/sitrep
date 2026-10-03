import { expect, type Page, test } from "@playwright/test"

import {
  admin,
  createDataSource,
  createPanel,
  createSite,
  signIn,
  unique,
} from "../helpers"

/** createIncident opens an incident through the API. */
async function createIncident(
  page: Page,
  site: string,
  title: Record<string, string>,
  update: Record<string, unknown>,
): Promise<{ id: string }> {
  return admin(page, "POST", `/api/admin/sites/${site}/incidents`, {
    title,
    update,
  })
}

/** text fetches a URL from the page and returns the body. */
async function text(page: Page, url: string): Promise<string> {
  return page.evaluate(async url => (await fetch(url)).text(), url)
}

test("opens, updates and resolves an incident", async({ page }) => {
  await signIn(page)
  const ds = await createDataSource(page)
  const slug = unique("inc")
  const site = await createSite(page, slug)
  await createPanel(page, site.id, {
    type:       "timeseries",
    title:      { en: "Load" },
    datasource: ds.id,
    query:      "1",
    range:      "1h",
  })

  await page.goto(`/admin/sites/${site.id}/incidents`)
  await expect(page.getByText("No incidents yet.")).toBeVisible()
  await page.getByRole("link", { name: "New incident" }).click()
  await page.getByLabel("Title").fill("Database outage")
  await page.getByRole("button", { name: "1 hour ago" }).click()
  await expect(page.getByRole("radio", { name: "Active" })).toBeChecked()
  await page.getByText("Major", { exact: true }).click()
  await page.getByRole("textbox", { name: "Description" }).fill("Writes **fail**.")
  await expect(page.getByRole("region", { name: "Preview" }))
    .toContainText("Writes fail.")
  await page.getByRole("button", { name: "Create incident" }).click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Database outage",
  })).toBeVisible()
  await expect(page.locator(".sr-view-head")).toContainText("Severity: Major")

  await page.goto(`/${slug}/`)
  await expect(
    page.getByRole("status").filter({ hasText: "Some systems are degraded" }),
  ).toBeVisible()
  const current = page.locator("section")
    .filter({ has: page.getByRole("heading", { name: "Current incidents" }) })
  await expect(current).toContainText("Database outage")
  await expect(current).toContainText("Writes fail.")

  // The incident's band covers the chart; hovering it names the incident,
  // a click opens it.
  const chart = page.locator("[data-panel-id]")
    .filter({ hasText: "Load" })
    .locator(".u-over")
  await expect(chart).toBeVisible()
  await chart.hover({
    position: {
      x: 200,
      y: 50,
    },
  })
  await expect(page.locator(".sr-tooltip"))
    .toContainText("Incident: Database outage")
  await chart.click({
    position: {
      x: 200,
      y: 50,
    },
  })
  await expect(page.getByRole("heading", {
    level: 2,
    name:  "Database outage",
  })).toBeVisible()
  await expect(page).toHaveTitle(`Database outage · ${slug} en`)

  await page.goto(`/admin/sites/${site.id}/incidents`)
  await page.getByRole("link", { name: /Database outage/ }).click()
  await page.getByText("Resolved", { exact: true }).click()
  await page.getByRole("textbox", { name: "Description" }).fill("Fixed.")
  await page.getByRole("button", { name: "Post update" }).click()
  await expect(page.getByText("Update posted.")).toBeVisible()
  await expect(page.locator(".sr-timeline > li")).toHaveCount(2)

  // The payload the page loaded before may come from the HTTP cache for
  // 10 seconds; the next refresh brings the resolved incident.
  await page.goto(`/${slug}/`)
  await expect(
    page.getByRole("status").filter({ hasText: "All systems operational" }),
  ).toBeVisible({ timeout: 20_000 })
  const recent = page.locator("section")
    .filter({ has: page.getByRole("heading", { name: "Recent incidents" }) })
  await expect(recent).toContainText("Fixed.")

  const feed = await text(page, `/${slug}/feed.atom`)
  expect(feed).toContain("<title>Database outage: Resolved</title>")
  expect(feed).toContain("<title>Database outage: Active</title>")
  expect(feed).toContain("Previous updates")
})

test("follows the rules of the timeline", async({ page }) => {
  await signIn(page)
  const site = await createSite(page, unique("rules"))
  const incident = await createIncident(page, site.id, { en: "Slow API" }, {
    status:      "active",
    description: { en: "Slow." },
  })
  await page.goto(`/admin/sites/${site.id}/incidents/${incident.id}`)

  await page.getByRole("button", { name: "Post update" }).click()
  await expect(page.locator(".sr-timeline > li")).toHaveCount(1)
  const unchanged = page.getByRole("radio", { name: "Unchanged" }).first()
  expect(await unchanged.evaluate((el: HTMLInputElement) => el.validationMessage))
    .toBe("Choose a new status, a new severity or both.")

  await page.getByText("Critical", { exact: true }).click()
  await page.getByRole("button", { name: "Post update" }).click()
  await expect(page.locator(".sr-timeline > li")).toHaveCount(2)
  const opening = page.locator(".sr-timeline > li").last()
  await expect(opening.getByRole("button", { name: /^Delete update/ }))
    .toBeDisabled()
  await expect(opening).toContainText("The opening update can only be deleted")

  await opening.getByRole("button", { name: /^Edit update/ }).click()
  await expect(page.getByRole("button", { name: "Add update" })).toBeVisible()
  await page.getByText("Planned", { exact: true }).click()
  await page.getByRole("button", {
    name:  "Save",
    exact: true,
  }).click()
  await expect(page.getByText("Update saved.")).toBeVisible()
  await expect(page.locator(".sr-timeline > li").last()).toContainText("Planned")

  const latest = page.locator(".sr-timeline > li").first()
  await latest.getByRole("button", { name: /^Delete update/ }).click()
  await page.getByRole("dialog")
    .getByRole("button", { name: "Delete update" })
    .click()
  await expect(page.locator(".sr-timeline > li")).toHaveCount(1)

  await page.locator(".sr-timeline > li")
    .getByRole("button", { name: /^Delete update/ })
    .click()
  await expect(page.getByRole("dialog")).toContainText("This is the only update.")
  await page.getByRole("dialog")
    .getByRole("button", { name: "Delete incident" })
    .click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  /^Incidents of/,
  })).toBeVisible()
  await expect(page.getByText("No incidents yet.")).toBeVisible()
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("zeigt Vorfälle, ihre Details und das Archiv", async({ page }) => {
    await signIn(page)
    const slug = unique("vorfall")
    const site = await createSite(page, slug, ["de", "en"])
    await page.goto(`/admin/sites/${site.id}/incidents/new`)
    await page.getByRole("textbox", { name: /^Titel/ }).fill("Wartung")
    await page.getByText("Geplant", { exact: true }).click()
    await page.getByRole("textbox", { name: "Beschreibung" })
      .fill("Samstag von 10 bis 12 Uhr.")
    await page.getByRole("button", { name: "Vorfall anlegen" }).click()
    await expect(page.getByRole("heading", {
      level: 1,
      name:  "Wartung",
    })).toBeVisible()

    await page.goto(`/${slug}/`)
    await expect(page).toHaveURL(new RegExp(`/${slug}/de/$`))
    const upcoming = page.locator("section")
      .filter({ has: page.getByRole("heading", { name: "Geplante Wartungen" }) })
    await expect(upcoming).toContainText("Samstag von 10 bis 12 Uhr.")
    await upcoming.getByRole("link", { name: /Wartung/ }).click()
    await expect(page.getByRole("heading", {
      level: 2,
      name:  "Wartung",
    })).toBeFocused()
    await expect(page.locator(".sr-timeline")).toContainText("Geplant")

    await page.getByRole("link", { name: "Übersicht" }).click()
    await page.getByRole("link", { name: "Vorfallsverlauf" }).click()
    await expect(page).toHaveURL(new RegExp(`/${slug}/de/incidents$`))
    await expect(page.getByRole("heading", {
      level: 2,
      name:  "Alle Vorfälle",
    })).toBeVisible()
    await expect(page.getByRole("link", { name: /Wartung/ })).toBeVisible()

    const feed = await text(page, `/${slug}/de/feed.atom`)
    expect(feed).toContain(`xml:lang="de"`)
    expect(feed).toContain("<title>Wartung: Geplant</title>")
  })
})

test("pages through the archive", async({ page }) => {
  await signIn(page)
  const slug = unique("archive")
  const site = await createSite(page, slug)
  for (let i = 0; i < 21; i++) {
    await createIncident(page, site.id, { en: `Incident ${i}` }, {
      status:      "planned",
      description: { en: "x" },
    })
  }

  await page.goto(`/${slug}/incidents`)
  await expect(page.getByRole("navigation", { name: "Pages" }))
    .toContainText("Page 1 of 2")
  await expect(page.getByRole("link", { name: /^Incident \d/ })).toHaveCount(20)
  await expect(page.getByRole("link", { name: "Newer" })).toHaveCount(0)

  await page.getByRole("link", { name: "Older" }).click()
  await expect(page).toHaveURL(new RegExp(`/${slug}/incidents\\?page=2$`))
  await expect(page.getByRole("link", { name: /^Incident 0/ })).toBeVisible()
  await expect(page.getByRole("link", { name: "Older" })).toHaveCount(0)

  const res = await page.goto(`/${slug}/incidents?page=3`)
  expect(res?.status()).toBe(404)
  await expect(page.getByRole("heading", { name: "Page not found" })).toBeVisible()
})

test("lets allowed origins read incidents.json", async({ page }) => {
  await signIn(page)
  const slug = unique("cors")
  const site = await createSite(page, slug)
  await createIncident(page, site.id, { en: "Outage" }, {
    status:      "active",
    description: { en: "Down" },
  })
  await page.goto(`/admin/sites/${site.id}/settings`)
  await page.getByRole("button", { name: "Add origin" }).click()
  await page.getByRole("textbox", { name: "Origin 1" })
    .fill("http://elsewhere.localhost:26071/path")
  await expect(
    page.getByText("Enter an origin like https://www.example.com, without a path."),
  ).toBeVisible()
  await page.getByRole("textbox", { name: "Origin 1" })
    .fill("http://elsewhere.localhost:26071")
  await page.getByRole("button", { name: "Add origin" }).click()
  await page.getByLabel("Keep finished incidents (days)").fill("30")
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByText("Status page saved.")).toBeVisible()

  const url = `http://sitrep.localhost:26071/${slug}/incidents.json`
  await page.goto("http://elsewhere.localhost:26071/healthz")
  const incidents = await page.evaluate(async url => (await fetch(url)).json(), url)
  expect(incidents).toMatchObject([{
    title:  "Outage",
    phase:  "ongoing",
    status: "active",
  }])

  await page.goto("http://other.localhost:26071/healthz")
  await expect(page.evaluate(async url => (await fetch(url)).json(), url))
    .rejects.toThrow()
})
