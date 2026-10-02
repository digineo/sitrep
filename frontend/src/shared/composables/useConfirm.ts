import { shallowRef } from "vue"

export interface ConfirmOptions {
  title:   string
  message: string
  /** confirm labels the confirming button, e.g. "Delete status page". */
  confirm: string
  /** danger styles the confirming button for destructive actions. */
  danger?: boolean
}

interface Pending extends ConfirmOptions {
  resolve: (confirmed: boolean) => void
}

const pending = shallowRef<Pending | null>(null)

/** useConfirm asks the user in the confirmation dialog. */
export function useConfirm() {
  /** confirm resolves to whether the user confirmed. */
  function confirm(options: ConfirmOptions): Promise<boolean> {
    pending.value?.resolve(false)
    return new Promise((resolve) => {
      pending.value = {
        ...options,
        resolve,
      }
    })
  }
  return {
    confirm,
    pending,
  }
}
