<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"

import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import { api, fieldErrors, localizedErrors } from "../api"
import LanguagesField from "../components/LanguagesField.vue"
import LegalPageField from "../components/LegalPageField.vue"
import MarkdownEditor from "../components/MarkdownEditor.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { resolveText } from "../rules"
import { useNotices } from "../stores/notices"
import { useOverview } from "../stores/overview"
import type { Settings } from "../types"

const { t, locale } = useI18n()
const notices = useNotices()
const overview = useOverview()
usePageTitle(() => t("settings.title"))

const sites = computed(() => (overview.sites ?? [])
  .map(site => ({
    id:    site.id,
    label: resolveText(site.name, locale.value, site.languages),
  }))
  .sort((a, b) => a.label.localeCompare(b.label, locale.value)))

const form = ref<Settings | null>(null)
const saved = ref("")
const errors = ref<Record<string, string>>({})
const saving = ref(false)

useUnsavedChanges(
  () => form.value !== null && JSON.stringify(form.value) !== saved.value,
)

function apply(s: Settings) {
  form.value = s
  saved.value = JSON.stringify(s)
}

onMounted(async() => {
  try {
    apply(await api<Settings>("GET", "/api/admin/settings"))
  } catch(err) {
    notices.loadFailed(err)
  }
})

async function save() {
  saving.value = true
  errors.value = {}
  try {
    apply(await api<Settings>("PUT", "/api/admin/settings", form.value))
    notices.success(t("settings.saved"))
  } catch(err) {
    errors.value = fieldErrors(err)
    notices.failure(t("toast.saveFailed"), err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <h1 class="title">
    {{ t("settings.title") }}
  </h1>
  <form
      v-if="form"
      class="sr-form"
      @submit.prevent="save"
  >
    <h2 class="title is-5">
      {{ t("settings.languages") }}
    </h2>
    <LanguagesField
        v-model="form.languages"
        :errors="{ enabled: errors['languages.enabled'], primary: errors['languages.primary'] }"
    />
    <h2 class="title is-5 mt-5">
      {{ t("settings.appearance") }}
    </h2>
    <SRField
        v-slot="{ id, describedby, invalid }"
        :label="t('settings.defaultTheme')"
        :help="t('settings.defaultThemeHelp')"
        :error="errors.defaultTheme && t(`error.${errors.defaultTheme}`)"
    >
      <div class="select">
        <select
            :id
            v-model="form.defaultTheme"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
          <option value="light">
            {{ t("theme.light") }}
          </option>
          <option value="dark">
            {{ t("theme.dark") }}
          </option>
          <option value="system">
            {{ t("theme.system") }}
          </option>
        </select>
      </div>
    </SRField>
    <h2 class="title is-5 mt-5">
      {{ t("settings.legal") }}
    </h2>
    <p class="block sr-muted">
      {{ t("settings.legalHelp") }}
    </p>
    <LegalPageField
        v-model="form.legal.imprint"
        :label="t('legal.imprint.title')"
        :languages="form.languages"
        :errors
        path="legal.imprint"
    />
    <LegalPageField
        v-model="form.legal.privacy"
        :label="t('legal.privacy.title')"
        :languages="form.languages"
        :errors
        path="legal.privacy"
    />
    <h2 class="title is-5 mt-5">
      {{ t("settings.landing") }}
    </h2>
    <SRField
        v-slot="{ id, describedby, invalid }"
        :label="t('settings.landingSite')"
        :help="t('settings.landingSiteHelp')"
        :error="errors.landingSite && t(`error.${errors.landingSite}`)"
    >
      <div class="select">
        <select
            :id
            v-model="form.landingSite"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
          <option :value="undefined">
            {{ t("settings.landingSiteNone") }}
          </option>
          <option
              v-for="site in sites"
              :key="site.id"
              :value="site.id"
          >
            {{ site.label }}
          </option>
        </select>
      </div>
    </SRField>
    <SRLocalizedInput
        v-if="!form.landingSite"
        v-slot="{ value, update, attrs }"
        v-model="form.landing"
        :label="t('settings.landingText')"
        :languages="form.languages"
        :maxlength="50000"
        :help="t('settings.landingHelp')"
        :errors="localizedErrors(errors, 'landing', t)"
    >
      <MarkdownEditor
          :model-value="value"
          v-bind="attrs"
          @update:model-value="update"
      />
    </SRLocalizedInput>
    <SRButton
        type="submit"
        variant="primary"
        class="mt-4"
        :loading="saving"
    >
      {{ t("common.save") }}
    </SRButton>
  </form>
</template>
