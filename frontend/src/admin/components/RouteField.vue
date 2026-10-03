<script setup lang="ts">
import { computed, inject, useId, useTemplateRef, watchEffect } from "vue"
import { useI18n } from "vue-i18n"

import { type Bootstrap } from "../../shared/bootstrap"
import SRField from "../../shared/components/SRField.vue"
import { routeError, siteURL } from "../rules"
import type { Route } from "../types"

const props = defineProps<{
  /** errors maps "mode", "slug" and "domain" to server error codes. */
  errors?: Record<string, string | undefined>
}>()

const route = defineModel<Route>({ required: true })

const { t } = useI18n()
const bootstrap = inject<Bootstrap>("bootstrap")!
const legendId = useId()
const input = useTemplateRef<HTMLInputElement>("input")
const modes = ["path", "subdomain", "custom"] as const

const field = computed(() => route.value.mode === "custom" ? "domain" : "slug")
const clientError = computed(
  () => routeError(route.value, bootstrap.baseDomains ?? []),
)
const error = computed(() => {
  const code = props.errors?.[field.value]
    ?? (route.value[field.value] ? clientError.value?.code : undefined)
  return code && t(`error.${code}`)
})
const url = computed(() => clientError.value
  ? ""
  : siteURL(route.value, bootstrap.baseDomains ?? [], window.location))

// The browser refuses to submit an invalid route, like the server would.
watchEffect(() => input.value?.setCustomValidity(
  clientError.value ? t(`error.${clientError.value.code}`) : "",
))

function setMode(mode: Route["mode"]) {
  route.value = {
    ...route.value,
    mode,
  }
}
</script>

<template>
  <fieldset
      class="field"
      :aria-labelledby="legendId"
  >
    <legend
        :id="legendId"
        class="label"
    >
      {{ t("route.label") }}
    </legend>
    <div class="control sr-modes">
      <label
          v-for="mode in modes"
          :key="mode"
          class="radio"
      >
        <input
            type="radio"
            :checked="route.mode === mode"
            :value="mode"
            @change="setMode(mode)"
        >
        {{ t(`route.${mode}`) }}
      </label>
    </div>
  </fieldset>
  <SRField
      v-slot="{ id, describedby, invalid }"
      :label="t(`route.${field}`)"
      :help="url ? t('route.hint', { url }) : t(`route.${field}Help`)"
      :error
  >
    <input
        :id
        ref="input"
        class="input"
        :value="route[field] ?? ''"
        required
        :maxlength="field === 'domain' ? 253 : 63"
        autocapitalize="off"
        spellcheck="false"
        :aria-describedby="describedby"
        :aria-invalid="invalid"
        @input="route = { ...route, [field]: ($event.target as HTMLInputElement).value.trim() }"
    >
  </SRField>
</template>

<style scoped>
.sr-modes {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
</style>
