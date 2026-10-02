import { onBeforeUnmount, onMounted } from "vue"
import { useI18n } from "vue-i18n"
import { onBeforeRouteLeave } from "vue-router"

import { useConfirm } from "../../shared/composables/useConfirm"

/** useUnsavedChanges asks before leaving a form with unsaved changes. */
export function useUnsavedChanges(dirty: () => boolean) {
  const { t } = useI18n()
  const { confirm } = useConfirm()

  onBeforeRouteLeave(() => !dirty() || confirm({
    title:   t("unsaved.title"),
    message: t("unsaved.message"),
    confirm: t("unsaved.discard"),
    danger:  true,
  }))

  function onBeforeUnload(event: BeforeUnloadEvent) {
    if (dirty()) {
      event.preventDefault()
    }
  }

  onMounted(() => window.addEventListener("beforeunload", onBeforeUnload))
  onBeforeUnmount(() => window.removeEventListener("beforeunload", onBeforeUnload))
}
