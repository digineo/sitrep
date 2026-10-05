import { expect, test } from "@playwright/test"

import { admin, createSite, signIn, unique } from "../helpers"

test("shows responders only what they may do", async({ page }) => {
  await signIn(page)
  const slug = unique("responders")
  const site = await createSite(page, slug)
  await admin(page, "POST", `/api/admin/sites/${site.id}/members`, {
    login: "test-responder",
    role:  "responder",
  })

  const path = `/admin/sites/${site.id}`
  await page.goto(`/auth/bypass/login?as=test-responder&return=${encodeURIComponent(path)}`)
  await expect(page.getByRole("heading", {
    level: 1,
    name:  `${slug} en`,
  })).toBeVisible()
  await expect(page.getByRole("link", { name: "Add panel" })).toHaveCount(0)

  const nav = page.getByRole("navigation", { name: "Main navigation" })
  await expect(nav.getByRole("link", { name: "Incidents" })).toBeVisible()
  await expect(nav.getByRole("link", { name: "Data sources" })).toHaveCount(0)
  await expect(nav.getByRole("link", { name: "Settings" })).toHaveCount(0)

  await page.goto("/admin/settings")
  await expect(page.getByRole("heading", { name: "Page not found" })).toBeVisible()
})
