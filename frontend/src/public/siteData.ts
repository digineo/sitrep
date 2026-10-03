import { reactive, shallowRef } from "vue"

import {
  merge,
  type Payload,
  type SiteBasics,
  type SiteData,
} from "../shared/payload"

/** siteData is the state of the site page, shared by its frame and views. */
export const siteData = shallowRef<SiteData | null>(null)

/** unavailable holds what the page shows of an offline or paused site. */
export const unavailable = shallowRef<SiteBasics | null>(null)

/** live is the state of the latest refresh, for the live indicator. */
export const live = reactive({
  state:     "loading" as "loading" | "ok" | "failed",
  updatedAt: undefined as Date | undefined,
  pulse:     0,
  /** error is the catalog key of the latest failure's message. */
  error:     "",
  detail:    "",
})

let dataLang = ""

/** HTTPError is a failed response. */
export class HTTPError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly details?: unknown,
  ) {
    super(`HTTP ${status}`)
  }
}

/** getJSON fetches a path of the public API. */
export async function getJSON<T>(path: string, signal: AbortSignal): Promise<T> {
  const res = await fetch(path, {
    signal,
    headers: { Accept: "application/json" },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    throw new HTTPError(
      res.status,
      body?.error?.code ?? "internal",
      body?.error?.details,
    )
  }
  return await res.json()
}

/** isNotFound reports whether a load failed because the object does not exist. */
export function isNotFound(err: unknown): boolean {
  return err instanceof HTTPError && (err.status === 400 || err.status === 404)
}

/**
 * loadSite fetches the site payload in lang. With data in the same
 * language, only the changes since its cursor are requested.
 */
export async function loadSite(
  siteId: string,
  lang: string,
  signal: AbortSignal,
): Promise<{
  lang:    string
  payload: Payload
}> {
  const since = siteData.value && dataLang === lang
    ? `&since=${encodeURIComponent(siteData.value.cursor)}`
    : ""
  const path = `/api/public/sites/${siteId}?lang=${lang}${since}`
  return {
    lang,
    payload: await getJSON<Payload>(path, signal),
  }
}

/** applySite merges a loaded payload into the page state. */
export function applySite({ lang, payload }: {
  lang:    string
  payload: Payload
}) {
  siteData.value = merge(dataLang === lang ? siteData.value : null, payload)
  unavailable.value = null
  dataLang = lang
  Object.assign(live, {
    state:     "ok",
    updatedAt: new Date(),
    pulse:     live.pulse + 1,
    error:     "",
    detail:    "",
  })
}

/**
 * failSite records a failed refresh; the page keeps its last good state.
 * An unavailable site replaces the state with what the page still shows.
 */
export function failSite(err: unknown) {
  if (err instanceof HTTPError && err.code === "site_unavailable") {
    siteData.value = null
    unavailable.value = err.details as SiteBasics
    return
  }

  live.state = "failed"
  if (err instanceof HTTPError) {
    live.error = `error.${err.code}`
    live.detail = err.message
  } else {
    live.error = "error.network"
    live.detail = err instanceof Error ? err.message : String(err)
  }
}
