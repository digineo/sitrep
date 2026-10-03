<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import type { State } from "../payload"
import { banners } from "../status"

const props = defineProps<{ status: State }>()

const { t } = useI18n()
const banner = computed(() => banners[props.status])
</script>

<template>
  <div
      class="notification sr-banner"
      :class="banner.color"
      role="status"
  >
    <span class="icon"><component
        :is="banner.icon"
        aria-hidden="true"
    /></span>
    <span>{{ t(`banner.${status}`) }}</span>
  </div>
</template>

<style scoped>
.sr-banner {
  display: flex;
  gap: 0.75rem;
  align-items: center;
  font-size: 1.125rem;
  font-weight: 600;
}
</style>
