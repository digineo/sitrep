import { useI18n } from "vue-i18n"

/** useLanguage switches the UI language and remembers the visitor's choice. */
export function useLanguage() {
  const { locale } = useI18n({ useScope: "global" })

  /** apply shows the UI in lang. */
  function apply(lang: string) {
    locale.value = lang
    document.documentElement.lang = lang
  }

  /** remember stores lang as the visitor's explicit choice. */
  function remember(lang: string) {
    const value = encodeURIComponent(lang)
    document.cookie = `lang=${value}; Path=/; SameSite=Lax; Max-Age=31536000`
  }

  /** choose applies and remembers lang. */
  function choose(lang: string) {
    remember(lang)
    apply(lang)
  }
  return {
    locale,
    apply,
    remember,
    choose,
  }
}
