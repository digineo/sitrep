import { defineStore } from "pinia"
import { ref } from "vue"

import { errorCode, isUnauthorized } from "../api"

export interface Toast {
  id:      number
  kind:    "success" | "failure"
  /** text is the message of a success, or the prefix of a failure. */
  text:    string
  /** code is the error code of a failure. */
  code?:   string
  /** detail explains a failure further, e.g. where an import failed. */
  detail?: string
}

/** useNotices holds toasts and the load error banner. */
export const useNotices = defineStore("notices", () => {
  const toasts = ref<Toast[]>([])
  /** loadError is the error code of a failed load, shown above the view. */
  const loadError = ref<string | null>(null)
  let lastId = 0

  function close(id: number) {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }

  /** success shows a toast that hides after 4 seconds. */
  function success(text: string) {
    const id = ++lastId
    toasts.value.push({
      id,
      kind: "success",
      text,
    })
    setTimeout(() => close(id), 4000)
  }

  /**
   * failure shows a toast until it is closed. A 401 shows none, it opens
   * the login screen.
   */
  function failure(prefix: string, err: unknown, detail?: string) {
    if (!isUnauthorized(err)) {
      toasts.value.push({
        id:   ++lastId,
        kind: "failure",
        text: prefix,
        code: errorCode(err),
        detail,
      })
    }
  }

  function loadFailed(err: unknown) {
    if (!isUnauthorized(err)) {
      loadError.value = errorCode(err)
    }
  }
  return {
    toasts,
    loadError,
    close,
    success,
    failure,
    loadFailed,
  }
})
