import { expect, test } from "@playwright/test"

import { createSite, signIn, stub, unique, updateSite } from "../helpers"

test("sets the legal pages of a status page", async({ page }) => {
  await signIn(page)
  const slug = unique("legal")
  const site = await createSite(page, slug, ["en", "de"])
  await page.goto(`/admin/sites/${site.id}/settings`)
  await page.getByLabel("Imprint", { exact: true }).selectOption({ label: "Text" })
  const imprintLabel = page.getByText("Imprint: text", { exact: true })
  const imprint = page.locator(".field", { has: imprintLabel })
  await imprint.getByRole("textbox").fill("**Shop Inc.**, Main Street 1")
  await imprint.getByRole("tab", { name: /Deutsch/ }).click()
  await imprint.getByRole("textbox").fill("**Laden GmbH**, Hauptstraße 1")
  await page.getByLabel("Privacy statement", { exact: true })
    .selectOption({ label: "Link to a page elsewhere" })
  await page.getByRole("textbox", { name: "Privacy statement: link" })
    .fill(`${stub}/api/v1/status/flags`)
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByText("Status page saved.")).toBeVisible()

  await page.goto(`/${slug}/en/`)
  const footer = page.getByRole("contentinfo")
  await expect(footer).toHaveText("Privacy statement · Imprint")
  await expect(footer.getByRole("link", { name: "Privacy statement" }))
    .toHaveAttribute("href", `${stub}/api/v1/status/flags`)
  await footer.getByRole("link", { name: "Imprint" }).click()
  await expect(page).toHaveURL(`/${slug}/en/imprint`)
  await expect(page).toHaveTitle(`Imprint · ${slug} en`)
  await expect(page.getByRole("heading", {
    level: 2,
    name:  "Imprint",
  })).toBeFocused()
  await expect(page.getByRole("main")).toContainText("Shop Inc., Main Street 1")

  await page.getByRole("button", { name: "Language: English" }).click()
  await page.getByRole("menuitemradio", { name: "Deutsch" }).click()
  await expect(page).toHaveURL(`/${slug}/de/impressum`)
  await expect(page.getByRole("heading", {
    level: 2,
    name:  "Impressum",
  })).toBeVisible()
  await expect(page.getByRole("main")).toContainText("Laden GmbH, Hauptstraße 1")
  await expect(page.getByRole("main")).not.toContainText("Shop Inc.")

  await page.goto(`/${slug}/imprint`)
  await expect(page).toHaveURL(`/${slug}/en/imprint`)

  await page.goto(`/${slug}/de/datenschutz`)
  await expect(page).toHaveURL(`${stub}/api/v1/status/flags`)
})

test("hides legal pages in mode none", async({ page }) => {
  await signIn(page)
  const slug = unique("nolegal")
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
  await page.goto(`/${slug}/`)
  await expect(page.getByRole("contentinfo")).toHaveText("Imprint")

  const res = await page.goto(`/${slug}/privacy`)
  expect(res?.status()).toBe(404)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Page not found",
  })).toBeVisible()

  const none = unique("none")
  const other = await createSite(page, none)
  await updateSite(page, other.id, {
    legal: {
      imprint: { mode: "none" },
      privacy: { mode: "none" },
    },
  })
  await page.goto(`/${none}/`)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${none} en`,
  })).toBeVisible()
  await expect(page.getByRole("contentinfo")).toHaveCount(0)
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("legt die rechtlichen Seiten einer Statusseite fest", async({ page }) => {
    await signIn(page)
    const slug = unique("recht")
    const site = await createSite(page, slug, ["de"])
    await page.goto(`/admin/sites/${site.id}/settings`)
    await page.getByLabel("Datenschutzerklärung", { exact: true })
      .selectOption({ label: "Text" })
    await page.getByRole("textbox", { name: "Datenschutzerklärung: Text" })
      .fill("Wir speichern *nichts*.")
    await page.getByRole("button", { name: "Speichern" }).click()
    await expect(page.getByText("Statusseite gespeichert.")).toBeVisible()

    await page.goto(`/${slug}/`)
    await page.getByRole("contentinfo")
      .getByRole("link", { name: "Datenschutzerklärung" })
      .click()
    await expect(page).toHaveURL(`/${slug}/datenschutz`)
    await expect(page.getByRole("main")).toContainText("Wir speichern nichts.")
    await page.getByRole("link", { name: "Übersicht" }).click()
    await expect(page).toHaveURL(`/${slug}/`)
  })
})
