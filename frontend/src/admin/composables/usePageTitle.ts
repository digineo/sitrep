import { useHead } from "@unhead/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"

/** usePageTitle sets the document title to "<view> · SitRep admin". */
export function usePageTitle(view: () => string) {
  const { t } = useI18n()
  useHead({ title: computed(() => t("page.adminTitle", { view: view() })) })
}
