<script setup lang="ts">
import { Plus } from "@lucide/vue"
import { computed, ref, shallowRef, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import IncidentEntry from "../../shared/components/IncidentEntry.vue"
import SRPagination from "../../shared/components/SRPagination.vue"
import { api, ApiError } from "../api"
import { usePageTitle } from "../composables/usePageTitle"
import { resolveText } from "../rules"
import { useNotices } from "../stores/notices"
import type { Incident, Site } from "../types"
import NotFoundView from "./NotFoundView.vue"

const { t, locale } = useI18n()
const route = useRoute()
const notices = useNotices()

const id = route.params.site as string
const site = shallowRef<Site | null>(null)
const list = shallowRef<{
  incidents: Incident[]
  pages:     number
} | null>(null)
const notFound = ref(false)
const name = computed(() => site.value
  ? resolveText(site.value.name, locale.value, site.value.languages)
  : "")
usePageTitle(() => t("incidentList.title", { site: name.value }))

/** page is the requested page, or 0 if the query is not a page number. */
const page = computed(() => {
  const raw = route.query.page ?? "1"
  return typeof raw === "string" && /^[1-9]\d*$/.test(raw) ? Number(raw) : 0
})

watch(page, async(n) => {
  list.value = null
  try {
    if (!n) {
      throw new ApiError(404, "not_found")
    }

    [site.value, list.value] = await Promise.all([
      site.value ?? api<Site>("GET", `/api/admin/sites/${id}`),
      api<{
        incidents: Incident[]
        pages:     number
      }>("GET", `/api/admin/sites/${id}/incidents?page=${n}`),
    ])
  } catch(err) {
    notFound.value = err instanceof ApiError && err.status === 404
    if (!notFound.value) {
      notices.loadFailed(err)
    }
  }
}, { immediate: true })

/**
 * entry returns an incident in the console's language, shaped like a public
 * one.
 */
function entry(incident: Incident) {
  const langs = site.value!.languages
  return {
    ...incident,
    title:   resolveText(incident.title, locale.value, langs),
    updates: incident.updates.map(u => ({
      ...u,
      html: resolveText(u.html, locale.value, langs),
    })),
  }
}
</script>

<template>
  <NotFoundView v-if="notFound" />
  <template v-else-if="site">
    <div class="sr-view-head block">
      <h1 class="title mb-0">
        {{ t("incidentList.title", { site: name }) }}
      </h1>
      <RouterLink
          :to="`/sites/${id}/incidents/new`"
          class="button is-primary"
      >
        <span class="icon"><Plus aria-hidden="true" /></span>
        <span>{{ t("incidentList.new") }}</span>
      </RouterLink>
    </div>
    <template v-if="list">
      <p
          v-if="!list.incidents.length"
          class="block"
      >
        {{ t("incidentList.empty") }}
      </p>
      <ul class="block">
        <li
            v-for="incident in list.incidents"
            :key="incident.id"
            class="block"
        >
          <IncidentEntry
              :incident="entry(incident)"
              :to="`/sites/${id}/incidents/${incident.id}`"
              :timezone="site.timezone"
          >
            <p class="is-size-7 sr-muted mt-1">
              {{ t("incidentEditor.author", { name: incident.author.displayName }) }}
            </p>
          </IncidentEntry>
        </li>
      </ul>
      <SRPagination
          :page
          :pages="list.pages"
          :to="n => ({ query: n > 1 ? { page: n } : {} })"
      />
    </template>
  </template>
</template>

<style scoped>
.sr-view-head {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
  justify-content: space-between;
}
</style>
