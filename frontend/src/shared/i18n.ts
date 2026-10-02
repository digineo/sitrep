import { createI18n } from "vue-i18n"

import type en from "../../../locales/en.json"

export type Messages = typeof en

const files = import.meta.glob<Messages>("../../../locales/*.json", {
  eager:  true,
  import: "default",
})

/** The translation catalogs by language code, one per file. */
export const catalogs: Record<string, Messages> = Object.fromEntries(
  Object.entries(files).map(([path, messages]) => [
    path.slice(path.lastIndexOf("/") + 1, -".json".length),
    messages,
  ]),
)

/** The supported language codes, sorted. */
export const supported = Object.keys(catalogs).sort()

export function createAppI18n(lang: string) {
  return createI18n({
    legacy:         false,
    locale:         lang,
    fallbackLocale: "en",
    messages:       catalogs,
  })
}

/** endonym returns the language's own name, e.g. "Deutsch". */
export function endonym(lang: string): string {
  return catalogs[lang]?.language.name ?? lang
}
