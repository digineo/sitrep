import { onBeforeUnmount, watch, type WatchSource } from "vue"

/**
 * watchDebounced calls cb once source has not changed for delay
 * milliseconds. Changes are watched deeply.
 */
export function watchDebounced<T>(
  source: WatchSource<T>,
  cb: (value: T) => void,
  delay: number,
) {
  let timer: ReturnType<typeof setTimeout> | undefined
  const debounced = (value: T) => {
    clearTimeout(timer)
    timer = setTimeout(() => cb(value), delay)
  }

  watch(source, debounced, { deep: true })
  onBeforeUnmount(() => clearTimeout(timer))
}
