<script setup lang="ts">
import { onErrorCaptured, ref, watch } from "vue"
import { useI18n } from "vue-i18n"

const props = defineProps<{
  /** resetKey changes with the panel's data; a change tries again. */
  resetKey: unknown
}>()

const { t } = useI18n()
const failed = ref(false)

// A panel that fails to render shows a warning instead of breaking the page.
onErrorCaptured(() => {
  failed.value = true
  return false
})
watch(() => props.resetKey, () => (failed.value = false))
</script>

<template>
  <div
      v-if="failed"
      class="notification is-warning"
  >
    {{ t("panel.renderFailed") }}
  </div>
  <slot v-else />
</template>
