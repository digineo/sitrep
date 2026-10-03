<script setup lang="ts">
import { inject, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import { type Bootstrap } from "../shared/bootstrap"
import { useLanguage } from "../shared/composables/useLanguage"
import LandingFrame from "./components/LandingFrame.vue"
import SiteFrame from "./components/SiteFrame.vue"
import { pagePath } from "./urls"

const bootstrap = inject<Bootstrap>("bootstrap")!
const route = useRoute()
const router = useRouter()
const { apply, remember } = useLanguage()

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
  <SiteFrame
      v-if="bootstrap.mode === 'site'"
      @switch-language="switchLanguage"
  />
  <LandingFrame
      v-else
      @switch-language="switchLanguage"
  />
</template>
