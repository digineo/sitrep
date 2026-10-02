import type { Page } from "@playwright/test"

/** signIn signs in as the bypass provider's test admin. */
export async function signIn(page: Page, path = "/admin/") {
  await page.goto(`/auth/bypass/login?return=${encodeURIComponent(path)}`)
}
