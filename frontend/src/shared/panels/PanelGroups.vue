<script setup lang="ts">
import { computed } from "vue"
import type { RouteLocationRaw } from "vue-router"

import type { PanelData, PanelInfo, Span } from "../payload"
import PanelBoundary from "./PanelBoundary.vue"
import StatPanel from "./StatPanel.vue"
import StatusTable from "./StatusTable.vue"
import TimeseriesPanel from "./TimeseriesPanel.vue"

const props = defineProps<{
  panels:        PanelInfo[]
  data:          Record<string, PanelData>
  timezone:      string
  /** spans are the incidents to shade in charts. */
  spans?:        Span[]
  /** incidentLink returns the location a click on an incident's band opens. */
  incidentLink?: (id: string) => RouteLocationRaw
}>()

defineSlots<{
  /** actions renders controls next to a panel's title. */
  actions?: (props: { panel: PanelInfo }) => unknown
}>()

const byType = (type: PanelInfo["type"]) =>
  computed(() => props.panels.filter(p => p.type === type))
const status = byType("status")
const stats = byType("stat")
const charts = byType("timeseries")
</script>

<template>
  <PanelBoundary
      v-if="status.length"
      :reset-key="status.map(p => data[p.id])"
  >
    <StatusTable
        :panels="status"
        :data
        :timezone
        class="block"
    >
      <template #actions="{ panel }">
        <slot
            name="actions"
            :panel
        />
      </template>
    </StatusTable>
  </PanelBoundary>
  <div
      v-if="stats.length"
      class="sr-stat-grid block"
  >
    <PanelBoundary
        v-for="panel in stats"
        :key="panel.id"
        :reset-key="data[panel.id]"
    >
      <StatPanel
          :panel
          :data="data[panel.id]"
          :timezone
      >
        <template #actions>
          <slot
              name="actions"
              :panel
          />
        </template>
      </StatPanel>
    </PanelBoundary>
  </div>
  <div
      v-if="charts.length"
      class="sr-chart-grid block"
  >
    <PanelBoundary
        v-for="panel in charts"
        :key="panel.id"
        :reset-key="data[panel.id]"
    >
      <TimeseriesPanel
          :panel
          :data="data[panel.id]"
          :timezone
          :spans
          :incident-link
      >
        <template #actions>
          <slot
              name="actions"
              :panel
          />
        </template>
      </TimeseriesPanel>
    </PanelBoundary>
  </div>
</template>

<style scoped>
.sr-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 1rem;
}

.sr-chart-grid {
  display: grid;
  gap: 1rem;
}

@media (width >= 1024px) {
  .sr-chart-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
