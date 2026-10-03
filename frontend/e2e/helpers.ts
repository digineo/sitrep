import { randomUUID } from "node:crypto"

import { expect, type Page } from "@playwright/test"

/** stub is the URL of the Prometheus stub. */
export const stub = "http://127.0.0.1:26090"

/** signIn signs in as the bypass provider's test admin. */
export async function signIn(page: Page, path = "/admin/") {
  await page.goto(`/auth/bypass/login?return=${encodeURIComponent(path)}`)
}

/** unique returns a name no other test uses. */
export function unique(prefix: string): string {
  return `${prefix}-${randomUUID().slice(0, 8)}`
}

/**
 * admin calls the admin API from the signed-in page. Only the browser
 * resolves *.localhost hosts, so the request runs there.
 */
export async function admin<T>(
  page: Page,
  method: string,
  path: string,
  data?: unknown,
): Promise<T> {
  const res = await page.evaluate(
    async({ method, path, data }) => {
      const res = await fetch(path, {
        method,
        headers: { "Content-Type": "application/json" },
        body:    data === undefined ? undefined : JSON.stringify(data),
      })
      return {
        status: res.status,
        body:   await res.text(),
      }
    },
    {
      method,
      path,
      data,
    },
  )
  expect(res.status < 300, `${method} ${path}: ${res.body}`).toBeTruthy()
  return (res.body ? JSON.parse(res.body) : undefined) as T
}

/** createDataSource creates a data source for the stub. */
export async function createDataSource(page: Page): Promise<{
  id:   string
  name: string
}> {
  return admin(page, "POST", "/api/admin/datasources", {
    name:   unique("Stub"),
    type:   "prometheus",
    config: { url: stub },
  })
}

/** createSite creates a path-mode site with the languages. */
export async function createSite(
  page: Page,
  slug: string,
  languages = ["en"],
  mode = "path",
): Promise<{ id: string }> {
  const name = Object.fromEntries(languages.map(l => [l, `${slug} ${l}`]))
  return admin(page, "POST", "/api/admin/sites", {
    name,
    languages: {
      enabled: languages,
      primary: languages[0],
    },
    route: {
      mode,
      slug,
    },
  })
}

/** createPanel creates a panel of a site. */
export async function createPanel(
  page: Page,
  site: string,
  panel: Record<string, unknown>,
): Promise<{ id: string }> {
  return admin(page, "POST", `/api/admin/sites/${site}/panels`, panel)
}

/** updateSite changes settings of a site through the admin API. */
export async function updateSite(
  page: Page,
  id: string,
  changes: Record<string, unknown>,
) {
  const site = await admin<Record<string, unknown>>(
    page,
    "GET",
    `/api/admin/sites/${id}`,
  )
  await admin(page, "PUT", `/api/admin/sites/${id}`, {
    ...site,
    ...changes,
  })
}
