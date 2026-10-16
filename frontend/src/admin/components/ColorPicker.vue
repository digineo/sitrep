<script setup lang="ts">
import { ChevronDown } from "@lucide/vue"
import { computed, nextTick, ref, useId, useTemplateRef } from "vue"
import { useI18n } from "vue-i18n"

defineProps<{
  /** id is the trigger's, for a label. */
  id?:   string
  /** label is the trigger's accessible name, before the chosen color. */
  label: string
}>()

/** The model is "#rrggbb", or undefined for the palette's color. */
const model = defineModel<string | undefined>({ required: true })

/**
 * hues are the chart palette's hues in a dark, a base and a light shade.
 * Base shades have at least 3:1 contrast on light and dark panels; dark and
 * light ones only 1.5:1 on the panels of the other color scheme.
 */
const hues = {
  blue:    ["#00308d", "#3879f6", "#b6d0ff"],
  amber:   ["#4c3401", "#a67706", "#ffc65d"],
  green:   ["#024228", "#1e9360", "#7be4ad"],
  magenta: ["#6e0056", "#ca4da6", "#ffbbe7"],
  cyan:    ["#013f48", "#058da0", "#69dff5"],
  violet:  ["#451e8b", "#886cdd", "#d2c9fe"],
}
const shades = ["dark", "base", "light"] as const

/** swatches lists the colors row by row, lightest first, a column per hue. */
const swatches = [...shades].reverse().flatMap(shade => Object.entries(hues)
  .map(([hue, colors]) => ({
    hue,
    shade,
    color: colors[shades.indexOf(shade)]!,
  })))

const { t } = useI18n()
const panelId = useId()
const name = useId()
const open = ref(false)
const root = useTemplateRef<HTMLElement>("root")
const trigger = useTemplateRef<HTMLButtonElement>("trigger")

/** swatchName names a swatch, e.g. "Blue, light". */
const swatchName = (s: typeof swatches[number]) =>
  t(`colorPicker.shades.${s.shade}`, { hue: t(`colorPicker.hues.${s.hue}`) })

const swatch = computed(() => swatches.find(s => s.color === model.value))
const custom = computed(() => !!model.value && !swatch.value)
const current = computed(() => {
  if (swatch.value) {
    return swatchName(swatch.value)
  }
  return model.value
    ? t("colorPicker.customValue", { color: model.value })
    : t("colorPicker.auto")
})

async function show() {
  open.value = true
  await nextTick()
  root.value?.querySelector<HTMLInputElement>("input[type=radio]:checked")?.focus()
}

function close(refocus: boolean) {
  open.value = false
  if (refocus) {
    trigger.value?.focus()
  }
}

function onFocusOut(event: FocusEvent) {
  // The browser's color dialog takes the window's focus; the picker stays.
  if (document.hasFocus() && !root.value?.contains(event.relatedTarget as Node | null)) {
    close(false)
  }
}
</script>

<template>
  <div
      ref="root"
      class="dropdown"
      :class="{ 'is-active': open }"
      @focusout="onFocusOut"
      @keydown.esc.prevent="close(true)"
  >
    <div class="dropdown-trigger">
      <button
          :id
          ref="trigger"
          type="button"
          class="button is-small"
          :aria-label="`${label}: ${current}`"
          :aria-expanded="open"
          :aria-controls="panelId"
          @click="open ? close(false) : show()"
      >
        <span
            class="sr-swatch"
            :class="{ 'is-auto': !model }"
            :style="model ? { background: model } : undefined"
        />
        <span class="icon is-small"><ChevronDown aria-hidden="true" /></span>
      </button>
    </div>
    <div
        :id="panelId"
        class="dropdown-menu"
    >
      <fieldset class="dropdown-content sr-colors">
        <legend class="sr-visually-hidden">
          {{ label }}
        </legend>
        <label class="radio">
          <input
              type="radio"
              :name
              :checked="!model"
              @change="model = undefined"
          >
          {{ t("colorPicker.auto") }}
        </label>
        <div class="sr-grid">
          <input
              v-for="s in swatches"
              :key="s.color"
              class="sr-swatch"
              type="radio"
              :name
              :value="s.color"
              :checked="model === s.color"
              :aria-label="swatchName(s)"
              :title="swatchName(s)"
              :style="{ background: s.color }"
              @change="model = s.color"
          >
        </div>
        <label class="sr-custom">
          <input
              class="sr-swatch"
              :class="{ 'is-checked': custom }"
              type="color"
              :value="model ?? '#3879f6'"
              @input="model = ($event.target as HTMLInputElement).value"
          >
          {{ t("colorPicker.custom") }}
        </label>
      </fieldset>
    </div>
  </div>
</template>

<style scoped>
.sr-colors {
  display: grid;
  gap: 0.5rem;
  padding: 0.75rem;
}

.sr-grid {
  display: grid;
  grid-template-columns: repeat(6, auto);
  gap: 0.375rem;
}

.sr-custom {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  cursor: pointer;
}

.sr-swatch {
  display: inline-block;
  width: 1.25rem;
  height: 1.25rem;
  padding: 0;
  margin: 0;
  cursor: pointer;
  border: 1px solid var(--bulma-border);
  border-radius: 4px;
  appearance: none;
}

/* Automatic shows no color of its own. */
.sr-swatch.is-auto {
  background: repeating-linear-gradient(-45deg, transparent 0 3px, var(--bulma-border) 3px 4px);
}

.sr-swatch:checked,
.sr-swatch.is-checked {
  outline: 2px solid var(--bulma-text-strong);
  outline-offset: 2px;
}

.sr-swatch:focus-visible {
  outline: 2px solid var(--bulma-link);
  outline-offset: 2px;
}

input[type="color"].sr-swatch::-webkit-color-swatch-wrapper {
  padding: 0;
}

input[type="color"].sr-swatch::-webkit-color-swatch,
input[type="color"].sr-swatch::-moz-color-swatch {
  border: none;
  border-radius: 3px;
}
</style>
