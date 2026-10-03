import { expect, test } from "@playwright/test"

import {
  createDataSource,
  createPanel,
  createSite,
  signIn,
  unique,
  updateSite,
} from "../helpers"

test("takes a status page offline", async({ page }) => {
  await signIn(page)
  const slug = unique("offline")
  const site = await createSite(page, slug)
  await updateSite(page, site.id, {
    legal: {
      imprint: {
        mode: "text",
        text: { en: "Shop Inc." },
      },
      privacy: { mode: "none" },
    },
  })
  await page.goto(`/admin/sites/${site.id}/settings`)
  await page.getByLabel("Availability")
    .selectOption({ label: "Offline: the public page is switched off" })
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByText("Status page saved.")).toBeVisible()
  const entry = page.getByRole("group").filter({ hasText: slug })
  await expect(entry.getByRole("img", { name: "Offline" })).toBeVisible()

  await page.goto("/admin/")
  await expect(page.locator(".card").filter({ hasText: `${slug} en` }))
    .toContainText("Offline")

  const res = await page.goto(`/${slug}/`)
  expect(res?.status()).toBe(503)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${slug} en`,
  })).toBeVisible()
  await expect(page.getByText("This status page is currently unavailable."))
    .toBeVisible()
  await expect(page.getByRole("link", { name: "Incident history" })).toHaveCount(0)
  await expect(page.getByRole("link", { name: "Feed" })).toHaveCount(0)
  await expect(page.getByText("Current service status")).toHaveCount(0)
  await expect(page.getByRole("button", { name: /^Color scheme/ })).toBeVisible()

  await page.getByRole("contentinfo").getByRole("link", { name: "Imprint" }).click()
  await expect(page).toHaveURL(`/${slug}/imprint`)
  await expect(page.getByRole("main")).toContainText("Shop Inc.")
  expect((await page.goto(`/${slug}/imprint`))?.status()).toBe(200)
})

test("pauses a status page", async({ page }) => {
  await signIn(page)
  const ds = await createDataSource(page)
  const slug = unique("paused")
  const site = await createSite(page, slug)
  await createPanel(page, site.id, {
    type:       "stat",
    title:      { en: "Users" },
    datasource: ds.id,
    query:      "42",
  })
  await updateSite(page, site.id, { availability: "paused" })
  await page.goto(`/admin/sites/${site.id}`)
  const entry = page.getByRole("group").filter({ hasText: slug })
  await expect(entry.getByRole("img", { name: "Paused" })).toBeVisible()
  await expect(page.locator("[data-panel-id]").filter({ hasText: "Users" }))
    .toContainText("No data yet")

  const res = await page.goto(`/${slug}/`)
  expect(res?.status()).toBe(503)
  await expect(page.getByText("This status page is currently unavailable."))
    .toBeVisible()
  await expect(page.getByText("Users")).toHaveCount(0)
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("schaltet eine Statusseite ab", async({ page }) => {
    await signIn(page)
    const slug = unique("abgeschaltet")
    const site = await createSite(page, slug, ["de"])
    await page.goto(`/admin/sites/${site.id}/settings`)
    await page.getByLabel("Verfügbarkeit")
      .selectOption({ label: "Pausiert: auch keine Abfragen an Datenquellen" })
    await page.getByRole("button", { name: "Speichern" }).click()
    const entry = page.getByRole("group").filter({ hasText: slug })
    await expect(entry.getByRole("img", { name: "Pausiert" })).toBeVisible()

    await page.goto(`/${slug}/`)
    await expect(page.getByText("Diese Statusseite ist derzeit nicht verfügbar."))
      .toBeVisible()
  })
})
