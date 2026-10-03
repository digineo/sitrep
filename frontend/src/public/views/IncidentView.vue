<script setup lang="ts">
import { ArrowLeft } from "@lucide/vue"
import { useHead } from "@unhead/vue"
import { computed, inject } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import { type Bootstrap } from "../../shared/bootstrap"
import IncidentTags from "../../shared/components/IncidentTags.vue"
import UpdateEntry from "../../shared/components/UpdateEntry.vue"
import { useLoad } from "../../shared/composables/useLoad"
import type { Incident } from "../../shared/payload"
import { getJSON, isNotFound, siteData } from "../siteData"
import { pagePath } from "../urls"

const { t, locale } = useI18n()
const route = useRoute()
const bootstrap = inject<Bootstrap>("bootstrap")!

const id = computed(() => route.params.id as string)
// The incident is fetched again whenever the incidents change.
const { value: incident, error } = useLoad(
  () => `${id.value} ${locale.value}`,
  signal => getJSON<Incident>(
    `/api/public/sites/${bootstrap.siteId}/incidents/${encodeURIComponent(id.value)}?lang=${locale.value}`,
    signal,
  ),
  () => siteData.value?.incidents,
)
const notFound = computed(() => isNotFound(error.value))
const multi = bootstrap.languages.length > 1
const overview = computed(() => pagePath({ kind: "overview" }, locale.value, multi))

useHead({
  title: computed(() => {
    const view = notFound.value ? t("incidents.notFound") : incident.value?.title
    return view && siteData.value
      ? t("page.title", {
        view,
        site: siteData.value.site.name,
      })
      : siteData.value?.site.name ?? ""
  }),
})
</script>

<template>
  <RouterLink
      :to="overview"
      class="sr-back"
  >
    <span class="icon"><ArrowLeft aria-hidden="true" /></span>
    <span>{{ t("incidents.back") }}</span>
  </RouterLink>
  <div class="sr-detail-head block">
    <h2
        class="title is-4 mb-0"
        data-main-heading
    >
      {{ notFound ? t("incidents.notFound") : incident?.title }}
    </h2>
    <IncidentTags
        v-if="incident && !notFound"
        :status="incident.status"
        :severity="incident.severity"
    />
  </div>
  <template v-if="!notFound">
    <div
        v-if="error && !incident"
        class="notification is-danger"
        role="alert"
    >
      {{ t("site.loadFailed") }}
    </div>
    <div
        v-else-if="!incident || !siteData"
        aria-busy="true"
    />
    <ol
        v-else
        class="sr-timeline"
    >
      <li
          v-for="update in incident.updates.toReversed()"
          :key="update.id"
      >
        <UpdateEntry
            :update
            :timezone="siteData.site.timezone"
        />
      </li>
    </ol>
  </template>
</template>

<style scoped>
.sr-detail-head {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}
</style>
