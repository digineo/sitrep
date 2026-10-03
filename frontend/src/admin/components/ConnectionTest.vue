<script setup lang="ts">
import { PlugZap } from "@lucide/vue"
import { ref } from "vue"
import { useI18n } from "vue-i18n"

import SRButton from "../../shared/components/SRButton.vue"
import { api, errorCode } from "../api"

const props = defineProps<{
  /** path is the test endpoint. */
  path:  string
  /** body returns the configuration to test, if not the stored one. */
  body?: () => unknown
}>()

const { t } = useI18n()
const busy = ref(false)
const result = ref<{
  ok:   boolean
  text: string
} | null>(null)

async function run() {
  busy.value = true
  result.value = null
  try {
    const res = await api<{
      ok:      boolean
      detail?: string
      error?:  string
    }>("POST", props.path, props.body?.())
    result.value = res.ok
      ? {
        ok:   true,
        text: t("dataSources.testOk", { detail: res.detail ?? "" }),
      }
      : {
        ok:   false,
        text: t("dataSources.testFailed", { error: res.error ?? "" }),
      }
  } catch(err) {
    result.value = {
      ok:   false,
      text: t("dataSources.testFailed", { error: t(`error.${errorCode(err)}`) }),
    }
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <span class="sr-test">
    <SRButton
        :loading="busy"
        @click="run"
    >
      <span class="icon"><PlugZap aria-hidden="true" /></span>
      <span>{{ t("dataSources.test") }}</span>
    </SRButton>
    <span
        role="status"
        :class="result && (result.ok ? 'has-text-success' : 'has-text-danger')"
    >{{ result?.text }}</span>
  </span>
</template>

<style scoped>
.sr-test {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}
</style>
