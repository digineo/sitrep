<script setup lang="ts">
import { ArrowDown, ArrowUp } from "@lucide/vue"
import { computed, useId } from "vue"
import { useI18n } from "vue-i18n"

import SRField from "../../shared/components/SRField.vue"
import { endonym, supported } from "../../shared/i18n"
import type { Languages } from "../types"

defineProps<{
  /** errors maps "enabled" and "primary" to error codes. */
  errors?: Record<string, string | undefined>
}>()

const model = defineModel<Languages>({ required: true })

const { t } = useI18n()
const legendId = useId()

// Enabled languages come first, in their order.
const rows = computed(() => [
  ...model.value.enabled,
  ...supported.filter(lang => !model.value.enabled.includes(lang)),
])

function toggle(lang: string, on: boolean) {
  const enabled = rows.value.filter(
    l => l === lang ? on : model.value.enabled.includes(l),
  )
  const primary = enabled.includes(model.value.primary)
    ? model.value.primary
    : enabled[0] ?? ""
  model.value = {
    enabled,
    primary,
  }
}

function move(index: number, by: number) {
  const enabled = [...model.value.enabled]
  enabled.splice(index + by, 0, ...enabled.splice(index, 1))
  model.value = {
    ...model.value,
    enabled,
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
      {{ t("languages.enabled") }}
    </legend>
    <ul class="block">
      <li
          v-for="(lang, i) in rows"
          :key="lang"
          class="sr-language"
      >
        <label class="checkbox">
          <input
              type="checkbox"
              :checked="model.enabled.includes(lang)"
              :disabled="model.enabled.length === 1 && model.enabled[0] === lang"
              @change="toggle(lang, ($event.target as HTMLInputElement).checked)"
          >
          <span :lang="lang">{{ endonym(lang) }}</span>
        </label>
        <span
            v-if="model.enabled.includes(lang)"
            class="buttons are-small"
        >
          <button
              type="button"
              class="button"
              :disabled="i === 0"
              :aria-label="t('languages.moveUp', { language: endonym(lang) })"
              @click="move(i, -1)"
          >
            <span class="icon"><ArrowUp aria-hidden="true" /></span>
          </button>
          <button
              type="button"
              class="button"
              :disabled="i === model.enabled.length - 1"
              :aria-label="t('languages.moveDown', { language: endonym(lang) })"
              @click="move(i, 1)"
          >
            <span class="icon"><ArrowDown aria-hidden="true" /></span>
          </button>
        </span>
      </li>
    </ul>
    <p class="help">
      {{ t("languages.enabledHelp") }}
    </p>
    <p
        v-if="errors?.enabled"
        class="help is-danger"
    >
      {{ t(`error.${errors.enabled}`) }}
    </p>
  </fieldset>
  <SRField
      v-slot="{ id, describedby, invalid }"
      :label="t('languages.primary')"
      :help="t('languages.primaryHelp')"
      :error="errors?.primary && t(`error.${errors.primary}`)"
  >
    <div class="select">
      <select
          :id
          :value="model.primary"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
          @change="model = { ...model, primary: ($event.target as HTMLSelectElement).value }"
      >
        <option
            v-for="lang in model.enabled"
            :key="lang"
            :value="lang"
            :lang="lang"
        >
          {{ endonym(lang) }}
        </option>
      </select>
    </div>
  </SRField>
</template>

<style scoped>
.sr-language {
  display: flex;
  align-items: center;
  justify-content: space-between;
  max-width: 24rem;
  min-height: 2.5rem;
}

.sr-language .buttons {
  margin: 0;
}

.checkbox {
  display: inline-flex;
  gap: 0.5rem;
  align-items: center;
}
</style>
