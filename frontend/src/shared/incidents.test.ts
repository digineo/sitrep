import { describe, expect, it } from "vitest"

import { cut, excerpt, plainText } from "./incidents"

describe("excerpts", () => {
  it("takes the text of rendered HTML with whitespace collapsed", () => {
    expect(plainText(
      "<p>Some <strong>bold</strong>\ntext</p>\n<ul>\n<li>one</li>\n</ul>\n",
    )).toBe("Some bold text one")
    expect(plainText("<p>Fish &amp; chips</p>")).toBe("Fish & chips")
  })

  it("cuts at a word boundary where possible", () => {
    expect(cut("Short", 200)).toBe("Short")
    expect(cut("a".repeat(200), 200)).toBe("a".repeat(200))
    expect(cut("one two three", 9)).toBe("one two…")
    expect(cut("one two three", 8)).toBe("one two…")
    expect(cut("abcdefghijk", 8)).toBe("abcdefgh…")
  })

  it("counts code points", () => {
    expect(cut("😀😀😀 😀😀", 4)).toBe("😀😀😀…")
    expect(cut("äöü äöü äöü", 9)).toBe("äöü äöü…")
  })

  it("uses the newest update with a description", () => {
    const update = (html: string) => ({
      id: "u",
      at: "2026-10-02T12:00:00Z",
      html,
    })
    const updates = [update("<p>First</p>"), update("<p>Second</p>"), update("")]
    expect(excerpt({ updates })).toBe("Second")
    expect(excerpt({ updates: [update("")] })).toBe("")
  })
})
