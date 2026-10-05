import { expect, type Page, test } from "@playwright/test"

import { admin, createSite, unique } from "../helpers"

/** signInAs signs in user with single sign-on on the mock IdP's page. */
async function signInAs(page: Page, user: string) {
  await page.getByRole("link", { name: "Sign in with single sign-on" }).click()
  await page.getByRole("link", {
    name:  user,
    exact: true,
  }).click()
}

test("binds an account added by email at its first sign-in", async({ page, browser }) => {
  await page.goto("/admin/")
  await signInAs(page, "ann")
  const slug = unique("roles")
  const site = await createSite(page, slug)

  await page.goto(`/admin/sites/${site.id}/members`)
  await page.getByRole("button", { name: "Add member" }).click()
  const dialog = page.getByRole("dialog", { name: "Add member" })
  const email = dialog.getByLabel("Email address")
  await email.fill("carol@example.org")
  await expect(dialog.getByText("This address is not at example.com")).toBeVisible()
  await email.fill("carol@example.com")
  await dialog.getByLabel("Role").selectOption({ label: "Responder" })
  await dialog.getByRole("button", { name: "Add member" }).click()
  await expect(dialog).toBeHidden()
  const row = page.getByRole("row", { name: /carol@example\.com/ })
  await expect(row.getByText("Not signed in yet")).toBeVisible()

  const context = await browser.newContext({ baseURL: test.info().project.use.baseURL })
  const carol = await context.newPage()
  await carol.goto("/admin/")
  await signInAs(carol, "carol")
  const nav = carol.getByRole("navigation", { name: "Main navigation" })
  await expect(nav.getByText(`${slug} en`)).toBeVisible()
  await expect(nav.getByRole("link", { name: "Data sources" })).toHaveCount(0)
  await expect(nav.getByRole("link", { name: "Members" })).toHaveCount(0)

  await carol.goto(`/admin/sites/${site.id}/settings`)
  await expect(carol.getByRole("heading", { name: "Page not found" })).toBeVisible()
  await context.close()

  await page.reload()
  await expect(row.getByText("Carol Responder")).toBeVisible()
  await expect(row.getByText("Not signed in yet")).toHaveCount(0)
  await admin(page, "DELETE", `/api/admin/sites/${site.id}`)
})
