import { expect, test } from "@playwright/test"

import { signIn } from "../helpers"

// These tests change the instance settings, so they run one after another.
test.describe.configure({ mode: "serial" })

test("saves the instance languages and default color scheme", async({ page }) => {
  await signIn(page, "/admin/settings")
  await expect(page).toHaveTitle("Settings · SitRep admin")
  await page.getByRole("checkbox", { name: "Deutsch" }).uncheck()
  await expect(page.getByRole("checkbox", { name: "English" })).toBeDisabled()
  await page.getByLabel("Default color scheme").selectOption("dark")
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByText("Settings saved.")).toBeVisible()

  await page.goto("/")
  await expect(page).toHaveURL("/")
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark")
  await expect(page.getByRole("button", { name: /^Language/ })).toHaveCount(0)

  await page.goto("/en/")
  await expect(page).toHaveURL("/")
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("orders the languages and picks the primary one", async({ page }) => {
    await signIn(page, "/admin/settings")
    await page.getByRole("checkbox", { name: "Deutsch" }).check()
    await page.getByRole("button", { name: "Deutsch nach oben verschieben" })
      .click()
    await page.getByLabel("Hauptsprache").selectOption("de")
    await page.getByLabel("Standard-Farbschema").selectOption("system")
    await page.getByRole("button", { name: "Speichern" }).click()
    await expect(page.getByText("Einstellungen gespeichert.")).toBeVisible()

    await page.reload()
    await expect(page.getByRole("checkbox")).toHaveCount(2)
    await expect(page.getByRole("main").getByRole("listitem").first())
      .toContainText("Deutsch")
    await expect(page.getByLabel("Hauptsprache")).toHaveValue("de")
  })
})

test("falls back to the primary language", async({ browser }) => {
  const page = await browser.newPage({
    baseURL: "http://sitrep.localhost:26072",
    locale:  "fr-FR",
  })
  await page.goto("/")
  await expect(page).toHaveURL("/de/")
  await expect(page.locator("html")).not.toHaveAttribute("data-theme")
  await page.close()
})
