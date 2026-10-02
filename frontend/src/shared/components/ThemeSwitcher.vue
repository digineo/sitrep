<script setup lang="ts">
import { Monitor, Moon, Sun } from "@lucide/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import { type Theme, useTheme } from "../composables/useTheme"
import SRDropdown from "./SRDropdown.vue"

defineProps<{
  compact?: boolean
  up?:      boolean
  right?:   boolean
}>()

const { t } = useI18n()
const { theme, setTheme } = useTheme()

const items = computed(() => [
  {
    value: "light" as Theme,
    label: t("theme.light"),
    icon:  Sun,
  },
  {
    value: "dark" as Theme,
    label: t("theme.dark"),
    icon:  Moon,
  },
  {
    value: "system" as Theme,
    label: t("theme.system"),
    icon:  Monitor,
  },
])
const label = computed(() => t(
  "theme.current",
  { name: items.value.find(i => i.value === theme.value)!.label },
))
</script>

<template>
  <SRDropdown
      :model-value="theme"
      :items
      :label
      :compact
      :up
      :right
      @update:model-value="setTheme"
  />
</template>
