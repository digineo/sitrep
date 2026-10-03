<script setup lang="ts">
import { useI18n } from "vue-i18n"

import { useNotices } from "../stores/notices"

const { t } = useI18n()
const notices = useNotices()
</script>

<template>
  <div
      class="sr-toasts"
      aria-live="polite"
  >
    <div
        v-for="toast in notices.toasts"
        :key="toast.id"
        class="notification"
        :class="toast.kind === 'success' ? 'is-success' : 'is-danger'"
    >
      <button
          type="button"
          class="delete"
          :aria-label="t('common.close')"
          @click="notices.close(toast.id)"
      />
      <template v-if="toast.kind === 'success'">
        {{ toast.text }}
      </template>
      <template v-else>
        <strong>{{ toast.text }}:</strong> {{ t(`error.${toast.code}`) }}
        <p
            v-if="toast.detail"
            class="sr-detail"
        >
          {{ toast.detail }}
        </p>
      </template>
    </div>
  </div>
</template>

<style scoped>
.sr-toasts {
  position: fixed;
  right: 1rem;
  bottom: 1rem;
  z-index: 40;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  width: min(24rem, calc(100vw - 2rem));
}

.notification {
  margin: 0;
}

.sr-detail {
  margin-top: 0.25rem;
  white-space: pre-line;
  overflow-wrap: anywhere;
}
</style>
