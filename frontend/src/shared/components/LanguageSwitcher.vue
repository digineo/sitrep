<script setup lang="ts">
import { Languages } from "@lucide/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import { endonym } from "../i18n"
import SRDropdown from "./SRDropdown.vue"

const props = defineProps<{
  languages: string[]
  compact?:  boolean
  up?:       boolean
  right?:    boolean
}>()

const model = defineModel<string>({ required: true })

const { t } = useI18n()
const items = computed(() => props.languages.map(lang => ({
  value: lang,
  label: endonym(lang),
  icon:  Languages,
  lang,
})))
</script>

<template>
  <SRDropdown
      v-model="model"
      :items
      :label="t('language.current', { name: endonym(model) })"
      :compact
      :up
      :right
  />
</template>
