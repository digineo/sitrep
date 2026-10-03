import { onBeforeUnmount, onMounted } from "vue"

export interface PollingOptions<T> {
  /** interval returns the time between loads in milliseconds. */
  interval: () => number
  /** paused skips loads and drops results while it returns true. */
  paused?:  () => boolean
  onResult: (value: T) => void
  onError:  (err: unknown) => void
}

/** timeout is the time after which a load is aborted. */
const timeout = 10_000

/**
 * usePolling loads at once and then every interval while the document is
 * visible; becoming visible loads at once. Each load gets a signal that
 * aborts it after 10 seconds. Only the latest load's outcome is applied,
 * so out-of-order responses are dropped.
 */
export function usePolling<T>(
  load: (signal: AbortSignal) => Promise<T>,
  options: PollingOptions<T>,
) {
  let latest = 0
  let timer: ReturnType<typeof setTimeout> | undefined

  function schedule() {
    clearTimeout(timer)
    if (document.visibilityState === "visible") {
      timer = setTimeout(refresh, options.interval())
    }
  }

  /** refresh loads at once and restarts the interval. */
  async function refresh() {
    clearTimeout(timer)
    if (options.paused?.()) {
      schedule()
      return
    }

    const id = ++latest
    const controller = new AbortController()
    const abort = setTimeout(() => controller.abort(), timeout)
    try {
      const value = await load(controller.signal)
      if (id === latest && !options.paused?.()) {
        options.onResult(value)
      }
    } catch(err) {
      if (id === latest) {
        options.onError(err)
      }
    } finally {
      clearTimeout(abort)
      if (id === latest) {
        schedule()
      }
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === "visible") {
      void refresh()
    } else {
      clearTimeout(timer)
    }
  }

  onMounted(() => {
    document.addEventListener("visibilitychange", onVisibilityChange)
    void refresh()
  })
  onBeforeUnmount(() => {
    latest++
    clearTimeout(timer)
    document.removeEventListener("visibilitychange", onVisibilityChange)
  })

  return { refresh }
}
