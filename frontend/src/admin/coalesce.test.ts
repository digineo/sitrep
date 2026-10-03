import { describe, expect, it } from "vitest"

import { coalesce } from "./coalesce"

describe("coalesce", () => {
  it("runs one save at a time and sends the latest value after it", async() => {
    const saved: number[] = []
    let release!: () => void
    const save = coalesce(async(v: number) => {
      saved.push(v)
      if (v === 1) {
        await new Promise<void>(r => (release = r))
      }
    })

    const first = save(1)
    void save(2)
    void save(3)
    expect(saved).toEqual([1])

    release()
    await first
    expect(saved).toEqual([1, 3])

    await save(4)
    expect(saved).toEqual([1, 3, 4])
  })

  it("drops the merged value after a failure", async() => {
    const saved: number[] = []
    let fail!: (e: Error) => void
    const save = coalesce(async(v: number) => {
      saved.push(v)
      if (v === 1) {
        await new Promise<void>((_, reject) => (fail = reject))
      }
    })

    const first = save(1)
    void save(2)
    fail(new Error("conflict"))
    await expect(first).rejects.toThrow("conflict")
    expect(saved).toEqual([1])

    await save(5)
    expect(saved).toEqual([1, 5])
  })
})
