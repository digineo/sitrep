<script setup lang="ts">
import { useI18n } from "vue-i18n"

import SRStatusDot from "../components/SRStatusDot.vue"
import StaleTag from "../components/StaleTag.vue"
import type { PanelData, PanelInfo, StatusData } from "../payload"

defineProps<{
  panels:   PanelInfo[]
  data:     Record<string, PanelData>
  timezone: string
}>()

const { t } = useI18n()

/** state is a status panel's state: its threshold state while fresh. */
function state(d?: PanelData) {
  return d?.state === "fresh" ? (d.data as StatusData).state : "unknown"
}
</script>

<template>
  <table class="table is-fullwidth sr-status-table">
    <tbody>
      <tr
          v-for="panel in panels"
          :key="panel.id"
          :data-panel-id="panel.id"
      >
        <th scope="row">
          <span class="sr-panel-head">
            <span>{{ panel.title }}</span>
            <StaleTag
                v-if="data[panel.id]?.state === 'stale'"
                :fetched-at="data[panel.id]!.fetchedAt!"
                :timezone
            />
            <slot
                name="actions"
                :panel
            />
          </span>
          <span
              v-if="panel.description"
              class="sr-muted sr-description is-block"
          >{{ panel.description }}</span>
        </th>
        <td class="sr-muted sr-description-column">
          {{ panel.description }}
        </td>
        <td class="sr-state">
          <span
              v-if="!data[panel.id] || data[panel.id]!.state === 'pending'"
              class="sr-muted"
          >{{ t("panel.noData") }}</span>
          <span
              v-else
              class="sr-state-label"
          >
            <SRStatusDot :state="state(data[panel.id])" />
            {{ t(`status.${state(data[panel.id])}`) }}
          </span>
        </td>
      </tr>
    </tbody>
  </table>
</template>

<style scoped>
th {
  font-weight: 600;
}

.sr-description {
  font-weight: normal;
}

.sr-state {
  text-align: right;
  white-space: nowrap;
}

.sr-state-label {
  display: inline-flex;
  gap: 0.5rem;
  align-items: center;
}

.sr-description-column {
  display: none;
}

@media (width >= 1024px) {
  .sr-description {
    display: none !important;
  }

  .sr-description-column {
    display: table-cell;
  }
}
</style>
