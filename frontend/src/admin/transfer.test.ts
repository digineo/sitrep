import { describe, expect, it } from "vitest"

import { ApiError } from "./api"
import { importDetail } from "./transfer"

const t = (key: string, params?: Record<string, unknown>) => params
  ? `${key} ${JSON.stringify(params)}`
  : key

describe("importDetail", () => {
  it("names the line a file cannot be read at", () => {
    const err = new ApiError(400, "invalid_yaml", [], { line: 7 })
    expect(importDetail(err, t)).toBe(`import.line {"line":7}`)
  })

  it("lists the failing fields by path", () => {
    const err = new ApiError(400, "invalid", [
      {
        path: "panels[2].title.de",
        code: "required",
      },
      {
        path: "version",
        code: "unsupported_version",
      },
    ])
    expect(importDetail(err, t)).toBe(
      "panels[2].title.de: error.required\n"
      + "version: error.unsupported_version",
    )
  })

  it("has nothing to add to other errors", () => {
    expect(importDetail(new ApiError(413, "too_large"), t)).toBeUndefined()
    expect(importDetail(new Error("x"), t)).toBeUndefined()
  })
})
