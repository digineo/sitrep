<script setup lang="ts">
import { useI18n } from "vue-i18n"

import SRField from "../../shared/components/SRField.vue"
import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import { localizedErrors } from "../api"
import type { Languages, LegalPage } from "../types"
import MarkdownEditor from "./MarkdownEditor.vue"

const props = defineProps<{
  /** label names the page, e.g. "Imprint". */
  label:     string
  languages: Languages
  /** inherit offers the instance's page, for sites. */
  inherit?:  boolean
  /**
   * errors maps field paths to error codes; path is the page's prefix, e.g.
   * "legal.imprint".
   */
  errors:    Record<string, string>
  path:      string
}>()

/**
 * The model keeps the values of other modes, so that switching back restores
 * them until saved.
 */
const page = defineModel<LegalPage>({ required: true })

const { t } = useI18n()
const modes = props.inherit
  ? ["inherit", "none", "url", "text"] as const
  : ["none", "url", "text"] as const
const modeError = () => props.errors[`${props.path}.mode`]
</script>

<template>
  <SRField
      v-slot="{ id, describedby, invalid }"
      :label
      :error="modeError() && t(`error.${modeError()}`)"
  >
    <div class="select">
      <select
          :id
          :value="page.mode"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
          @change="page = { ...page, mode: ($event.target as HTMLSelectElement).value as LegalPage['mode'] }"
      >
        <option
            v-for="mode in modes"
            :key="mode"
            :value="mode"
        >
          {{ t(`legalPage.${mode}`) }}
        </option>
      </select>
    </div>
  </SRField>
  <SRLocalizedInput
      v-if="page.mode === 'url'"
      v-slot="{ value, update, attrs }"
      :model-value="page.url"
      :label="t('legalPage.urlOf', { page: label })"
      :languages
      required
      :maxlength="2000"
      :errors="localizedErrors(errors, `${path}.url`, t)"
      @update:model-value="page = { ...page, url: $event }"
  >
    <input
        v-bind="attrs"
        class="input"
        type="url"
        placeholder="https://"
        :value
        @input="update(($event.target as HTMLInputElement).value)"
    >
  </SRLocalizedInput>
  <SRLocalizedInput
      v-if="page.mode === 'text'"
      v-slot="{ value, update, attrs }"
      :model-value="page.text"
      :label="t('legalPage.textOf', { page: label })"
      :languages
      required
      :maxlength="50000"
      :errors="localizedErrors(errors, `${path}.text`, t)"
      @update:model-value="page = { ...page, text: $event }"
  >
    <MarkdownEditor
        :model-value="value"
        v-bind="attrs"
        @update:model-value="update"
    />
  </SRLocalizedInput>
</template>
