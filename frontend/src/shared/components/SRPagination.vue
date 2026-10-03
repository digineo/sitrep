<script setup lang="ts">
import { useI18n } from "vue-i18n"
import type { RouteLocationRaw } from "vue-router"

defineProps<{
  page:  number
  pages: number
  /** to returns the location of a page. */
  to:    (page: number) => RouteLocationRaw
}>()

const { t } = useI18n()
</script>

<template>
  <nav
      v-if="pages > 1"
      class="sr-pagination"
      :aria-label="t('incidents.pagination')"
  >
    <RouterLink
        v-if="page > 1"
        :to="to(page - 1)"
        class="button"
    >
      {{ t("incidents.newer") }}
    </RouterLink>
    <span class="sr-pagination-page">{{ t("incidents.page", { page, pages }) }}</span>
    <RouterLink
        v-if="page < pages"
        :to="to(page + 1)"
        class="button"
    >
      {{ t("incidents.older") }}
    </RouterLink>
  </nav>
</template>

<style scoped>
.sr-pagination {
  display: flex;
  gap: 1rem;
  align-items: center;
  justify-content: center;
}
</style>
