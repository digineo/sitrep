import { describe, expect, it } from "vitest"

import { bandStyle, textOn } from "./contrast"

describe("textOn", () => {
  it("picks the text color with the higher contrast ratio", () => {
    expect(textOn("#ffffff")).toBe("#000000")
    expect(textOn("#000000")).toBe("#ffffff")
    expect(textOn("#ffcc00")).toBe("#000000")
    expect(textOn("#112233")).toBe("#ffffff")
    expect(textOn("#e5484d")).toBe("#000000")
    expect(textOn("#1d5bd6")).toBe("#ffffff")
  })

  it("decides close calls by the ratio, not by lightness", () => {
    // Black reaches 4.59:1 on the product blue, white 4.57:1.
    expect(textOn("#2f6feb")).toBe("#000000")
    expect(textOn("#2e6deb")).toBe("#ffffff")
  })
})

describe("bandStyle", () => {
  it("sets the band and text colors", () => {
    expect(bandStyle("#112233")).toEqual({
      "--sr-band":      "#112233",
      "--sr-band-text": "#ffffff",
    })
  })

  it("leaves bands transparent without a brand color", () => {
    expect(bandStyle(undefined)).toBeUndefined()
    expect(bandStyle("")).toBeUndefined()
  })
})
