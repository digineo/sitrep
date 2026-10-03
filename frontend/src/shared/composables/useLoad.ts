import { onBeforeUnmount, shallowRef, watch, type WatchSource } from "vue"

/** timeout is the time after which a load is aborted. */
const timeout = 10_000

/**
 * useLoad loads whenever key changes, dropping the previous value first, so
 * that a view never shows another key's data. A change of refresh loads
 * again and keeps the value until the new one arrives. Only the latest
 * load's outcome is applied, and each load is aborted after 10 seconds.
 */
export function useLoad<T>(
  key: () => string,
  load: (signal: AbortSignal) => Promise<T>,
  refresh?: WatchSource,
) {
  const value = shallowRef<T>()
  const error = shallowRef<unknown>()
  let latest = 0

  async function run(clear: boolean) {
    const id = ++latest
    if (clear) {
      value.value = undefined
      error.value = undefined
    }

    try {
      const v = await load(AbortSignal.timeout(timeout))
      if (id === latest) {
        value.value = v
        error.value = undefined
      }
    } catch(err) {
      if (id === latest) {
        error.value = err
      }
    }
  }

  watch(key, () => void run(true), { immediate: true })
  if (refresh) {
    watch(refresh, () => void run(false))
  }

  onBeforeUnmount(() => latest++)
  return {
    value,
    error,
  }
}
