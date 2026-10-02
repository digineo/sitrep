<script setup lang="ts">
import { inject, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import { type Bootstrap } from "../shared/bootstrap"
import LanguageSwitcher from "../shared/components/LanguageSwitcher.vue"
import ThemeSwitcher from "../shared/components/ThemeSwitcher.vue"
import { useLanguage } from "../shared/composables/useLanguage"
import { pagePath } from "./urls"

const bootstrap = inject<Bootstrap>("bootstrap")!
const route = useRoute()
const router = useRouter()
const { locale, apply, remember } = useLanguage()

// The URL's language is the page's language.
watch(
  () => route.meta.lang,
  lang => apply(lang ?? bootstrap.lang),
  { immediate: true },
)

// Switching the language opens the same page in that language.
async function switchLanguage(lang: string) {
  remember(lang)
  const page = route.meta.page ?? "overview"
  const target = {
    kind: page,
    id:   route.params.id as string | undefined,
  }
  await router.push(pagePath(target, lang, true))
}
</script>

<template>
  <div class="sr-corner">
    <ThemeSwitcher right />
    <LanguageSwitcher
        v-if="bootstrap.languages.length > 1"
        :model-value="locale"
        :languages="bootstrap.languages"
        right
        @update:model-value="switchLanguage"
    />
  </div>
  <main class="container px-4">
    <RouterView />
  </main>
</template>
