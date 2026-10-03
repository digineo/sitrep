import { catalogs } from "../shared/i18n"
import { useSession } from "./stores/session"

/** ApiError is an error answer of the admin API. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly fields: {
      path: string
      code: string
    }[] = [],
    /** details carry data that explains the error. */
    readonly details?: unknown,
  ) {
    super(code)
  }
}

/** api calls the admin API. A 401 ends the session in the console. */
export async function api<T>(
  method: string,
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      headers: {
        "Content-Type": "application/json",
        "Accept":       "application/json",
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    })
  } catch {
    throw new ApiError(0, "network")
  }

  if (res.status === 401) {
    useSession().expire()
  }
  if (!res.ok) {
    const data = await res.json().catch(() => null)
    throw new ApiError(
      res.status,
      data?.error?.code ?? "internal",
      data?.error?.fields,
      data?.error?.details,
    )
  }
  return (res.status === 204 ? undefined : await res.json()) as T
}

/** errorCode returns the catalog key suffix below "error." for err. */
export function errorCode(err: unknown): string {
  if (err instanceof ApiError && err.code in catalogs.en!.error) {
    return err.code
  }
  return "internal"
}

/** fieldErrors maps the failing fields of a validation error to their codes. */
export function fieldErrors(err: unknown): Record<string, string> {
  if (!(err instanceof ApiError)) {
    return {}
  }
  return Object.fromEntries(err.fields.map(f => [f.path, f.code]))
}

export function isUnauthorized(err: unknown): boolean {
  return err instanceof ApiError && err.status === 401
}

/**
 * localizedErrors picks the errors of a localized field, e.g. "name.de",
 * and translates them, by language.
 */
export function localizedErrors(
  errors: Record<string, string>,
  field: string,
  t: (key: string) => string,
): Record<string, string> {
  return Object.fromEntries(Object.entries(errors)
    .filter(([path]) => path.startsWith(`${field}.`))
    .map(([path, code]) => [path.slice(field.length + 1), t(`error.${code}`)]))
}
