import { expect, type Page, test } from "@playwright/test"

/** signInAs starts single sign-on and picks user on the mock IdP's page. */
async function signInAs(page: Page, sso: string, user: string) {
  await page.getByRole("link", { name: sso }).click()
  await page.getByRole("link", {
    name:  user,
    exact: true,
  }).click()
}

test("signs in with single sign-on", async({ page }) => {
  await page.goto("/admin/settings")
  await signInAs(page, "Sign in with single sign-on", "ann")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Settings",
  })).toBeVisible()
  await expect(page).toHaveURL("/admin/settings")
  await expect(page.locator("#sr-sidebar").getByText("Ann Admin")).toBeVisible()
})

test("refuses users outside the admin group", async({ page }) => {
  await page.goto("/admin/settings")
  await signInAs(page, "Sign in with single sign-on", "bob")
  const notice = page.getByRole("alert")
  await expect(notice)
    .toHaveText("Your account is not a member of the admin group. Ask whoever manages your accounts for access.")
  await expect(page).toHaveURL("/admin/settings")
  await notice.getByRole("button", { name: "Dismiss" }).click()
  await expect(notice).toBeHidden()

  await signInAs(page, "Sign in with single sign-on", "ann")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Settings",
  })).toBeVisible()
})

test("reports a cancelled sign-in", async({ page }) => {
  await page.goto("/admin/")
  await signInAs(page, "Sign in with single sign-on", "cancel")
  await expect(page.getByRole("alert"))
    .toHaveText("Your identity provider denied the sign-in.")
})

test("ignores unknown error codes", async({ page }) => {
  await page.goto("/admin/?login-error=toString")
  await expect(page.getByRole("link", { name: "Sign in with single sign-on" }))
    .toBeVisible()
  await expect(page).toHaveURL("/admin/")
  await expect(page.getByRole("alert")).toHaveCount(0)
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("signs in with single sign-on", async({ page }) => {
    await page.goto("/admin/")
    await signInAs(page, "Mit Single Sign-on anmelden", "bob")
    await expect(page.getByRole("alert"))
      .toHaveText("Ihr Konto gehört nicht zur Admin-Gruppe. Wenden Sie sich für den Zugang an die Verwaltung Ihrer Konten.")

    await signInAs(page, "Mit Single Sign-on anmelden", "ann")
    await expect(page.getByRole("heading", {
      level: 1,
      name:  "Statusseiten",
    })).toBeVisible()
  })
})
