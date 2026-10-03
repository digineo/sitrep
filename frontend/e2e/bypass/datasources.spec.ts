import { expect, test } from "@playwright/test"

import { admin, createSite, signIn, stub, unique } from "../helpers"

test("creates and tests a data source, and a panel uses it", async({ page }) => {
  await signIn(page, "/admin/datasources/new")
  const name = unique("Stub")
  await page.getByLabel("Name", { exact: true }).fill(name)
  await page.getByLabel("URL").fill(stub)
  await page.getByRole("button", { name: "Test connection" }).click()
  await expect(
    page.getByRole("status").filter({ hasText: /^Connection works \(\d+ ms\)$/ }),
  ).toBeVisible()
  await page.getByRole("button", { name: "Save" }).click()

  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Data sources",
  })).toBeVisible()
  const row = page.getByRole("row").filter({ hasText: name })
  await expect(row).toContainText(stub)
  await row.getByRole("button", { name: "Test connection" }).click()
  await expect(row.getByRole("status")).toHaveText(/^Connection works/)

  const slug = unique("ds")
  const site = await createSite(page, slug)
  await page.goto(`/admin/sites/${site.id}/panels/new`)
  await page.getByLabel("Data source").selectOption({ label: name })
  await page.getByLabel("Type").selectOption({ label: "Stat: a single value" })
  await page.getByLabel("Title").fill("Answer")
  await page.getByRole("textbox", { name: "Query" }).fill("42")
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${slug} en`,
  })).toBeVisible()
  await expect(page.locator("[data-panel-id]").filter({ hasText: "Answer" }))
    .toContainText("42")
})

test("keeps secrets out of forms", async({ page }) => {
  await signIn(page, "/admin/datasources/new")
  const name = unique("Secret")
  await page.getByLabel("Name", { exact: true }).fill(name)
  await page.getByLabel("URL").fill(stub)
  await page.getByLabel("Authentication")
    .selectOption({ label: "Username and password" })
  await page.getByLabel("Username").fill("ann")
  await page.getByLabel("Password").fill("hunter2")
  await page.getByRole("button", { name: "Save" }).click()

  await page.getByRole("link", { name }).click()
  await expect(page.getByText("A value is stored.")).toBeVisible()
  await expect(page.getByText("hunter2")).toBeHidden()
  await page.getByRole("button", { name: "Remove" }).click()
  await expect(page.getByText("The value is removed when you save.")).toBeVisible()
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByText("Data source saved.").last()).toBeVisible()
  await expect(page.getByLabel("Password")).toHaveValue("")
})

test("reports failing connections and data sources in use", async({ page }) => {
  await signIn(page)
  const ds = await admin<{ id: string, name: string }>(
    page,
    "POST",
    "/api/admin/datasources",
    {
      name:   unique("Down"),
      type:   "prometheus",
      config: { url: "http://127.0.0.1:1" },
    },
  )
  const slug = unique("used")
  const site = await createSite(page, slug)
  await admin(page, "POST", `/api/admin/sites/${site.id}/panels`, {
    type:       "stat",
    title:      { en: "Up" },
    datasource: ds.id,
    query:      "1",
  })

  await page.goto(`/admin/datasources/${ds.id}`)
  await page.getByRole("button", { name: "Test connection" }).click()
  await expect(page.getByRole("status").filter({ hasText: "Connection failed" }))
    .toContainText("connection refused")

  await page.getByRole("button", { name: "Delete data source" }).click()
  await page.getByRole("dialog")
    .getByRole("button", { name: "Delete data source" })
    .click()
  await expect(page.getByRole("alert"))
    .toContainText("Panels of these status pages still use this data source")
  await expect(page.getByRole("alert").getByRole("link", { name: `${slug} en` }))
    .toHaveAttribute("href", `/admin/sites/${site.id}`)
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("legt eine Datenquelle an", async({ page }) => {
    await signIn(page, "/admin/datasources/new")
    const name = unique("Stub")
    await page.getByLabel("Name", { exact: true }).fill(name)
    await page.getByLabel("URL").fill(stub)
    await page.getByLabel("Zeitlimit").fill("5s")
    await page.getByRole("button", { name: "Verbindung testen" }).click()
    await expect(
      page.getByRole("status").filter({ hasText: /^Verbindung funktioniert/ }),
    ).toBeVisible()
    await page.getByRole("button", { name: "Speichern" }).click()

    await expect(page.getByRole("heading", {
      level: 1,
      name:  "Datenquellen",
    })).toBeVisible()
    await expect(page.getByRole("row").filter({ hasText: name }))
      .toContainText("Prometheus")
  })
})
