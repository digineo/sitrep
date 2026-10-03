import { expect, type Page, test } from "@playwright/test"

import {
  admin,
  createDataSource,
  createPanel,
  createSite,
  signIn,
  unique,
} from "../helpers"

/** addPanel fills the panel editor of the open preview. */
async function addPanel(
  page: Page,
  dataSource: string,
  type: string,
  title: string,
  query: string,
) {
  await page.getByRole("link", { name: "Add panel" }).click()
  await page.getByLabel("Data source").selectOption({ label: dataSource })
  await page.getByLabel("Type").selectOption({ label: type })
  await page.getByLabel("Title").fill(title)
  await page.getByRole("textbox", { name: "Query" }).fill(query)
}

test("creates a status page with panels and shows it", async({ page }) => {
  await signIn(page)
  const ds = await createDataSource(page)
  const slug = unique("shop")
  await page.goto("/admin/sites/new")
  await page.getByRole("checkbox", { name: "Deutsch" }).uncheck()
  await page.getByLabel("Name (English)").fill("Shop")
  await page.getByLabel("Address").fill(slug)
  await expect(
    page.getByText(`The status page will be reachable at http://sitrep.localhost:26071/${slug}/`),
  ).toBeVisible()
  await page.getByRole("button", { name: "Create status page" }).click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Shop",
  })).toBeVisible()

  await addPanel(page, ds.name, "Status: up or down by thresholds", "API", "1")
  await page.getByRole("button", { name: "Add rule" }).click()
  await page.getByRole("button", { name: "Save" }).click()
  await expect(page.getByRole("row", { name: /API/ })).toContainText("Operational")

  await addPanel(page, ds.name, "Stat: a single value", "Latency", "1234.5")
  await page.getByLabel("Decimals").fill("1")
  await page.getByLabel("Unit").fill("ms")
  await page.getByRole("button", { name: "Widget preview" }).click()
  await expect(page.getByRole("region", { name: "Widget preview" }))
    .toContainText("1,234.5 ms")
  await page.getByRole("button", { name: "Save" }).click()

  await addPanel(page, ds.name, "Chart: values over time", "Load", "up")
  await page.getByLabel("Time range").fill("1h")
  await page.getByRole("button", { name: "Data preview" }).click()
  await expect(page.getByRole("region", { name: "Data preview" }))
    .toContainText("series")
  await page.getByRole("button", { name: "Save" }).click()
  await expect(
    page.locator("[data-panel-id]").filter({ hasText: "Load" }).locator("canvas"),
  ).toBeVisible()

  await page.goto(`/${slug}/`)
  await expect(page).toHaveTitle("Shop")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Shop",
  })).toBeVisible()
  await expect(
    page.getByRole("status").filter({ hasText: "All systems operational" }),
  ).toBeVisible()
  await expect(page.getByRole("row", { name: /API/ })).toContainText("Operational")
  await expect(page.locator("[data-panel-id]").filter({ hasText: "Latency" }))
    .toContainText("1,234.5 ms")
  await expect(page.getByRole("img", { name: "Load, last 1 hour: Load 1.00" }))
    .toBeVisible()
  await expect(page.getByText(/^Updated \d/)).toBeVisible()
})

test("orders panels by keyboard and by dragging", async({ page }) => {
  await signIn(page)
  const ds = await createDataSource(page)
  const site = await createSite(page, unique("order"))
  for (const title of ["A", "B", "C"]) {
    await createPanel(page, site.id, {
      type:       "stat",
      title:      { en: title },
      datasource: ds.id,
      query:      "1",
    })
  }

  await page.goto(`/admin/sites/${site.id}`)
  const titles = page.locator(".sr-stat h3")
  await expect(titles).toHaveText(["A", "B", "C"])

  await page.getByRole("button", { name: "Move panel B" }).focus()
  await page.keyboard.press("ArrowUp")
  await expect(titles).toHaveText(["B", "A", "C"])
  await expect(page.getByRole("button", { name: "Move panel B" })).toBeFocused()
  await expect(page.getByText("B moved to position 1 of 3")).toBeAttached()

  const grip = page.getByRole("button", { name: "Move panel C" })
  const target = page.locator("[data-panel-id]").filter({ hasText: "B" }).first()
  await grip.hover()
  await page.mouse.down()
  const box = (await target.boundingBox())!
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 5 })
  await page.mouse.up()
  await expect(titles).toHaveText(["C", "B", "A"])

  await page.reload()
  await expect(titles).toHaveText(["C", "B", "A"])
})

test("redirects to the language of multi-language sites", async({ page }) => {
  await signIn(page)
  const multi = unique("multi")
  const single = unique("single")
  await createSite(page, multi, ["en", "de"])
  await createSite(page, single, ["de"])

  await page.goto(`/${multi}`)
  await expect(page).toHaveURL(`/${multi}/en/`)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${multi} en`,
  })).toBeVisible()
  await page.getByRole("button", { name: "Language: English" }).click()
  await page.getByRole("menuitemradio", { name: "Deutsch" }).click()
  await expect(page).toHaveURL(`/${multi}/de/`)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${multi} de`,
  })).toBeVisible()
  await expect(page.getByText("Aktueller Dienststatus")).toBeVisible()

  await page.goto(`/${multi}/`)
  await expect(page).toHaveURL(`/${multi}/de/`)

  await page.goto(`/${single}/en/`)
  await expect(page).toHaveURL(`/${single}/`)
  await expect(
    page.getByRole("status").filter({ hasText: "Alle Systeme betriebsbereit" }),
  ).toBeVisible()
})

test("serves subdomain sites", async({ page }) => {
  await signIn(page)
  const slug = unique("sub")
  await createSite(page, slug, ["en"], "subdomain")
  await page.goto(`http://${slug}.sitrep.localhost:26071/`)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${slug} en`,
  })).toBeVisible()
  await expect(
    page.getByRole("status").filter({ hasText: "All systems operational" }),
  ).toBeVisible()

  // Reverse proxies with on-demand TLS ask before requesting a certificate.
  const known = await page.goto(`/tls/authorize?domain=${slug}.sitrep.localhost`)
  expect(known?.status()).toBe(200)
  const unknown = await page.goto(
    `/tls/authorize?domain=${unique("sub")}.sitrep.localhost`,
  )
  expect(unknown?.status()).toBe(404)
})

test("shows outages and unusable data", async({ page }) => {
  await signIn(page)
  const ds = await createDataSource(page)
  const slug = unique("down")
  const site = await createSite(page, slug)
  await createPanel(page, site.id, {
    type:       "status",
    title:      { en: "Checkout" },
    datasource: ds.id,
    query:      "0",
    thresholds: [{
      op:    "<",
      value: 1,
      state: "down",
    }],
  })
  await page.goto(`/admin/sites/${site.id}`)
  await expect(
    page.getByRole("status").filter({ hasText: "Major outage" }),
  ).toBeVisible()
  await expect(page.getByRole("row", { name: /Checkout/ })).toContainText("Down")

  // Public responses are cached for 10 seconds, so the public page is
  // opened only after the data source broke.
  await admin(page, "PUT", `/api/admin/datasources/${ds.id}`, {
    name:   ds.name,
    config: { url: "http://127.0.0.1:1" },
  })
  await page.goto(`/${slug}/`)
  await expect(
    page.getByRole("status").filter({ hasText: "Status not fully available" }),
  ).toBeVisible()
  await expect(page.getByRole("row", { name: /Checkout/ }))
    .toContainText("No data yet")
})

test("remembers the color scheme on public pages", async({ page }) => {
  await signIn(page)
  const slug = unique("theme")
  await createSite(page, slug)
  await page.goto(`/${slug}/`)
  await page.getByRole("button", { name: "Color scheme: System" }).click()
  await page.getByRole("menuitemradio", { name: "Dark" }).click()
  await page.reload()
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark")
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("legt eine Statusseite mit Panel an", async({ page }) => {
    await signIn(page)
    const ds = await createDataSource(page)
    const slug = unique("laden")
    await page.goto("/admin/sites/new")
    await page.getByLabel("Name (English)").fill("Laden")
    await page.getByLabel("Adresse").fill(slug)
    await page.getByRole("button", { name: "Statusseite anlegen" }).click()
    await expect(page.getByRole("heading", {
      level: 1,
      name:  "Laden",
    })).toBeVisible()

    await page.getByRole("link", { name: "Panel hinzufügen" }).click()
    await page.getByLabel("Datenquelle").selectOption({ label: ds.name })
    await page.getByLabel("Typ")
      .selectOption({ label: "Kennzahl: ein einzelner Wert" })
    await page.getByRole("textbox", { name: /^Titel English/ }).fill("Antwort")
    await page.getByRole("textbox", { name: "Abfrage" }).fill("4242.5")
    await page.getByLabel("Nachkommastellen").fill("1")
    await page.getByRole("button", { name: "Speichern" }).click()
    await expect(page.locator("[data-panel-id]").filter({ hasText: "Antwort" }))
      .toContainText("4.242,5")

    await page.goto(`/${slug}/de/`)
    await expect(
      page.getByRole("status").filter({ hasText: "Alle Systeme betriebsbereit" }),
    ).toBeVisible()
    await expect(page.locator("[data-panel-id]").filter({ hasText: "Antwort" }))
      .toContainText("4.242,5")
  })
})
