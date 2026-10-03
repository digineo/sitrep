<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import { useConfirm } from "../../shared/composables/useConfirm"
import { api, fieldErrors, localizedErrors } from "../api"
import LanguagesField from "../components/LanguagesField.vue"
import RouteField from "../components/RouteField.vue"
import TimezoneField from "../components/TimezoneField.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { resolveText } from "../rules"
import { useNotices } from "../stores/notices"
import type { Settings, Site } from "../types"

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const notices = useNotices()
const { confirm } = useConfirm()

const id = route.params.site as string
const form = ref<Site | null>(null)
const defaultTheme = ref<Settings["defaultTheme"]>("system")
const saved = ref("")
const errors = ref<Record<string, string>>({})
const saving = ref(false)

const title = computed(() => form.value
  ? resolveText(form.value.name, locale.value, form.value.languages)
  : "")

usePageTitle(() => t("site.settingsOf", { site: title.value }))
useUnsavedChanges(
  () => form.value !== null && JSON.stringify(form.value) !== saved.value,
)

function apply(site: Site) {
  form.value = site
  saved.value = JSON.stringify(site)
}

onMounted(async() => {
  try {
    const [site, settings] = await Promise.all([
      api<Site>("GET", `/api/admin/sites/${id}`),
      api<Settings>("GET", "/api/admin/settings"),
    ])
    defaultTheme.value = settings.defaultTheme
    apply(site)
  } catch(err) {
    notices.loadFailed(err)
  }
})

async function save() {
  saving.value = true
  errors.value = {}
  try {
    apply(await api<Site>("PUT", `/api/admin/sites/${id}`, form.value))
    notices.success(t("site.saved"))
    await router.push(`/sites/${id}`)
  } catch(err) {
    errors.value = fieldErrors(err)
    notices.failure(t("toast.saveFailed"), err)
  } finally {
    saving.value = false
  }
}

async function remove() {
  const ok = await confirm({
    title:   t("site.deleteTitle"),
    message: t("site.deleteMessage", { site: title.value }),
    confirm: t("site.delete"),
    danger:  true,
  })
  if (!ok) {
    return
  }

  try {
    await api("DELETE", `/api/admin/sites/${id}`)
    saved.value = JSON.stringify(form.value)
    await router.push("/")
  } catch(err) {
    notices.failure(t("toast.deleteFailed"), err)
  }
}
</script>

<template>
  <h1 class="title">
    {{ t("site.settingsOf", { site: title }) }}
  </h1>
  <form
      v-if="form"
      class="sr-form"
      @submit.prevent="save"
  >
    <SRLocalizedInput
        v-model="form.name"
        :label="t('site.name')"
        :languages="form.languages"
        required
        :maxlength="200"
        :errors="localizedErrors(errors, 'name', t)"
    />
    <LanguagesField
        v-model="form.languages"
        :errors="{ enabled: errors['languages.enabled'], primary: errors['languages.primary'] }"
    />
    <TimezoneField
        v-model="form.timezone"
        :error="errors.timezone && t(`error.${errors.timezone}`)"
    />
    <RouteField
        v-model="form.route"
        :errors="{ slug: errors['route.slug'], domain: errors['route.domain'] }"
    />
    <SRField
        v-slot="{ id: fieldId, describedby }"
        :label="t('site.theme')"
    >
      <div class="select">
        <select
            :id="fieldId"
            v-model="form.theme"
            :aria-describedby="describedby"
        >
          <option value="inherit">
            {{ t("site.themeInherit", { theme: t(`theme.${defaultTheme}`) }) }}
          </option>
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
    <div class="field is-grouped">
      <SRButton
          type="submit"
          variant="primary"
          :loading="saving"
      >
        {{ t("common.save") }}
      </SRButton>
      <RouterLink
          :to="`/sites/${id}`"
          class="button"
      >
        {{ t("common.cancel") }}
      </RouterLink>
      <SRButton
          variant="danger"
          class="ml-auto"
          @click="remove"
      >
        {{ t("site.delete") }}
      </SRButton>
    </div>
  </form>
</template>

<style scoped>
.sr-form {
  max-width: 40rem;
}
</style>
