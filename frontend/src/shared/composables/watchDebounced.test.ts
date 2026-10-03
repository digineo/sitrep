import { mount } from "@vue/test-utils"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { defineComponent, h, nextTick, ref } from "vue"

import { watchDebounced } from "./watchDebounced"

describe("watchDebounced", () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it("calls back once the source settled", async() => {
    const source = ref({ q: "" })
    const calls: string[] = []
    const wrapper = mount(defineComponent({
      setup() {
        watchDebounced(source, v => calls.push(v.q), 500)
        return () => h("div")
      },
    }))
    source.value.q = "u"
    await nextTick()
    vi.advanceTimersByTime(300)
    source.value.q = "up"
    await nextTick()
    vi.advanceTimersByTime(499)
    expect(calls).toEqual([])
    vi.advanceTimersByTime(1)
    expect(calls).toEqual(["up"])

    source.value.q = "upx"
    await nextTick()
    wrapper.unmount()
    vi.advanceTimersByTime(1000)
    expect(calls).toEqual(["up"])
  })
})
