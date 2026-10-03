<script setup lang="ts">
import { useHead } from "@unhead/vue"
import { computed, inject } from "vue"
import { useI18n } from "vue-i18n"

import { type Bootstrap } from "../../shared/bootstrap"
import IncidentList from "../../shared/components/IncidentList.vue"
import StatusBanner from "../../shared/components/StatusBanner.vue"
import PanelGroups from "../../shared/panels/PanelGroups.vue"
import { live, siteData } from "../siteData"
import { pagePath } from "../urls"

const { t, locale } = useI18n()
const bootstrap = inject<Bootstrap>("bootstrap")!
useHead({ title: computed(() => siteData.value?.site.name ?? "") })

const incidentLink = (id: string) => pagePath(
  {
    kind: "incident",
    id,
  },
  locale.value,
  bootstrap.languages.length > 1,
)
const recent = computed(() => siteData.value?.incidents.finished.slice(0, 5) ?? [])
</script>

<template>
  <div
      v-if="!siteData && live.state === 'failed'"
      class="notification is-danger"
      role="alert"
  >
    {{ t("site.loadFailed") }}
  </div>
  <div
      v-else-if="!siteData"
      aria-busy="true"
  />
  <template v-else>
    <StatusBanner :status="siteData.status" />
    <IncidentList
        v-if="siteData.incidents.ongoing.length"
        :title="t('incidents.current')"
        :incidents="siteData.incidents.ongoing"
        :timezone="siteData.site.timezone"
        :to="incidentLink"
    />
    <IncidentList
        v-if="siteData.incidents.upcoming.length"
        :title="t('incidents.upcoming')"
        :incidents="siteData.incidents.upcoming"
        :timezone="siteData.site.timezone"
        :to="incidentLink"
    />
    <PanelGroups
        :panels="siteData.site.panels"
        :data="siteData.panels"
        :timezone="siteData.site.timezone"
        :spans="siteData.incidents.spans"
        :incident-link
    />
    <IncidentList
        v-if="recent.length"
        :title="t('incidents.recent')"
        :incidents="recent"
        :timezone="siteData.site.timezone"
        :to="incidentLink"
    />
  </template>
</template>
