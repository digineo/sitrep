import type { Component } from "vue"
import type { RouteRecordRaw } from "vue-router"

import { catalogs } from "../shared/i18n"

export type PageKind = "overview" | "archive" | "incident" | "imprint" | "privacy"

export interface Page {
  kind: PageKind
  id?:  string
}

/**
 * pagePath returns the path of a page in lang, below the base path. With
 * several enabled languages, the path starts with the language. Legal
 * pages use the slugs of lang.
 */
export function pagePath(page: Page, lang: string, multi: boolean): string {
  const paths: Record<PageKind, () => string> = {
    overview: () => "/",
    archive:  () => "/incidents",
    incident: () => `/incidents/${page.id}`,
    imprint:  () => `/${catalogs[lang]!.legal.imprint.slug}`,
    privacy:  () => `/${catalogs[lang]!.legal.privacy.slug}`,
  }
  const path = paths[page.kind]()
  return multi ? `/${lang}${path}` : path
}

declare module "vue-router" {
  interface RouteMeta {
    lang?: string
    page?: PageKind
  }
}

/** pageRoutes returns a route per page view in every enabled language. */
export function pageRoutes(
  languages: string[],
  views: Partial<Record<PageKind, Component>>,
): RouteRecordRaw[] {
  const multi = languages.length > 1
  const entries = Object.entries(views)
  return languages.flatMap(lang => entries.map(([kind, component]) => ({
    path: pagePath(
      {
        kind: kind as PageKind,
        id:   ":id",
      },
      lang,
      multi,
    ),
    component,
    meta: {
      lang,
      page: kind as PageKind,
    },
  })))
}
