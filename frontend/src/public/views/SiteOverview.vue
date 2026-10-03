<script setup lang="ts">
import { useHead } from "@unhead/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import StatusBanner from "../../shared/components/StatusBanner.vue"
import PanelGroups from "../../shared/panels/PanelGroups.vue"
import { live, siteData } from "../siteData"

const { t } = useI18n()
useHead({ title: computed(() => siteData.value?.site.name ?? "") })
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
    <PanelGroups
        :panels="siteData.site.panels"
        :data="siteData.panels"
        :timezone="siteData.site.timezone"
    />
  </template>
</template>
