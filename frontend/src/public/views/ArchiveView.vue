<script setup lang="ts">
import { ArrowLeft } from "@lucide/vue"
import { useHead } from "@unhead/vue"
import { computed, inject } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import { type Bootstrap } from "../../shared/bootstrap"
import IncidentList from "../../shared/components/IncidentList.vue"
import SRPagination from "../../shared/components/SRPagination.vue"
import { useLoad } from "../../shared/composables/useLoad"
import type { Incident } from "../../shared/payload"
import { getJSON, isNotFound, siteData } from "../siteData"
import { pagePath } from "../urls"
import NotFoundView from "./NotFoundView.vue"

interface Archive {
  incidents: Incident[]
  pages:     number
}

const { t, locale } = useI18n()
const route = useRoute()
const bootstrap = inject<Bootstrap>("bootstrap")!
const multi = bootstrap.languages.length > 1

/** page is the requested page, or 0 if the query is not a page number. */
const page = computed(() => {
  const raw = route.query.page ?? "1"
  return typeof raw === "string" && /^[1-9]\d*$/.test(raw) ? Number(raw) : 0
})
// The page is fetched again whenever the incidents change.
const { value: archive, error } = useLoad(
  () => `${page.value} ${locale.value}`,
  signal => page.value
    ? getJSON<Archive>(
      `/api/public/sites/${bootstrap.siteId}/incidents?lang=${locale.value}&page=${page.value}`,
      signal,
    )
    : Promise.reject(new Error("not a page number")),
  () => siteData.value?.incidents,
)
const notFound = computed(() => !page.value || isNotFound(error.value))
const archivePath = computed(
  () => pagePath({ kind: "archive" }, locale.value, multi),
)
const pageLink = (n: number) => ({
  path:  archivePath.value,
  query: n > 1 ? { page: n } : {},
})
const incidentLink = (id: string) => pagePath(
  {
    kind: "incident",
    id,
  },
  locale.value,
  multi,
)

useHead({
  title: computed(() => siteData.value
    ? t("page.title", {
      view: t("incidents.all"),
      site: siteData.value.site.name,
    })
    : ""),
})
</script>

<template>
  <NotFoundView v-if="notFound" />
  <template v-else>
    <RouterLink
        :to="pagePath({ kind: 'overview' }, locale, multi)"
        class="sr-back"
    >
      <span class="icon"><ArrowLeft aria-hidden="true" /></span>
      <span>{{ t("incidents.back") }}</span>
    </RouterLink>
    <h2
        class="title is-4"
        data-main-heading
    >
      {{ t("incidents.all") }}
    </h2>
    <div
        v-if="error && !archive"
        class="notification is-danger"
        role="alert"
    >
      {{ t("site.loadFailed") }}
    </div>
    <div
        v-else-if="!archive || !siteData"
        aria-busy="true"
    />
    <template v-else>
      <p v-if="!archive.incidents.length">
        {{ t("incidents.none") }}
      </p>
      <IncidentList
          :incidents="archive.incidents"
          :timezone="siteData.site.timezone"
          :to="incidentLink"
      />
      <SRPagination
          :page
          :pages="archive.pages"
          :to="pageLink"
      />
    </template>
  </template>
</template>
