<script setup lang="ts">
import { onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"

import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import { api, fieldErrors } from "../api"
import LanguagesField from "../components/LanguagesField.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { useNotices } from "../stores/notices"
import type { Settings } from "../types"

const { t } = useI18n()
const notices = useNotices()
usePageTitle(() => t("settings.title"))

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
