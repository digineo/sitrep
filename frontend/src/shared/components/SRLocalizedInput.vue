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
}>()

/**
 * The model holds a value per language; values of languages that are not
 * enabled are kept.
 */
const model = defineModel<Record<string, string> | undefined>({ required: true })

const { t } = useI18n()
const id = useId()
const active = ref(props.languages.primary)
const tabs = ref<HTMLButtonElement[]>([])
const inputs = ref<HTMLInputElement[]>([])

const value = (lang: string) => model.value?.[lang] ?? ""
const anySet = computed(
  () => props.languages.enabled.some(lang => value(lang).trim()),
)

/** missing marks a blank language while another one has a value. */
const missing = (lang: string) => lang !== props.languages.primary
  && anySet.value && !value(lang).trim()

/** marker returns the hint shown on a language's tab. */
function marker(lang: string): string {
  if (lang === props.languages.primary) {
    return t("localized.primary")
  }
  return missing(lang) ? t("localized.missing") : ""
}

function update(lang: string, v: string) {
  model.value = {
    ...model.value,
    [lang]: v,
  }
}

async function select(lang: string, focus: "tab" | "input" | false = false) {
  active.value = lang
  await nextTick()
  const i = props.languages.enabled.indexOf(lang)
  if (focus) {
    (focus === "tab" ? tabs.value[i] : inputs.value[i])?.focus()
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
</script>

<template>
  <div class="field">
    <label
        :id="`${id}-label`"
        class="label"
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
              :aria-label="[endonym(lang), marker(lang)].filter(Boolean).join(', ')"
              :tabindex="lang === active ? 0 : -1"
              @click="select(lang)"
              @keydown="onTabKey($event, i)"
          >
            <span :lang>{{ endonym(lang) }}</span>
            <span
                v-if="marker(lang)"
                class="tag ml-1"
                :class="{ 'is-warning': missing(lang) }"
            >{{ marker(lang) }}</span>
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
      <component
          :is="multiline ? 'textarea' : 'input'"
          :id="`${id}-${lang}`"
          ref="inputs"
          :class="multiline ? 'textarea sr-autogrow' : 'input'"
          :lang
          :value="value(lang)"
          :required="required && lang === languages.primary"
          :maxlength
          :aria-labelledby="languages.enabled.length > 1 ? `${id}-label ${id}-tab-${lang}` : `${id}-label`"
          :aria-describedby="[help && `${id}-help`, errors?.[lang] && `${id}-error-${lang}`].filter(Boolean).join(' ') || undefined"
          :aria-invalid="errors?.[lang] ? true : undefined"
          @input="update(lang, ($event.target as HTMLInputElement).value)"
          @invalid="onInvalid(lang)"
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
