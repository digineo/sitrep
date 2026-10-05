<script setup lang="ts">
import { computed, nextTick, ref, useId } from "vue"
import { useI18n } from "vue-i18n"

import { endonym } from "../i18n"

const props = defineProps<{
  label:     string
  languages: {
    enabled: string[]
    primary: string
  }
  required?:  boolean
  multiline?: boolean
  maxlength?: number
  help?:      string
  /** errors maps languages to error messages. */
  errors?:    Record<string, string | undefined>
  /** hideLabel leaves the label to assistive technology. */
  hideLabel?: boolean
}>()

/**
 * The model holds a value per language; values of languages that are not
 * enabled are kept.
 */
const model = defineModel<Record<string, string> | undefined>({ required: true })

defineSlots<{
  /** default replaces the input of a language; attrs belong on its form control. */
  default?: (props: {
    value:  string
    update: (value: string) => void
    attrs:  Record<string, unknown>
  }) => unknown
}>()

const { t } = useI18n()
const id = useId()
const selected = ref(props.languages.primary)
// A disabled language falls back to the primary one.
const active = computed(() => props.languages.enabled.includes(selected.value)
  ? selected.value
  : props.languages.primary)
const tabs = ref<HTMLButtonElement[]>([])

const value = (lang: string) => model.value?.[lang] ?? ""
const anySet = computed(
  () => props.languages.enabled.some(lang => value(lang).trim()),
)

/** missing marks a blank language while another one has a value. */
const missing = (lang: string) => anySet.value && !value(lang).trim()

/** tabLabel names a language's tab with its hints. */
function tabLabel(lang: string): string {
  return [
    endonym(lang),
    lang === props.languages.primary && t("localized.primary"),
    missing(lang) && t("localized.missing"),
  ].filter(Boolean).join(", ")
}

function update(lang: string, v: string) {
  model.value = {
    ...model.value,
    [lang]: v,
  }
}

async function select(lang: string, focus: "tab" | "input" | false = false) {
  selected.value = lang
  await nextTick()
  if (focus === "tab") {
    tabs.value[props.languages.enabled.indexOf(lang)]?.focus()
  } else if (focus) {
    document.getElementById(`${id}-${lang}`)?.focus()
  }
}

function onTabKey(event: KeyboardEvent, i: number) {
  const n = props.languages.enabled.length
  const next = {
    ArrowRight: (i + 1) % n,
    ArrowLeft:  (i - 1 + n) % n,
    Home:       0,
    End:        n - 1,
  }[event.key]
  if (next !== undefined) {
    event.preventDefault()
    void select(props.languages.enabled[next]!, "tab")
  }
}

// A failed required check shows the primary language, so that the browser
// can focus its input.
function onInvalid(lang: string) {
  if (active.value !== lang) {
    void select(lang, "input")
  }
}

/** attrs returns the attributes of a language's form control. */
function attrs(lang: string): Record<string, unknown> {
  const multi = props.languages.enabled.length > 1
  const describedby = [
    props.help && `${id}-help`,
    props.errors?.[lang] && `${id}-error-${lang}`,
  ].filter(Boolean).join(" ") || undefined
  return {
    "id":               `${id}-${lang}`,
    lang,
    "required":         props.required && lang === props.languages.primary,
    "maxlength":        props.maxlength,
    "aria-labelledby":  multi ? `${id}-label ${id}-tab-${lang}` : `${id}-label`,
    "aria-describedby": describedby,
    "aria-invalid":     props.errors?.[lang] ? true : undefined,
    "onInvalid":        () => onInvalid(lang),
  }
}
</script>

<template>
  <div class="field">
    <label
        :id="`${id}-label`"
        class="label"
        :class="{ 'sr-visually-hidden': hideLabel }"
        :for="`${id}-${active}`"
    >{{ label }}</label>
    <div
        v-if="languages.enabled.length > 1"
        class="tabs is-small mb-2"
    >
      <ul role="tablist">
        <li
            v-for="(lang, i) in languages.enabled"
            :key="lang"
            :class="{ 'is-active': lang === active }"
            role="presentation"
        >
          <button
              :id="`${id}-tab-${lang}`"
              ref="tabs"
              type="button"
              role="tab"
              class="sr-tab"
              :aria-selected="lang === active"
              :aria-controls="`${id}-panel-${lang}`"
              :aria-label="tabLabel(lang)"
              :tabindex="lang === active ? 0 : -1"
              @click="select(lang)"
              @keydown="onTabKey($event, i)"
          >
            <span :lang>{{ endonym(lang) }}</span>
            <span
                v-if="lang === languages.primary"
                class="tag ml-1"
            >{{ t("localized.primary") }}</span>
            <span
                v-if="missing(lang)"
                class="tag is-warning ml-1"
            >{{ t("localized.missing") }}</span>
            <span
                v-if="errors?.[lang]"
                class="tag is-danger is-small ml-1"
            >!</span>
          </button>
        </li>
      </ul>
    </div>
    <div
        v-for="lang in languages.enabled"
        v-show="lang === active"
        :id="`${id}-panel-${lang}`"
        :key="lang"
        class="control"
        :role="languages.enabled.length > 1 ? 'tabpanel' : undefined"
        :aria-labelledby="languages.enabled.length > 1 ? `${id}-tab-${lang}` : undefined"
    >
      <slot
          v-if="$slots.default"
          :value="value(lang)"
          :update="(v: string) => update(lang, v)"
          :attrs="attrs(lang)"
      />
      <component
          :is="multiline ? 'textarea' : 'input'"
          v-else
          v-bind="attrs(lang)"
          :class="multiline ? 'textarea sr-autogrow' : 'input'"
          :value="value(lang)"
          @input="update(lang, ($event.target as HTMLInputElement).value)"
      />
      <p
          v-if="errors?.[lang]"
          :id="`${id}-error-${lang}`"
          class="help is-danger"
      >
        {{ errors[lang] }}
      </p>
    </div>
    <p
        v-if="help"
        :id="`${id}-help`"
        class="help"
    >
      {{ help }}
    </p>
  </div>
</template>

<style scoped>
.sr-tab {
  display: flex;
  align-items: center;
  padding: 0.5em 1em;
  font: inherit;
  color: inherit;
  cursor: pointer;
  background: none;
  border: none;
  border-bottom: 1px solid transparent;
}

.is-active .sr-tab {
  color: var(--bulma-link);
  border-bottom-color: var(--bulma-link);
}
</style>
