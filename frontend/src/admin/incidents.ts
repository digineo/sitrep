import { fromZonedInput, toZonedInput } from "../shared/format"
import type { IncidentStatus, Severity } from "../shared/payload"
import type { IncidentUpdate, Text } from "./types"

/**
 * UpdateDraft holds the fields of an update form. at is a datetime-local
 * value in the site's time zone; empty status and severity leave them
 * unset.
 */
export interface UpdateDraft {
  at:          string
  status:      IncidentStatus | ""
  severity:    Severity | ""
  description: Text
}

/**
 * draftOf returns the form fields of a stored update, or of a new opening
 * update.
 */
export function draftOf(
  update: IncidentUpdate | null,
  timeZone: string,
): UpdateDraft {
  if (!update) {
    return {
      at:          "",
      status:      "active",
      severity:    "",
      description: {},
    }
  }
  return {
    at:          toZonedInput(new Date(update.at), timeZone),
    status:      update.status ?? "",
    severity:    update.severity ?? "",
    description: { ...update.description },
  }
}

/**
 * updateBody returns the request body of a draft. The time is only sent
 * when it is set and differs from initial, so that an untouched time keeps
 * its seconds and an empty one means now or unchanged.
 */
export function updateBody(draft: UpdateDraft, timeZone: string, initial = "") {
  return {
    at: draft.at && draft.at !== initial
      ? fromZonedInput(draft.at, timeZone).toISOString()
      : undefined,
    status:      draft.status,
    severity:    draft.severity,
    description: draft.description,
  }
}

/**
 * deletable reports whether the update at index may be deleted: the
 * opening update only when the next one opens the incident in its place.
 */
export function deletable(
  updates: Pick<IncidentUpdate, "status">[],
  index: number,
): boolean {
  const next = updates[1]?.status
  return index > 0 || updates.length === 1
    || next === "planned" || next === "active"
}
