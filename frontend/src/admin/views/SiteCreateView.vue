<script setup lang="ts">
import { inject, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useRouter } from "vue-router"

import { type Bootstrap } from "../../shared/bootstrap"
import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import { endonym } from "../../shared/i18n"
import { api, fieldErrors } from "../api"
import LanguagesField from "../components/LanguagesField.vue"
import RouteField from "../components/RouteField.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { useNotices } from "../stores/notices"
import type { Languages, Route, Settings, Site } from "../types"

const { t } = useI18n()
const router = useRouter()
const notices = useNotices()
const bootstrap = inject<Bootstrap>("bootstrap")!
usePageTitle(() => t("site.create"))

const languages = ref<Languages>({
  enabled: bootstrap.languages,
  primary: bootstrap.primary,
})
const name = ref("")
const route = ref<Route>({
  mode: "path",
  slug: "",
})
const errors = ref<Record<string, string>>({})
const saving = ref(false)
const dirty = ref(true)
useUnsavedChanges(
  () => dirty.value && (name.value !== "" || route.value.slug !== ""),
)

// New sites start with the instance's languages.
void api<Settings>("GET", "/api/admin/settings")
  .then(s => (languages.value = s.languages))
  .catch(notices.loadFailed)

async function create() {
  saving.value = true
  errors.value = {}
  try {
    const site = await api<Site>("POST", "/api/admin/sites", {
      name:      { [languages.value.primary]: name.value },
      languages: languages.value,
      route:     route.value,
    })
    dirty.value = false
    notices.success(t("site.created"))
    await router.push(`/sites/${site.id}`)
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
    {{ t("site.create") }}
  </h1>
  <form
      class="sr-form"
      @submit.prevent="create"
  >
    <LanguagesField
        v-model="languages"
        :errors="{ enabled: errors['languages.enabled'], primary: errors['languages.primary'] }"
    />
    <SRField
        v-slot="{ id, describedby, invalid }"
        :label="t('site.nameIn', { language: endonym(languages.primary) })"
        :help="t('site.nameHelp')"
        :error="errors[`name.${languages.primary}`] && t(`error.${errors[`name.${languages.primary}`]}`)"
    >
      <input
          :id
          v-model="name"
          class="input"
          required
          maxlength="200"
          :lang="languages.primary"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
      >
    </SRField>
    <RouteField
        v-model="route"
        :errors="{ slug: errors['route.slug'], domain: errors['route.domain'] }"
    />
    <SRButton
        type="submit"
        variant="primary"
        :loading="saving"
    >
      {{ t("site.createSubmit") }}
    </SRButton>
  </form>
</template>

<style scoped>
.sr-form {
  max-width: 40rem;
}
</style>
