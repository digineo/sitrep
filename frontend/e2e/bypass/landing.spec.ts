import { expect, test } from "@playwright/test"

test("redirects to the visitor's language", async({ page }) => {
  await page.goto("/")
  await expect(page).toHaveURL("/en/")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "SitRep",
  })).toBeVisible()
  await expect(page.getByText("Status pages for your services")).toBeVisible()
  await expect(page.getByRole("link", { name: "Go to administration" }))
    .toHaveAttribute("href", "/admin/")
  await expect(page).toHaveTitle("Status pages")
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("redirects to German", async({ page }) => {
    await page.goto("/")
    await expect(page).toHaveURL("/de/")
    await expect(page.locator("html")).toHaveAttribute("lang", "de")
    await expect(page.getByText("Statusseiten für Ihre Dienste")).toBeVisible()
    await expect(page.getByRole("link", { name: "Zur Verwaltung" })).toBeVisible()
  })
})

test("switches and remembers the language", async({ page }) => {
  await page.goto("/en/")
  await page.getByRole("button", { name: "Language: English" }).click()
  await page.getByRole("menuitemradio", { name: "Deutsch" }).click()
  await expect(page).toHaveURL("/de/")
  await expect(page.getByText("Statusseiten für Ihre Dienste")).toBeVisible()

  await page.goto("/")
  await expect(page).toHaveURL("/de/")
})

test("follows a language link without remembering it", async({ page }) => {
  await page.goto("/de/")
  await page.goto("/")
  await expect(page).toHaveURL("/en/")
})

test("answers unknown paths with a not-found view", async({ page }) => {
  const res = await page.goto("/en/nope")
  expect(res?.status()).toBe(404)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Page not found",
  })).toBeVisible()
  await page.getByRole("link", { name: "Back to the overview" }).click()
  await expect(page).toHaveURL("/en/")
})

test("redirects bare language prefixes", async({ page }) => {
  await page.goto("/de")
  await expect(page).toHaveURL("/de/")
})
