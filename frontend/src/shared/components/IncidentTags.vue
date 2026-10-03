<script setup lang="ts">
import { useI18n } from "vue-i18n"

import { statusColors } from "../incidents"
import type { IncidentStatus, Severity } from "../payload"
import SRTag from "./SRTag.vue"

defineProps<{
  status?:   IncidentStatus
  severity?: Severity
}>()

const { t } = useI18n()
const severityTag = {
  minor:    undefined,
  major:    "warning",
  critical: "danger",
} as const
</script>

<template>
  <span class="tags sr-incident-tags">
    <SRTag
        v-if="status"
        :color="statusColors[status]"
    >{{ t(`incidents.status.${status}`) }}</SRTag>
    <SRTag
        v-if="severity"
        :color="severityTag[severity]"
        :light="severity !== 'minor'"
    >{{ t("incidents.severityLabel", { label: t(`incidents.severity.${severity}`) }) }}</SRTag>
  </span>
</template>

<style scoped>
.sr-incident-tags {
  flex-wrap: nowrap;
  margin: 0;
}

.sr-incident-tags > .tag {
  margin-bottom: 0;
}
</style>
