import { ApiError, request } from "./api"
import type { Site } from "./types"

/**
 * importSite creates a site from an exported YAML file or, with id, replaces
 * that site's settings and panels.
 */
export async function importSite(file: File, id?: string): Promise<Site> {
  const yaml = await file.text()
  const res = id
    ? await request("PUT", `/api/admin/sites/${id}/import`, yaml, "application/yaml")
    : await request("POST", "/api/admin/sites/import", yaml, "application/yaml")
  return await res.json()
}

/** exportSite downloads the YAML export of a site as name.yaml. */
export async function exportSite(id: string, name: string) {
  const blob = await (await request("GET", `/api/admin/sites/${id}/export`)).blob()
  const a = document.createElement("a")
  a.href = URL.createObjectURL(blob)
  a.download = `${name}.yaml`
  a.click()
  URL.revokeObjectURL(a.href)
}

/**
 * importDetail tells where a file failed to import: the line it cannot be
 * read at, or the failing fields.
 */
export function importDetail(
  err: unknown,
  t: (key: string, params?: Record<string, unknown>) => string,
): string | undefined {
  if (!(err instanceof ApiError)) {
    return undefined
  }

  const line = (err.details as { line?: number } | undefined)?.line
  if (err.code === "invalid_yaml" && line) {
    return t("import.line", { line })
  }
  return err.fields.map(f => `${f.path}: ${t(`error.${f.code}`)}`).join("\n")
    || undefined
}
