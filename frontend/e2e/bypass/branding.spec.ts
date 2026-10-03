import { expect, type Page, test } from "@playwright/test"

import {
  createDataSource,
  createPanel,
  createSite,
  signIn,
  unique,
  updateSite,
} from "../helpers"

const logo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 32" onload="alert(1)"><script>alert(1)</script><rect width="64" height="32" fill="#ffcc00"/></svg>`

/** favicon returns the decoded URL of the page's icon. */
async function favicon(page: Page): Promise<string> {
  const href = await page.locator("link[rel=icon]").getAttribute("href")
  return decodeURIComponent(href ?? "")
}

test("brands the header with a color and a logo", async({ page }) => {
  await signIn(page)
  const slug = unique("brand")
  const site = await createSite(page, slug)
  await page.goto(`/admin/sites/${site.id}/settings`)
  await page.getByRole("checkbox", { name: "No brand color" }).uncheck()
  await page.getByLabel("Brand color", { exact: true }).fill("#112233")
  await page.getByLabel("Upload SVG file").setInputFiles({
    name:     "logo.svg",
    mimeType: "image/svg+xml",
    buffer:   Buffer.from(logo),
  })
  const preview = page.locator(".sr-preview")
  await expect(preview).toHaveCSS("background-color", "rgb(17, 34, 51)")
  await expect(preview).toHaveCSS("color", "rgb(255, 255, 255)")
  await expect(preview.locator("img"))
    .toHaveAttribute("src", /^data:image\/svg\+xml,/)
  expect(decodeURIComponent(await preview.locator("img").getAttribute("src") ?? ""))
    .not.toContain("script")
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByText("Status page saved.")).toBeVisible()

  await page.goto(`/${slug}/`)
  const header = page.getByRole("banner")
  await expect(header).toHaveCSS("background-color", "rgb(17, 34, 51)")
  await expect(page.getByRole("heading", { level: 1 }))
    .toHaveCSS("color", "rgb(255, 255, 255)")
  const src = await header.locator("img").getAttribute("src")
  expect(src)
    .toMatch(new RegExp(`^/api/public/sites/${site.id}/logo/[0-9a-f]+\\.svg$`))
  const served = await page.evaluate(async url => (await fetch(url!)).text(), src)
  expect(served).toContain(`fill="#ffcc00"`)
  expect(served).not.toContain("alert")
  await expect(page.getByRole("contentinfo")).toHaveCount(0)

  await page.goto(`/admin/sites/${site.id}/settings`)
  await page.getByRole("checkbox", { name: "No brand color" }).check()
  await page.getByRole("button", { name: "Remove logo" }).click()
  await expect(preview).toHaveCount(0)
})

test("rejects a logo that is not an SVG image", async({ page }) => {
  await signIn(page)
  const site = await createSite(page, unique("nologo"))
  await page.goto(`/admin/sites/${site.id}/settings`)
  await page.getByLabel("Upload SVG file").setInputFiles({
    name:     "logo.svg",
    mimeType: "image/svg+xml",
    buffer:   Buffer.from("<html></html>"),
  })
  await expect(page.getByText("This file is not an SVG image.")).toBeVisible()
})

test("shows the status in the favicon", async({ page, browser, baseURL }) => {
  await signIn(page)
  const ds = await createDataSource(page)
  const fine = unique("fine")
  await createSite(page, fine)
  const down = unique("down")
  const site = await createSite(page, down)
  await createPanel(page, site.id, {
    type:       "status",
    title:      { en: "API" },
    datasource: ds.id,
    query:      "0",
    thresholds: [{
      op:    "<",
      value: 1,
      state: "down",
    }],
  })

  await page.goto(`/${fine}/`)
  await expect(
    page.getByRole("status").filter({ hasText: "All systems operational" }),
  ).toBeVisible()
  expect(await favicon(page)).toContain("#2f9e6b")

  await page.goto(`/${down}/`)
  await expect(
    page.getByRole("status").filter({ hasText: "Major outage" }),
  ).toBeVisible()
  expect(await favicon(page)).toContain("#e5484d")

  // A new browser has no cached payload of the site.
  await updateSite(page, site.id, { availability: "offline" })
  const other = await browser.newPage({ baseURL })
  await other.goto(`/${down}/`)
  await expect(other.getByText("This status page is currently unavailable."))
    .toBeVisible()
  expect(await favicon(other)).toContain("#2f6feb")
  await other.close()
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("setzt die Markenfarbe", async({ page }) => {
    await signIn(page)
    const slug = unique("marke")
    const site = await createSite(page, slug, ["de"])
    await page.goto(`/admin/sites/${site.id}/settings`)
    await page.getByRole("checkbox", { name: "Keine Markenfarbe" }).uncheck()
    await page.getByLabel("Markenfarbe", { exact: true }).fill("#ffcc00")
    await expect(page.getByText("Vorschau des Kopfbereichs")).toBeVisible()
    await page.getByRole("button", { name: "Speichern" }).click()
    await expect(page.getByText("Statusseite gespeichert.")).toBeVisible()

    await page.goto(`/${slug}/`)
    await expect(page.getByRole("banner"))
      .toHaveCSS("background-color", "rgb(255, 204, 0)")
    await expect(page.getByRole("heading", { level: 1 }))
      .toHaveCSS("color", "rgb(0, 0, 0)")
  })
})
