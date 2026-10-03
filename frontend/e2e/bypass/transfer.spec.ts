import { readFile } from "node:fs/promises"

import { expect, test } from "@playwright/test"

import {
  createDataSource,
  createPanel,
  createSite,
  signIn,
  unique,
} from "../helpers"

test("exports a status page and imports it as a new one", async({ page }) => {
  await signIn(page)
  const ds = await createDataSource(page)
  const slug = unique("export")
  const site = await createSite(page, slug)
  await createPanel(page, site.id, {
    type:       "stat",
    title:      { en: "Users" },
    datasource: ds.id,
    query:      "42",
  })
  await page.goto(`/admin/sites/${site.id}/settings`)
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    page.getByRole("button", { name: "Export" }).click(),
  ])
  expect(download.suggestedFilename()).toBe(`${slug}.yaml`)
  const yaml = await readFile((await download.path())!, "utf8")
  expect(yaml).toMatch(/^version: 1\n/)
  expect(yaml).toContain(`datasource: ${ds.name}`)

  const copy = unique("copy")
  await page.getByLabel("Import status page").setInputFiles({
    name:     "copy.yaml",
    mimeType: "application/yaml",
    buffer:   Buffer.from(yaml.replace(`slug: ${slug}`, `slug: ${copy}`)),
  })
  await expect(page.getByText("Status page imported.")).toBeVisible()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${slug} en`,
  })).toBeVisible()
  await expect(page).not.toHaveURL(`/admin/sites/${site.id}`)

  await page.goto(`/${copy}/`)
  await expect(page.locator("[data-panel-id]").filter({ hasText: "Users" }))
    .toContainText("42")
})

test("replaces a status page from a file", async({ page }) => {
  await signIn(page)
  const slug = unique("replace")
  const site = await createSite(page, slug)
  const yaml = `version: 1\nname: {en: Replaced}\nlanguages: {enabled: [en], primary: en}\nroute: {mode: path, slug: ${slug}}\npanels: []\n`
  await page.goto(`/admin/sites/${site.id}/settings`)
  await page.getByLabel("Import", { exact: true }).setInputFiles({
    name:     "new.yaml",
    mimeType: "application/yaml",
    buffer:   Buffer.from(yaml),
  })
  const dialog = page.getByRole("dialog", { name: "Replace status page?" })
  await expect(dialog).toContainText(`${slug} en`)
  await expect(dialog).toContainText("new.yaml")
  await dialog.getByRole("button", { name: "Replace status page" }).click()
  await expect(page.getByText("Status page imported.")).toBeVisible()
  await expect(page).toHaveURL(`/admin/sites/${site.id}`)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Replaced",
  })).toBeVisible()
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("meldet fehlerhafte Dateien", async({ page }) => {
    await signIn(page)
    await page.getByLabel("Statusseite importieren").setInputFiles({
      name:     "x.yaml",
      mimeType: "application/yaml",
      buffer:   Buffer.from("version: 1\ncolour: red\n"),
    })
    await expect(
      page.getByText("Import fehlgeschlagen: Die Datei ist kein gültiges YAML oder enthält unbekannte Schlüssel."),
    ).toBeVisible()
    await expect(page.getByText("Zeile 2")).toBeVisible()
  })
})
