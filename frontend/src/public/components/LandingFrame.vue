<script setup lang="ts">
import { inject } from "vue"
import { useI18n } from "vue-i18n"

import { type Bootstrap } from "../../shared/bootstrap"
import LanguageSwitcher from "../../shared/components/LanguageSwitcher.vue"
import LegalLinks from "../../shared/components/LegalLinks.vue"
import ThemeSwitcher from "../../shared/components/ThemeSwitcher.vue"
import { useLoad } from "../../shared/composables/useLoad"
import type { LegalKind, LegalLinks as Links } from "../../shared/payload"
import { getJSON } from "../siteData"
import { pagePath } from "../urls"

const emit = defineEmits<{ switchLanguage: [lang: string] }>()

const bootstrap = inject<Bootstrap>("bootstrap")!
const { locale } = useI18n()
const { value: legal } = useLoad(
  () => locale.value,
  signal => getJSON<Links>(`/api/public/legal?lang=${locale.value}`, signal),
)
const multi = bootstrap.languages.length > 1
const legalPath = (kind: LegalKind) => pagePath({ kind }, locale.value, multi)
</script>

<template>
  <div class="sr-page">
    <div class="sr-corner">
      <ThemeSwitcher right />
      <LanguageSwitcher
          v-if="bootstrap.languages.length > 1"
          :model-value="locale"
          :languages="bootstrap.languages"
          right
          @update:model-value="emit('switchLanguage', $event)"
      />
    </div>
    <main class="container px-4">
      <RouterView />
    </main>
    <footer
        v-if="legal"
        class="container px-4 py-4"
    >
      <LegalLinks
          :links="legal"
          :path="legalPath"
      />
    </footer>
  </div>
</template>

<style scoped>
/* The default landing page sits in the middle of the viewport. */
main:has(> .sr-landing) {
  display: flex;
  flex-direction: column;
  justify-content: center;
}
</style>
