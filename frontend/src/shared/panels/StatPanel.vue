<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import StaleTag from "../components/StaleTag.vue"
import { formatNumber } from "../format"
import type { PanelData, PanelInfo, StatData } from "../payload"

const props = defineProps<{
  panel:    PanelInfo
  data?:    PanelData
  timezone: string
}>()

const { t, locale } = useI18n()
const value = computed(
  () => props.data?.state === "pending"
    ? undefined
    : (props.data?.data as StatData | undefined)?.value,
)
</script>

<template>
  <div
      class="box sr-stat"
      :data-panel-id="panel.id"
  >
    <div class="sr-panel-head">
      <h3 class="title is-6 mb-0">
        {{ panel.title }}
      </h3>
      <StaleTag
          v-if="data?.state === 'stale'"
          :fetched-at="data.fetchedAt!"
          :timezone
      />
      <slot name="actions" />
    </div>
    <p
        v-if="panel.description"
        class="sr-muted"
    >
      {{ panel.description }}
    </p>
    <p
        v-if="value === undefined"
        class="sr-stat-value sr-muted"
    >
      {{ t("panel.noData") }}
    </p>
    <p
        v-else
        class="sr-stat-value"
    >
      <span class="sr-stat-number">{{ value === null ? "-" : formatNumber(value, locale, panel.decimals ?? 0) }}</span>
      <span
          v-if="panel.unit && value !== null"
          class="sr-muted"
      >{{ `\u00a0${panel.unit}` }}</span>
    </p>
  </div>
</template>

<style scoped>
.sr-stat {
  display: flex;
  flex-direction: column;
  height: 100%;
  margin: 0;
}

.sr-stat-value {
  margin-top: auto;
  padding-top: 0.75rem;
}

.sr-stat-number {
  font-size: 2rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
</style>
