<script setup lang="ts">
import { useI18n } from "vue-i18n"

import SRField from "../../shared/components/SRField.vue"
import type { DataSourceType, Field } from "../types"

/** SecretState says what happens to a secret on save. */
export interface SecretState {
  action: "keep" | "replace" | "clear"
  value:  string
}

const props = defineProps<{
  type:   DataSourceType
  /** stored lists the secrets that have a stored value. */
  stored: string[]
  /** errors maps field names to error codes. */
  errors: Record<string, string | undefined>
}>()

const values = defineModel<Record<string, string>>("values", { required: true })
const secrets = defineModel<Record<string, SecretState>>(
  "secrets",
  { required: true },
)

const { t, te } = useI18n()

const key = (field: Field, suffix: string) =>
  `datasource.${props.type.id}.${field.name}.${suffix}`
const visible = (field: Field) =>
  !field.when || values.value[field.when.field] === field.when.value
const help = (field: Field) =>
  te(key(field, "help")) ? t(key(field, "help")) : undefined

function setSecret(name: string, state: SecretState) {
  secrets.value = {
    ...secrets.value,
    [name]: state,
  }
}
</script>

<template>
  <template
      v-for="field in type.fields"
      :key="field.name"
  >
    <SRField
        v-if="visible(field)"
        v-slot="{ id, describedby, invalid }"
        :label="t(key(field, 'label'))"
        :help="help(field)"
        :error="errors[field.name] && t(`error.${errors[field.name]}`)"
    >
      <div
          v-if="field.kind === 'select'"
          class="select"
      >
        <select
            :id
            v-model="values[field.name]"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
          <option
              v-for="option in field.options"
              :key="option"
              :value="option"
          >
            {{ t(key(field, `options.${option}`)) }}
          </option>
        </select>
      </div>
      <label
          v-else-if="field.kind === 'bool'"
          class="checkbox"
      >
        <input
            :id
            type="checkbox"
            :checked="values[field.name] === 'true'"
            :aria-describedby="describedby"
            @change="values = { ...values, [field.name]: String(($event.target as HTMLInputElement).checked) }"
        >
      </label>
      <template v-else-if="field.kind === 'secret'">
        <div
            v-if="stored.includes(field.name) && secrets[field.name]?.action !== 'replace'"
            class="sr-secret"
        >
          <span
              :id
              class="sr-muted"
              tabindex="-1"
          >{{ secrets[field.name]?.action === "clear" ? t("dataSources.secretCleared") : t("dataSources.secretSet") }}</span>
          <span class="buttons are-small mb-0">
            <button
                type="button"
                class="button"
                @click="setSecret(field.name, { action: 'replace', value: '' })"
            >
              {{ t("dataSources.replace") }}
            </button>
            <button
                v-if="secrets[field.name]?.action !== 'clear'"
                type="button"
                class="button"
                @click="setSecret(field.name, { action: 'clear', value: '' })"
            >
              {{ t("dataSources.clear") }}
            </button>
            <button
                v-else
                type="button"
                class="button"
                @click="setSecret(field.name, { action: 'keep', value: '' })"
            >
              {{ t("dataSources.keep") }}
            </button>
          </span>
        </div>
        <input
            v-else
            :id
            class="input"
            type="password"
            autocomplete="new-password"
            :value="secrets[field.name]?.value ?? ''"
            :required="field.required && !stored.includes(field.name)"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
            @input="setSecret(field.name, { action: 'replace', value: ($event.target as HTMLInputElement).value })"
        >
      </template>
      <input
          v-else
          :id
          v-model="values[field.name]"
          class="input"
          :type="field.kind === 'url' ? 'url' : 'text'"
          :placeholder="field.default"
          :required="field.required"
          maxlength="2000"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
      >
    </SRField>
  </template>
</template>

<style scoped>
.sr-secret {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}
</style>
