import { describe, expect, it } from "vitest"

import { deletable, draftOf, updateBody } from "./incidents"

const update = {
  id:          "u",
  at:          "2026-10-02T12:05:42Z",
  status:      "monitoring" as const,
  description: { en: "x" },
  author:      {
    subject:     "a",
    displayName: "A",
  },
}

describe("update drafts", () => {
  it("starts a new incident as active", () => {
    expect(draftOf(null, "UTC")).toEqual({
      at:          "",
      status:      "active",
      severity:    "",
      description: {},
    })
  })

  it("shows a stored update in the site's time zone", () => {
    expect(draftOf(update, "Europe/Berlin")).toEqual({
      at:          "2026-10-02T14:05",
      status:      "monitoring",
      severity:    "",
      description: { en: "x" },
    })
  })

  it("sends the time only when it changed", () => {
    const draft = draftOf(update, "Europe/Berlin")
    expect(updateBody(draft, "Europe/Berlin", draft.at).at).toBeUndefined()

    const changed = {
      ...draft,
      at: "2026-10-02T15:00",
    }
    expect(updateBody(changed, "Europe/Berlin", draft.at).at)
      .toBe("2026-10-02T13:00:00.000Z")

    const cleared = {
      ...draft,
      at: "",
    }
    expect(updateBody(cleared, "Europe/Berlin").at).toBeUndefined()
  })
})

describe("deletable", () => {
  it("allows deleting the opening update only when the next one opens", () => {
    expect(deletable([{ status: "active" }, { status: "resolved" }], 0)).toBe(false)
    expect(deletable([{ status: "active" }, {}], 0)).toBe(false)
    expect(deletable([{ status: "planned" }, { status: "active" }], 0)).toBe(true)
    expect(deletable([{ status: "active" }, { status: "resolved" }], 1)).toBe(true)
    expect(deletable([{ status: "active" }], 0)).toBe(true)
  })
})
