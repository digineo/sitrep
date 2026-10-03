import { expect, type Page, test } from "@playwright/test"

/** from makes the page's requests come from address, as seen by the server. */
async function from(page: Page, address: string) {
  await page.setExtraHTTPHeaders({ "X-Forwarded-For": address })
}

test("signs in with username and password", async({ page }) => {
  await from(page, "192.0.2.1")
  await page.goto("/admin/settings")
  await page.getByLabel("Username").fill("admin")
  await page.getByLabel("Password").fill("wrong")
  await page.getByRole("button", { name: "Sign in" }).click()
  await expect(page.getByRole("alert"))
    .toHaveText("Sign-in failed. Check your username and password.")

  await page.getByLabel("Password").fill("correct horse")
  await page.getByRole("button", { name: "Sign in" }).click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Settings",
  })).toBeVisible()
  await expect(page.locator("#sr-sidebar").getByText("admin", { exact: true }))
    .toBeVisible()
})

test("refuses further attempts after five failures", async({ page }) => {
  await from(page, "192.0.2.2")
  await page.goto("/admin/")
  await page.getByLabel("Username").fill("throttled")
  for (let i = 0; i < 5; i++) {
    await page.getByLabel("Password").fill(`wrong ${i}`)
    await page.getByRole("button", { name: "Sign in" }).click()
    await expect(page.getByRole("alert"))
      .toHaveText("Sign-in failed. Check your username and password.")
  }

  await page.getByLabel("Password").fill("correct horse")
  await page.getByRole("button", { name: "Sign in" }).click()
  await expect(page.getByRole("alert"))
    .toHaveText("Too many attempts. Please try again later.")
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("signs in with username and password", async({ page }) => {
    await from(page, "192.0.2.3")
    await page.goto("/admin/")
    await page.getByLabel("Benutzername").fill("admin-de")
    await page.getByLabel("Passwort").fill("falsch")
    await page.getByRole("button", { name: "Anmelden" }).click()
    await expect(page.getByRole("alert"))
      .toHaveText("Anmeldung fehlgeschlagen. Prüfen Sie Benutzername und Passwort.")

    await page.getByLabel("Passwort").fill("correct horse")
    await page.getByRole("button", { name: "Anmelden" }).click()
    await expect(page.getByRole("heading", {
      level: 1,
      name:  "Statusseiten",
    })).toBeVisible()
  })

  test("refuses further attempts after five failures", async({ page }) => {
    await from(page, "192.0.2.4")
    await page.goto("/admin/")
    await page.getByLabel("Benutzername").fill("throttled-de")
    for (let i = 0; i < 5; i++) {
      await page.getByLabel("Passwort").fill(`falsch ${i}`)
      await page.getByRole("button", { name: "Anmelden" }).click()
      await expect(page.getByRole("alert")).toHaveText(
        "Anmeldung fehlgeschlagen. Prüfen Sie Benutzername und Passwort.",
      )
    }

    await page.getByRole("button", { name: "Anmelden" }).click()
    await expect(page.getByRole("alert"))
      .toHaveText("Zu viele Versuche. Bitte versuchen Sie es später erneut.")
  })
})
