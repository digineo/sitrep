<script setup lang="ts">
import { useId, useTemplateRef, watchEffect } from "vue"
import { useI18n } from "vue-i18n"

import SRField from "../../shared/components/SRField.vue"

defineProps<{ error?: string }>()
const model = defineModel<string>({ required: true })

const { t } = useI18n()
const listId = useId()
const input = useTemplateRef<HTMLInputElement>("input")
const zones = [
  "UTC",
  ...Intl.supportedValuesOf("timeZone").filter(z => z !== "UTC"),
]

watchEffect(() => input.value?.setCustomValidity(
  zones.includes(model.value) ? "" : t("error.invalid_timezone"),
))
</script>

<template>
  <SRField
      v-slot="{ id, describedby, invalid }"
      :label="t('site.timezone')"
      :help="t('site.timezoneHelp')"
      :error
  >
    <input
        :id
        ref="input"
        v-model="model"
        class="input"
        :list="listId"
        required
        autocomplete="off"
        spellcheck="false"
        :aria-describedby="describedby"
        :aria-invalid="invalid"
    >
    <datalist :id="listId">
      <option
          v-for="zone in zones"
          :key="zone"
          :value="zone"
      />
    </datalist>
  </SRField>
</template>
