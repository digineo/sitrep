<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"
import type { RouteLocationRaw } from "vue-router"

import { formatDateTime } from "../format"
import { excerpt } from "../incidents"
import type { Incident } from "../payload"
import IncidentTags from "./IncidentTags.vue"

const props = defineProps<{
  incident: Pick<
    Incident,
    "title" | "status" | "severity" | "lastActivity" | "updates"
  >
  to:       RouteLocationRaw
  timezone: string
}>()

defineSlots<{
  /** default adds a line below the excerpt. */
  default?: () => unknown
}>()

const { locale } = useI18n()
const text = computed(() => excerpt(props.incident))
</script>

<template>
  <RouterLink
      :to
      class="box sr-incident"
  >
    <div class="sr-incident-head">
      <span>
        <strong>{{ incident.title }}</strong>
        <span
            class="sr-incident-sep"
            aria-hidden="true"
        > · </span>
        <time
            class="sr-incident-time"
            :datetime="incident.lastActivity"
        >{{ formatDateTime(new Date(incident.lastActivity), locale, timezone) }}</time>
      </span>
      <IncidentTags
          :status="incident.status"
          :severity="incident.severity"
      />
    </div>
    <p
        v-if="text"
        class="sr-muted mt-1"
    >
      {{ text }}
    </p>
    <slot />
  </RouterLink>
</template>

<style scoped>
.sr-incident {
  display: block;
  color: inherit;
}

.sr-incident-head {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: flex-start;
  justify-content: space-between;
}

.sr-incident-time {
  color: var(--bulma-text-weak);
}

@media (width < 769px) {
  .sr-incident-sep {
    display: none;
  }

  .sr-incident-time {
    display: block;
  }
}
</style>
