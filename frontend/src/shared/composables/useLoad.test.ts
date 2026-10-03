import { mount } from "@vue/test-utils"
import { describe, expect, it } from "vitest"
import { defineComponent, nextTick, ref } from "vue"

import { useLoad } from "./useLoad"

/** deferred returns a promise with its resolve and reject functions. */
function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (err: unknown) => void
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

function setup() {
  const key = ref("a")
  const tick = ref(0)
  const pending: ReturnType<typeof deferred<string>>[] = []
  let state!: ReturnType<typeof useLoad<string>>
  mount(defineComponent({
    setup() {
      const load = () => {
        const d = deferred<string>()
        pending.push(d)
        return d.promise
      }

      state = useLoad(() => key.value, load, tick)
      return () => null
    },
  }))
  return {
    key,
    tick,
    pending,
    state,
  }
}

const flush = () => new Promise(resolve => setTimeout(resolve))

describe("useLoad", () => {
  it("drops the value when the key changes and keeps it on refresh", async() => {
    const { key, tick, pending, state } = setup()
    pending[0]!.resolve("a1")
    await flush()
    expect(state.value.value).toBe("a1")

    tick.value++
    await nextTick()
    expect(state.value.value).toBe("a1")

    pending[1]!.reject(new Error("down"))
    await flush()
    expect(state.value.value).toBe("a1")
    expect(state.error.value).toBeInstanceOf(Error)

    key.value = "b"
    await nextTick()
    expect(state.value.value).toBeUndefined()
    expect(state.error.value).toBeUndefined()
  })

  it("applies only the latest load", async() => {
    const { key, pending, state } = setup()
    key.value = "b"
    await nextTick()
    pending[1]!.resolve("b1")
    pending[0]!.resolve("a1")
    await flush()
    expect(state.value.value).toBe("b1")
  })
})
