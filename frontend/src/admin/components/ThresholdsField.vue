<script setup lang="ts">
import { ArrowDown, ArrowUp, Plus, Trash2 } from "@lucide/vue"
import { useId } from "vue"
import { useI18n } from "vue-i18n"

import type { Threshold } from "../types"

defineProps<{
  /** errors maps paths like "thresholds[1].op" to error codes. */
  errors: Record<string, string | undefined>
}>()

const model = defineModel<Threshold[]>({ required: true })

const { t } = useI18n()
const legendId = useId()
const helpId = useId()
const ops = ["<", "<=", ">", ">=", "==", "!="] as const
const states = ["operational", "degraded", "down"] as const

function set(i: number, change: Partial<Threshold>) {
  model.value = model.value.map((row, j) => j === i
    ? {
      ...row,
      ...change,
    }
    : row)
}

function move(i: number, by: number) {
  const rows = [...model.value]
  rows.splice(i + by, 0, ...rows.splice(i, 1))
  model.value = rows
}
</script>

<template>
  <fieldset
      class="field"
      :aria-labelledby="legendId"
      :aria-describedby="helpId"
  >
    <legend
        :id="legendId"
        class="label"
    >
      {{ t("panelEditor.thresholds") }}
    </legend>
    <div
        v-for="(row, i) in model"
        :key="i"
        class="sr-threshold"
    >
      <div class="select is-small">
        <select
            :value="row.op"
            :aria-label="t('panelEditor.thresholdOp', { n: i + 1 })"
            @change="set(i, { op: ($event.target as HTMLSelectElement).value as Threshold['op'] })"
        >
          <option
              v-for="op in ops"
              :key="op"
              :value="op"
          >
            {{ op }}
          </option>
        </select>
      </div>
      <input
          class="input is-small"
          type="number"
          step="any"
          required
          :value="row.value"
          :aria-label="t('panelEditor.thresholdValue', { n: i + 1 })"
          :aria-invalid="errors[`thresholds[${i}].value`] ? true : undefined"
          @input="set(i, { value: ($event.target as HTMLInputElement).valueAsNumber })"
      >
      <div class="select is-small">
        <select
            :value="row.state"
            :aria-label="t('panelEditor.thresholdState', { n: i + 1 })"
            @change="set(i, { state: ($event.target as HTMLSelectElement).value as Threshold['state'] })"
        >
          <option
              v-for="state in states"
              :key="state"
              :value="state"
          >
            {{ t(`status.${state}`) }}
          </option>
        </select>
      </div>
      <span class="buttons are-small mb-0">
        <button
            type="button"
            class="button"
            :disabled="i === 0"
            :aria-label="t('panelEditor.thresholdUp', { n: i + 1 })"
            @click="move(i, -1)"
        >
          <span class="icon"><ArrowUp aria-hidden="true" /></span>
        </button>
        <button
            type="button"
            class="button"
            :disabled="i === model.length - 1"
            :aria-label="t('panelEditor.thresholdDown', { n: i + 1 })"
            @click="move(i, 1)"
        >
          <span class="icon"><ArrowDown aria-hidden="true" /></span>
        </button>
        <button
            type="button"
            class="button"
            :aria-label="t('panelEditor.thresholdRemove', { n: i + 1 })"
            @click="model = model.filter((_, j) => j !== i)"
        >
          <span class="icon"><Trash2 aria-hidden="true" /></span>
        </button>
      </span>
    </div>
    <button
        v-if="model.length < 20"
        type="button"
        class="button is-small"
        @click="model = [...model, { op: '<', value: 1, state: 'down' }]"
    >
      <span class="icon"><Plus aria-hidden="true" /></span>
      <span>{{ t("panelEditor.thresholdAdd") }}</span>
    </button>
    <p
        :id="helpId"
        class="help"
    >
      {{ t("panelEditor.thresholdsHelp") }}
    </p>
  </fieldset>
</template>

<style scoped>
.sr-threshold {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
  margin-bottom: 0.5rem;
}

.sr-threshold .input {
  width: 8rem;
}
</style>
