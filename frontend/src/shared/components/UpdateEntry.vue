<script setup lang="ts">
import { useI18n } from "vue-i18n"

import { formatDateTime } from "../format"
import type { IncidentUpdate } from "../payload"
import IncidentTags from "./IncidentTags.vue"

defineProps<{
  update:   Pick<IncidentUpdate, "at" | "status" | "severity" | "html">
  timezone: string
}>()

defineSlots<{
  /** default adds content below the description. */
  default?: () => unknown
}>()

const { locale } = useI18n()
</script>

<template>
  <div class="sr-update">
    <div class="sr-update-head">
      <time :datetime="update.at">{{ formatDateTime(new Date(update.at), locale, timezone) }}</time>
      <IncidentTags
          :status="update.status"
          :severity="update.severity"
      />
    </div>
    <!-- eslint-disable vue/no-v-html -- server-rendered Markdown -->
    <div
        class="content mt-2"
        v-html="update.html"
    />
    <!-- eslint-enable vue/no-v-html -->
    <slot />
  </div>
</template>

<style scoped>
.sr-update-head {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  font-weight: 600;
}
</style>
