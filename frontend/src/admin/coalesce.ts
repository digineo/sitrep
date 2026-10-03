/**
 * coalesce wraps an async save so that at most one call runs at a time.
 * Calls made meanwhile are merged: only the latest value is saved after
 * the running call. The first call's promise settles when no save is left;
 * a failure drops the merged value.
 */
export function coalesce<T>(
  save: (value: T) => Promise<void>,
): (value: T) => Promise<void> {
  let running = false
  let next: { value: T } | null = null
  return async(value: T) => {
    next = { value }
    if (running) {
      return
    }

    running = true
    try {
      while (next) {
        const current: T = next.value
        next = null
        await save(current)
      }
    } finally {
      running = false
      next = null
    }
  }
}
