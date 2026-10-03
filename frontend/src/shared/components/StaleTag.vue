<script setup lang="ts">
import { useI18n } from "vue-i18n"

import { formatDateTime } from "../format"
import SRTag from "./SRTag.vue"

defineProps<{
  /** fetchedAt is the time of the stale data. */
  fetchedAt: string
  timezone:  string
}>()

const { t, locale } = useI18n()
</script>

<template>
  <SRTag
      color="warning"
      :title="t('panel.staleSince', { time: formatDateTime(new Date(fetchedAt), locale, timezone) })"
  >
    {{ t("panel.stale") }}
  </SRTag>
</template>
