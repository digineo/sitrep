<script setup lang="ts">
import { Check } from "@lucide/vue"
import { computed, shallowRef, useId, watchEffect } from "vue"
import { useI18n } from "vue-i18n"

import SRField from "../../shared/components/SRField.vue"
import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import { toZonedInput } from "../../shared/format"
import { statusColors } from "../../shared/incidents"
import type { IncidentStatus, Severity } from "../../shared/payload"
import { localizedErrors } from "../api"
import type { UpdateDraft } from "../incidents"
import type { Languages } from "../types"
import MarkdownEditor from "./MarkdownEditor.vue"

const props = defineProps<{
  /** opening is set for the first update, which opens the incident. */
  opening:   boolean
  /** edit is set for a stored update: an empty time leaves it unchanged. */
  edit?:     boolean
  timezone:  string
  languages: Languages
  /** errors maps field paths to error codes. */
  errors:    Record<string, string>
}>()

const draft = defineModel<UpdateDraft>({ required: true })

const { t } = useI18n()
const id = useId()
/** unchangedStatus is the radio button that keeps the status. */
const unchangedStatus = shallowRef<HTMLInputElement | null>(null)

const statuses = computed<(IncidentStatus | "")[]>(() => props.opening
  ? ["planned", "active"]
  : ["", "planned", "active", "investigating", "monitoring", "resolved"])
const severities: (Severity | "")[] = ["", "minor", "major", "critical"]
const severityColor = {
  "":         undefined,
  "minor":    undefined,
  "major":    "warning",
  "critical": "danger",
} as const

/** choiceClass styles a choice as a tag: solid while chosen, light otherwise. */
function choiceClass(color: string | undefined, chosen: boolean): string {
  if (!color) {
    return chosen ? "is-dark" : ""
  }
  return chosen ? `is-${color}` : `is-${color} is-light`
}

const statusLabel = (s: IncidentStatus | "") =>
  s ? t(`incidents.status.${s}`) : t("updateForm.unchanged")
const severityLabel = (s: Severity | "") => s
  ? t(`incidents.severity.${s}`)
  : props.opening ? t("updateForm.none") : t("updateForm.unchanged")

const descriptionHelp = computed(() => {
  if (draft.value.status === "planned") {
    return t("updateForm.plannedHelp")
  }
  return draft.value.status ? undefined : t("updateForm.optionalHelp")
})

// A later update that changes neither status nor severity is refused by
// the browser's own validation, at the status choice.
watchEffect(() => {
  unchangedStatus.value?.setCustomValidity(
    !draft.value.status && !draft.value.severity
      ? t("updateForm.nothingChanged")
      : "",
  )
})

function set(patch: Partial<UpdateDraft>) {
  draft.value = {
    ...draft.value,
    ...patch,
  }
}

function ago(hours: number) {
  const time = new Date(Date.now() - hours * 3_600_000)
  set({ at: toZonedInput(time, props.timezone) })
}
</script>

<template>
  <SRField
      v-slot="{ id: inputId, describedby, invalid }"
      :label="t('updateForm.time', { zone: timezone })"
      :help="edit ? t('updateForm.timeHelpEdit') : t('updateForm.timeHelpNew')"
      :error="errors.at && t(`error.${errors.at}`)"
  >
    <div class="sr-time">
      <input
          :id="inputId"
          class="input"
          type="datetime-local"
          step="60"
          :value="draft.at"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
          @input="set({ at: ($event.target as HTMLInputElement).value })"
      >
      <span class="buttons are-small mb-0">
        <button
            type="button"
            class="button"
            @click="ago(0)"
        >{{ t("updateForm.now") }}</button>
        <button
            type="button"
            class="button"
            @click="ago(1)"
        >{{ t("updateForm.hourAgo") }}</button>
        <button
            type="button"
            class="button"
            @click="ago(2)"
        >{{ t("updateForm.twoHoursAgo") }}</button>
      </span>
    </div>
  </SRField>
  <fieldset class="field">
    <legend class="label">
      {{ t("updateForm.status") }}
    </legend>
    <div class="sr-choices">
      <label
          v-for="status in statuses"
          :key="status"
          class="sr-choice"
      >
        <input
            :ref="(el) => { if (!status) unchangedStatus = el as HTMLInputElement | null }"
            class="sr-visually-hidden"
            type="radio"
            :name="`${id}-status`"
            :value="status"
            :checked="draft.status === status"
            :required="opening"
            @change="set({ status })"
        >
        <span
            class="tag"
            :class="choiceClass(status ? statusColors[status] : undefined, draft.status === status)"
        >
          <Check
              v-if="draft.status === status"
              class="sr-choice-check"
              aria-hidden="true"
          />
          {{ statusLabel(status) }}
        </span>
      </label>
    </div>
    <p
        v-if="errors.status"
        class="help is-danger"
    >
      {{ t(`error.${errors.status}`) }}
    </p>
  </fieldset>
  <fieldset class="field">
    <legend class="label">
      {{ t("updateForm.severity") }}
    </legend>
    <div class="sr-choices">
      <label
          v-for="severity in severities"
          :key="severity"
          class="sr-choice"
      >
        <input
            class="sr-visually-hidden"
            type="radio"
            :name="`${id}-severity`"
            :value="severity"
            :checked="draft.severity === severity"
            @change="set({ severity })"
        >
        <span
            class="tag"
            :class="choiceClass(severityColor[severity], draft.severity === severity)"
        >
          <Check
              v-if="draft.severity === severity"
              class="sr-choice-check"
              aria-hidden="true"
          />
          {{ severityLabel(severity) }}
        </span>
      </label>
    </div>
  </fieldset>
  <SRLocalizedInput
      v-slot="{ value, update, attrs }"
      :model-value="draft.description"
      :label="t('updateForm.description')"
      :languages
      :required="!!draft.status"
      :maxlength="50000"
      :help="descriptionHelp"
      :errors="localizedErrors(errors, 'description', t)"
      @update:model-value="set({ description: $event ?? {} })"
  >
    <MarkdownEditor
        :model-value="value"
        v-bind="attrs"
        @update:model-value="update"
    />
  </SRLocalizedInput>
</template>

<style scoped>
.sr-time {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  align-items: center;
}

.sr-time .input {
  width: auto;
}

.sr-choices {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.sr-choice {
  position: relative;
  cursor: pointer;
}

.sr-choice .tag {
  gap: 0.25rem;
  font-size: 0.875rem;
}

.sr-choice input:checked + .tag {
  font-weight: 600;
}

.sr-choice input:focus-visible + .tag {
  outline: 2px solid #2f6feb;
  outline-offset: 2px;
}

.sr-choice-check {
  width: 1em;
  height: 1em;
}
</style>
