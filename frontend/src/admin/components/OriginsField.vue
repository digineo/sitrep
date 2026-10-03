<script setup lang="ts">
import { Minus, Plus } from "@lucide/vue"
import { nextTick, useId } from "vue"
import { useI18n } from "vue-i18n"

import { originError } from "../rules"

defineProps<{
  /** error is a server error code for the list. */
  error?: string
}>()

/**
 * The model lists one origin per entry; empty entries are allowed while
 * editing.
 */
const origins = defineModel<string[]>({ required: true })

const { t } = useI18n()
const id = useId()

const entryError = (origin: string) => origin.trim() ? originError(origin) : null

function set(i: number, value: string, input: HTMLInputElement) {
  origins.value = origins.value.map((o, j) => j === i ? value : o)
  const code = entryError(value)
  input.setCustomValidity(code ? t(`error.${code}`) : "")
}

async function add() {
  origins.value = [...origins.value, ""]
  await nextTick()
  document.getElementById(`${id}-${origins.value.length - 1}`)?.focus()
}

function remove(i: number) {
  origins.value = origins.value.filter((_, j) => j !== i)
}
</script>

<template>
  <fieldset class="field">
    <legend class="label">
      {{ t("site.origins") }}
    </legend>
    <p
        :id="`${id}-help`"
        class="help mt-0 mb-2"
    >
      {{ t("site.originsHelp") }}
    </p>
    <div
        v-for="(origin, i) in origins"
        :key="i"
        class="field"
    >
      <div class="field has-addons mb-0">
        <div class="control is-expanded">
          <input
              :id="`${id}-${i}`"
              class="input"
              type="url"
              :value="origin"
              placeholder="https://www.example.com"
              maxlength="2000"
              :aria-label="t('site.origin', { n: i + 1 })"
              :aria-describedby="entryError(origin) ? `${id}-error-${i}` : `${id}-help`"
              :aria-invalid="entryError(origin) ? true : undefined"
              @input="set(i, ($event.target as HTMLInputElement).value, $event.target as HTMLInputElement)"
          >
        </div>
        <div class="control">
          <button
              type="button"
              class="button"
              :aria-label="t('site.originRemove', { n: i + 1 })"
              @click="remove(i)"
          >
            <span class="icon"><Minus aria-hidden="true" /></span>
          </button>
        </div>
      </div>
      <p
          v-if="entryError(origin)"
          :id="`${id}-error-${i}`"
          class="help is-danger"
      >
        {{ t(`error.${entryError(origin)}`) }}
      </p>
    </div>
    <button
        type="button"
        class="button is-small"
        :aria-label="t('site.originAdd')"
        @click="add"
    >
      <span class="icon"><Plus aria-hidden="true" /></span>
    </button>
    <p
        v-if="error"
        class="help is-danger"
    >
      {{ t(`error.${error}`) }}
    </p>
  </fieldset>
</template>
