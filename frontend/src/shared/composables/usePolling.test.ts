import { mount } from "@vue/test-utils"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { defineComponent, h } from "vue"

import { type PollingOptions, usePolling } from "./usePolling"

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", {
    value:        state,
    configurable: true,
  })
  document.dispatchEvent(new Event("visibilitychange"))
}

/** deferred returns a promise and the functions settling it. */
function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return {
    promise,
    resolve,
    reject,
  }
}

function mountPolling<T>(
  load: (signal: AbortSignal) => Promise<T>,
  options: Partial<PollingOptions<T>> = {},
) {
  const results: T[] = []
  const errors: unknown[] = []
  let refresh!: () => Promise<void>
  const wrapper = mount(defineComponent({
    setup() {
      refresh = usePolling(load, {
        interval: () => 1000,
        onResult: v => results.push(v),
        onError:  e => errors.push(e),
        ...options,
      }).refresh
      return () => h("div")
    },
  }))
  return {
    results,
    errors,
    wrapper,
    refresh: () => refresh(),
  }
}

describe("usePolling", () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setVisibility("visible")
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it("loads at once and then every interval", async() => {
    let n = 0
    const { results, wrapper } = mountPolling(async() => ++n)
    await vi.advanceTimersByTimeAsync(0)
    expect(results).toEqual([1])

    await vi.advanceTimersByTimeAsync(999)
    expect(results).toEqual([1])
    await vi.advanceTimersByTimeAsync(1)
    expect(results).toEqual([1, 2])

    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(5000)
    expect(results).toEqual([1, 2])
  })

  it("pauses while hidden and loads when visible again", async() => {
    let n = 0
    const { results } = mountPolling(async() => ++n)
    await vi.advanceTimersByTimeAsync(0)

    setVisibility("hidden")
    await vi.advanceTimersByTimeAsync(5000)
    expect(results).toEqual([1])

    setVisibility("visible")
    await vi.advanceTimersByTimeAsync(0)
    expect(results).toEqual([1, 2])
  })

  it("applies only the latest load", async() => {
    const calls = [deferred<string>(), deferred<string>()]
    let i = 0
    const { results, errors, refresh } = mountPolling(() => calls[i++]!.promise)
    await vi.advanceTimersByTimeAsync(0)
    void refresh()
    calls[1]!.resolve("new")
    calls[0]!.resolve("old")
    await vi.advanceTimersByTimeAsync(0)
    expect(results).toEqual(["new"])
    expect(errors).toEqual([])
  })

  it("reports failures and keeps polling", async() => {
    let fail = true
    const { results, errors } = mountPolling(async() => {
      if (fail) {
        throw new Error("down")
      }
      return "up"
    })
    await vi.advanceTimersByTimeAsync(0)
    expect(errors).toHaveLength(1)

    fail = false
    await vi.advanceTimersByTimeAsync(1000)
    expect(results).toEqual(["up"])
  })

  it("aborts loads after 10 seconds", async() => {
    let signal!: AbortSignal
    mountPolling((s) => {
      signal = s
      return new Promise(() => {})
    })
    await vi.advanceTimersByTimeAsync(0)
    expect(signal.aborted).toBe(false)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(signal.aborted).toBe(true)
  })

  it("skips loads and drops results while paused", async() => {
    let paused = false
    const pending = deferred<string>()
    let calls = 0
    const load = () => {
      calls++
      return pending.promise
    }

    const { results } = mountPolling(load, { paused: () => paused })
    await vi.advanceTimersByTimeAsync(0)
    paused = true
    pending.resolve("during drag")
    await vi.advanceTimersByTimeAsync(3000)
    expect(results).toEqual([])
    expect(calls).toBe(1)
  })
})
