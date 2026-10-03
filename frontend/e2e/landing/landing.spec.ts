import { expect, test } from "@playwright/test"

import { createSite, signIn, stub, unique } from "../helpers"

// These tests change the instance settings, so they run one after another.
test.describe.configure({ mode: "serial" })

test("sets the landing text and the global legal pages", async({ page }) => {
  await signIn(page, "/admin/settings")
  await page.getByLabel("Imprint", { exact: true }).selectOption({ label: "Text" })
  const imprintLabel = page.getByText("Imprint: text", { exact: true })
  const imprint = page.locator(".field", { has: imprintLabel })
  await imprint.getByRole("textbox").fill("**Acme Inc.**, Main Street 1")
  await imprint.getByRole("tab", { name: /Deutsch/ }).click()
  await imprint.getByRole("textbox").fill("**Acme GmbH**, Hauptstraße 1")
  await page.getByLabel("Privacy statement", { exact: true })
    .selectOption({ label: "Link to a page elsewhere" })
  await page.getByRole("textbox", { name: "Privacy statement: link" })
    .fill(`${stub}/api/v1/status/flags`)
  const landingLabel = page.getByText("Landing page text", { exact: true })
  const landing = page.locator(".field", { has: landingLabel })
  await landing.getByRole("textbox")
    .fill("# Welcome\n\nThe status of all Acme services.")
  await landing.getByRole("tab", { name: /Deutsch/ }).click()
  await landing.getByRole("textbox")
    .fill("# Willkommen\n\nDer Status aller Dienste von Acme.")
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByText("Settings saved.")).toBeVisible()

  await page.goto("/en/")
  await expect(page).toHaveTitle("Status pages")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Welcome",
  })).toBeVisible()
  await expect(page.getByText("The status of all Acme services.")).toBeVisible()
  await expect(page.getByRole("link", { name: "Go to administration" }))
    .toHaveCount(0)

  const footer = page.getByRole("contentinfo")
  await expect(footer.getByRole("link", { name: "Privacy statement" }))
    .toHaveAttribute("href", `${stub}/api/v1/status/flags`)
  await footer.getByRole("link", { name: "Imprint" }).click()
  await expect(page).toHaveURL("/en/imprint")
  await expect(page).toHaveTitle("Imprint · Status pages")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Imprint",
  })).toBeVisible()
  await expect(page.getByRole("main")).toContainText("Acme Inc., Main Street 1")
  await page.getByRole("link", { name: "Start page" }).click()
  await expect(page).toHaveURL("/en/")
})

test("sites inherit the global legal pages", async({ page }) => {
  await signIn(page)
  const slug = unique("inherit")
  await createSite(page, slug, ["de"])
  await page.goto(`/${slug}/`)
  await page.getByRole("contentinfo")
    .getByRole("link", { name: "Impressum" })
    .click()
  await expect(page).toHaveURL(`/${slug}/impressum`)
  await expect(page.getByRole("main")).toContainText("Acme GmbH, Hauptstraße 1")
})

test("links the legal pages on the login screen", async({ page }) => {
  await page.goto("/admin/")
  await expect(page.getByRole("link", { name: "Sign in with single sign-on" }))
    .toBeVisible()
  await page.getByRole("contentinfo").getByRole("link", { name: "Imprint" }).click()
  await expect(page).toHaveURL("/en/imprint")
  await expect(page.getByRole("main")).toContainText("Acme Inc.")
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("zeigt den Text der Startseite und das Impressum", async({ page }) => {
    await page.goto("/")
    await expect(page).toHaveURL("/de/")
    await expect(page.getByRole("heading", {
      level: 1,
      name:  "Willkommen",
    })).toBeVisible()

    await page.getByRole("contentinfo")
      .getByRole("link", { name: "Impressum" })
      .click()
    await expect(page).toHaveURL("/de/impressum")
    await expect(page).toHaveTitle("Impressum · Statusseiten")
    await expect(page.getByRole("main")).toContainText("Acme GmbH, Hauptstraße 1")

    await page.getByRole("button", { name: "Sprache: Deutsch" }).click()
    await page.getByRole("menuitemradio", { name: "English" }).click()
    await expect(page).toHaveURL("/en/imprint")
    await expect(page.getByRole("main")).toContainText("Acme Inc.")
  })
})
