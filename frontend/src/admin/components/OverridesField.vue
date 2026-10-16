<script setup lang="ts">
import { ArrowDown, ArrowUp, Plus, Trash2 } from "@lucide/vue"
import { useId } from "vue"
import { useI18n } from "vue-i18n"

import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import { localizedErrors } from "../api"
import type { Override, Text } from "../types"
import ColorPicker from "./ColorPicker.vue"

const props = defineProps<{
  /** errors maps paths like "overrides[1].label" to error codes. */
  errors:    Record<string, string>
  languages: {
    enabled: string[]
    primary: string
  }
  /** unit and decimals are the panel's, the defaults of a new override. */
  unit?:    Text
  decimals: number
}>()

const model = defineModel<Override[]>({ required: true })

const { t } = useI18n()
const id = useId()

function set(i: number, change: Partial<Override>) {
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

function add() {
  model.value = [...model.value, {
    label:    "",
    value:    "",
    unit:     props.unit,
    decimals: props.decimals,
    axis:     "left",
  }]
}

const invalid = (i: number, field: string) =>
  props.errors[`overrides[${i}].${field}`] ? true : undefined
</script>

<template>
  <fieldset
      class="field"
      :aria-labelledby="`${id}-legend`"
      :aria-describedby="`${id}-help`"
  >
    <legend
        :id="`${id}-legend`"
        class="label"
    >
      {{ t("panelEditor.overrides") }}
    </legend>
    <fieldset
        v-for="(row, i) in model"
        :key="i"
        class="box sr-override"
    >
      <legend class="sr-visually-hidden">
        {{ t("panelEditor.override", { n: i + 1 }) }}
      </legend>
      <div class="sr-override-row">
        <div>
          <label
              class="label is-small"
              :for="`${id}-${i}-label`"
          >{{ t("panelEditor.overrideLabel") }}</label>
          <input
              :id="`${id}-${i}-label`"
              class="input is-small"
              required
              placeholder="instance"
              :value="row.label"
              :aria-invalid="invalid(i, 'label')"
              @input="set(i, { label: ($event.target as HTMLInputElement).value })"
          >
        </div>
        <div>
          <label
              class="label is-small"
              :for="`${id}-${i}-value`"
          >{{ t("panelEditor.overrideValue") }}</label>
          <input
              :id="`${id}-${i}-value`"
              class="input is-small"
              :value="row.value"
              :aria-invalid="invalid(i, 'value')"
              @input="set(i, { value: ($event.target as HTMLInputElement).value })"
          >
        </div>
        <span class="buttons are-small mb-0 ml-auto">
          <button
              type="button"
              class="button"
              :disabled="i === 0"
              :aria-label="t('panelEditor.overrideUp', { n: i + 1 })"
              @click="move(i, -1)"
          >
            <span class="icon"><ArrowUp aria-hidden="true" /></span>
          </button>
          <button
              type="button"
              class="button"
              :disabled="i === model.length - 1"
              :aria-label="t('panelEditor.overrideDown', { n: i + 1 })"
              @click="move(i, 1)"
          >
            <span class="icon"><ArrowDown aria-hidden="true" /></span>
          </button>
          <button
              type="button"
              class="button"
              :aria-label="t('panelEditor.overrideRemove', { n: i + 1 })"
              @click="model = model.filter((_, j) => j !== i)"
          >
            <span class="icon"><Trash2 aria-hidden="true" /></span>
          </button>
        </span>
      </div>
      <div class="sr-override-row">
        <SRLocalizedInput
            class="sr-unit"
            :model-value="row.unit"
            :label="t('panelEditor.unit')"
            :languages
            small
            :maxlength="32"
            :errors="localizedErrors(errors, `overrides[${i}].unit`, t)"
            @update:model-value="set(i, { unit: $event })"
        />
        <div>
          <label
              class="label is-small"
              :for="`${id}-${i}-decimals`"
          >{{ t("panelEditor.decimals") }}</label>
          <input
              :id="`${id}-${i}-decimals`"
              class="input is-small sr-decimals"
              type="number"
              min="0"
              max="6"
              step="1"
              required
              :value="row.decimals"
              :aria-invalid="invalid(i, 'decimals')"
              @input="set(i, { decimals: ($event.target as HTMLInputElement).valueAsNumber })"
          >
        </div>
        <div>
          <label
              class="label is-small"
              :for="`${id}-${i}-axis`"
          >{{ t("panelEditor.overrideAxis") }}</label>
          <div class="select is-small">
            <select
                :id="`${id}-${i}-axis`"
                :value="row.axis"
                @change="set(i, { axis: ($event.target as HTMLSelectElement).value as Override['axis'] })"
            >
              <option value="left">
                {{ t("panelEditor.axes.left") }}
              </option>
              <option value="right">
                {{ t("panelEditor.axes.right") }}
              </option>
            </select>
          </div>
        </div>
        <div>
          <label
              class="label is-small"
              :for="`${id}-${i}-color`"
          >{{ t("panelEditor.overrideColor") }}</label>
          <ColorPicker
              :id="`${id}-${i}-color`"
              :model-value="row.color"
              :label="t('panelEditor.overrideColor')"
              @update:model-value="set(i, { color: $event })"
          />
        </div>
      </div>
    </fieldset>
    <button
        v-if="model.length < 20"
        type="button"
        class="button is-small"
        @click="add"
    >
      <span class="icon"><Plus aria-hidden="true" /></span>
      <span>{{ t("panelEditor.overrideAdd") }}</span>
    </button>
    <p
        :id="`${id}-help`"
        class="help"
    >
      {{ t("panelEditor.overridesHelp") }}
    </p>
  </fieldset>
</template>

<style scoped>
.sr-override {
  margin-bottom: 0.5rem;
  padding: 0.75rem;
}

.sr-override-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: end;
}

.sr-override-row + .sr-override-row {
  margin-top: 0.5rem;
}

.sr-override-row .sr-unit {
  width: 14rem;
  margin-bottom: 0;
}

.sr-override-row .input {
  width: 10rem;
}

.sr-override-row .sr-decimals {
  width: 5rem;
}
</style>
