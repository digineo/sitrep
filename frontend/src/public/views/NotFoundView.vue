<script setup lang="ts">
import { useHead } from "@unhead/vue"
import { computed, inject } from "vue"
import { useI18n } from "vue-i18n"

import { type Bootstrap } from "../../shared/bootstrap"
import { pagePath } from "../urls"

const { t, locale } = useI18n()
const bootstrap = inject<Bootstrap>("bootstrap")!
useHead({
  title: computed(() => t("page.title", {
    view: t("page.notFound"),
    site: t("page.landing"),
  })),
})
const multi = bootstrap.languages.length > 1
const overview = computed(
  () => bootstrap.basePath + pagePath({ kind: "overview" }, locale.value, multi),
)
</script>

<template>
  <h1 class="title">
    {{ t("page.notFound") }}
  </h1>
  <p class="block">
    {{ t("notFound.text") }}
  </p>
  <RouterLink :to="overview">
    {{ t("notFound.toOverview") }}
  </RouterLink>
</template>
