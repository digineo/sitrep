import { expect, test } from "@playwright/test"

import { signIn } from "../helpers"

test("signs in with single sign-on and out again", async({ page }) => {
  await page.goto("/admin/settings")
  await expect(page.getByText("Sign in to continue.")).toBeVisible()
  await expect(page).toHaveTitle("Sign in · SitRep admin")
  await page.getByRole("link", { name: "Sign in with single sign-on" }).click()

  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Settings",
  })).toBeVisible()
  await expect(page).toHaveURL("/admin/settings")
  await expect(page.getByRole("navigation", { name: "Main navigation" }))
    .toBeVisible()
  await expect(page.getByText("Test Admin")).toBeVisible()

  await page.getByRole("button", { name: "Sign out" }).click()
  await expect(page.getByText("You have been signed out.")).toBeVisible()
  await expect(page).toHaveURL("/admin/")

  await page.reload()
  await expect(page.getByText("Sign in to continue.")).toBeVisible()
  await expect(page.getByText("You have been signed out.")).toBeHidden()
})

test("dismisses the signed-out notice", async({ page }) => {
  await signIn(page)
  await page.getByRole("button", { name: "Sign out" }).click()
  await page.getByRole("button", { name: "Dismiss" }).click()
  await expect(page.getByText("You have been signed out.")).toBeHidden()
})

test.describe("in German", () => {
  test.use({ locale: "de-DE" })

  test("signs in and shows the console in German", async({ page }) => {
    await page.goto("/admin/")
    await expect(page.getByText("Melden Sie sich an, um fortzufahren."))
      .toBeVisible()
    await page.getByRole("link", { name: "Mit Single Sign-on anmelden" }).click()
    await expect(page.getByRole("heading", {
      level: 1,
      name:  "Statusseiten",
    })).toBeVisible()
    await expect(page.locator("html")).toHaveAttribute("lang", "de")
    await expect(page).toHaveTitle("Statusseiten · SitRep-Verwaltung")
  })
})

test("remembers the chosen language and color scheme", async({ page }) => {
  await signIn(page)
  const sidebar = page.locator("#sr-sidebar")

  await sidebar.getByRole("button", { name: "Language: English" }).click()
  await page.getByRole("menuitemradio", { name: "Deutsch" }).click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Statusseiten",
  })).toBeVisible()
  await expect(page.locator("html")).toHaveAttribute("lang", "de")

  await sidebar.getByRole("button", { name: "Farbschema: System" }).click()
  await page.getByRole("menuitemradio", { name: "Dunkel" }).click()
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark")

  await page.reload()
  await expect(page.locator("html")).toHaveAttribute("lang", "de")
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Statusseiten",
  })).toBeVisible()
})

test("operates the theme switcher with the keyboard", async({ page }) => {
  await signIn(page)
  const trigger = page.locator("#sr-sidebar")
    .getByRole("button", { name: "Color scheme: System" })
  await trigger.focus()
  await page.keyboard.press("ArrowDown")
  await expect(page.getByRole("menuitemradio", { name: "Light" })).toBeFocused()

  await page.keyboard.press("ArrowDown")
  await page.keyboard.press("Enter")
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark")
  await expect(
    page.locator("#sr-sidebar").getByRole("button", { name: "Color scheme: Dark" }),
  ).toBeFocused()

  await page.keyboard.press("ArrowUp")
  await page.keyboard.press("Escape")
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark")
})

test("opens the sidebar as drawer on small screens", async({ page }) => {
  await page.setViewportSize({
    width:  600,
    height: 800,
  })
  await signIn(page)
  const menu = page.getByRole("button", { name: "Menu" })
  await expect(menu).toHaveAttribute("aria-expanded", "false")
  await expect(page.getByRole("link", { name: "Settings" })).toBeHidden()

  await menu.click()
  await expect(menu).toHaveAttribute("aria-expanded", "true")
  await expect(page.locator("#sr-sidebar").getByRole("link", { name: "SitRep" }))
    .toBeFocused()
  await page.keyboard.press("Shift+Tab")
  await expect(page.locator("#sr-sidebar").getByRole("button").last())
    .toBeFocused()
  await page.keyboard.press("Tab")
  await expect(page.locator("#sr-sidebar").getByRole("link", { name: "SitRep" }))
    .toBeFocused()
  await page.keyboard.press("Escape")
  await expect(menu).toHaveAttribute("aria-expanded", "false")
  await expect(menu).toBeFocused()

  await menu.click()
  await page.getByRole("link", { name: "Settings" }).click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Settings",
  })).toBeFocused()
  await expect(menu).toHaveAttribute("aria-expanded", "false")
})

test("asks before leaving unsaved changes", async({ page }) => {
  await signIn(page, "/admin/settings")
  await page.getByLabel("Default color scheme").selectOption("dark")
  await page.locator("#sr-sidebar").getByRole("link", { name: "SitRep" }).click()

  const dialog = page.getByRole("dialog", { name: "Discard changes?" })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole("button", { name: "Cancel" })).toBeFocused()
  await page.keyboard.press("Escape")
  await expect(dialog).toBeHidden()
  await expect(page).toHaveURL("/admin/settings")

  await page.locator("#sr-sidebar").getByRole("link", { name: "SitRep" }).click()
  await dialog.getByRole("button", { name: "Discard changes" }).click()
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Status pages",
  })).toBeVisible()
})

test("shows a not-found view for unknown console routes", async({ page }) => {
  await signIn(page, "/admin/nope")
  await expect(page.getByRole("heading", {
    level: 1,
    name:  "Page not found",
  })).toBeVisible()
})
