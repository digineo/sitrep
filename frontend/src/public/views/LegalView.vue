<script setup lang="ts">
import { ArrowLeft } from "@lucide/vue"
import { useHead } from "@unhead/vue"
import { computed, inject } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import { type Bootstrap } from "../../shared/bootstrap"
import { useLoad } from "../../shared/composables/useLoad"
import type { LegalKind } from "../../shared/payload"
import { getJSON, isNotFound, siteData, unavailable } from "../siteData"
import { pagePath } from "../urls"
import NotFoundView from "./NotFoundView.vue"

const { t, locale } = useI18n()
const route = useRoute()
const bootstrap = inject<Bootstrap>("bootstrap")!
const site = bootstrap.mode === "site"

const kind = computed(() => route.meta.page as LegalKind)
const api = computed(() => site
  ? `/api/public/sites/${bootstrap.siteId}/legal/${kind.value}`
  : `/api/public/legal/${kind.value}`)
// A page is dropped as soon as another one is requested.
const { value: page, error } = useLoad(
  () => `${kind.value} ${locale.value}`,
  signal => getJSON<{ html: string }>(`${api.value}?lang=${locale.value}`, signal),
)
const title = computed(() => t(`legal.${kind.value}.title`))
const owner = computed(() => site
  ? siteData.value?.site.name ?? unavailable.value?.name ?? ""
  : t("page.landing"))
useHead({
  title: computed(() => t("page.title", {
    view: title.value,
    site: owner.value,
  })),
})
</script>

<template>
  <NotFoundView v-if="isNotFound(error)" />
  <template v-else>
    <RouterLink
        :to="pagePath({ kind: 'overview' }, locale, bootstrap.languages.length > 1)"
        class="sr-back"
    >
      <span class="icon"><ArrowLeft aria-hidden="true" /></span>
      <span>{{ site ? t("incidents.back") : t("legal.toLanding") }}</span>
    </RouterLink>
    <component
        :is="site ? 'h2' : 'h1'"
        class="title is-4"
        data-main-heading
    >
      {{ title }}
    </component>
    <div
        v-if="error"
        class="notification is-danger"
        role="alert"
    >
      {{ t("legal.loadFailed") }}
    </div>
    <div
        v-else-if="!page"
        aria-busy="true"
    />
    <!-- eslint-disable vue/no-v-html -- server-rendered Markdown -->
    <div
        v-else
        class="content"
        v-html="page.html"
    />
    <!-- eslint-enable vue/no-v-html -->
  </template>
</template>
