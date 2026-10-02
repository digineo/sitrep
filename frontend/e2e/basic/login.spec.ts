import { expect, test } from "@playwright/test"

test("signs in with username and password", async({ page }) => {
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
