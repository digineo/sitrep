<script setup lang="ts">
import { computed, ref, shallowRef } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import SRButton from "../../shared/components/SRButton.vue"
import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import { api, ApiError, fieldErrors, localizedErrors } from "../api"
import UpdateFields from "../components/UpdateFields.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { draftOf, updateBody } from "../incidents"
import { useNotices } from "../stores/notices"
import type { Incident, Site, Text } from "../types"
import NotFoundView from "./NotFoundView.vue"

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const notices = useNotices()
usePageTitle(() => t("incidentEditor.new"))

const id = route.params.site as string
const site = shallowRef<Site | null>(null)
const notFound = ref(false)
const title = ref<Text>({})
const draft = ref(draftOf(null, "UTC"))
const errors = ref<Record<string, string>>({})
const saving = ref(false)
const initial = JSON.stringify([title.value, draft.value])
const dirty = computed(() => JSON.stringify([title.value, draft.value]) !== initial)
useUnsavedChanges(() => dirty.value)

api<Site>("GET", `/api/admin/sites/${id}`)
  .then(s => (site.value = s))
  .catch((err) => {
    notFound.value = err instanceof ApiError && err.status === 404
    if (!notFound.value) {
      notices.loadFailed(err)
    }
  })

async function create() {
  saving.value = true
  errors.value = {}
  try {
    const path = `/api/admin/sites/${id}/incidents`
    const incident = await api<Incident>("POST", path, {
      title:  title.value,
      update: updateBody(draft.value, site.value!.timezone),
    })
    title.value = {}
    draft.value = draftOf(null, "UTC")
    notices.success(t("incidentEditor.created"))
    await router.push(`/sites/${id}/incidents/${incident.id}`)
  } catch(err) {
    errors.value = fieldErrors(err)
    notices.failure(t("toast.saveFailed"), err)
  } finally {
    saving.value = false
  }
}

/** updateErrors returns the errors of the opening update's fields. */
const updateErrors = computed(() => Object.fromEntries(Object.entries(errors.value)
  .filter(([path]) => path.startsWith("update."))
  .map(([path, code]) => [path.slice("update.".length), code])))
</script>

<template>
  <NotFoundView v-if="notFound" />
  <template v-else-if="site">
    <h1 class="title">
      {{ t("incidentEditor.new") }}
    </h1>
    <form
        class="sr-form"
        @submit.prevent="create"
    >
      <SRLocalizedInput
          v-model="title"
          :label="t('incidentEditor.title')"
          :languages="site.languages"
          required
          :maxlength="200"
          :errors="localizedErrors(errors, 'title', t)"
      />
      <UpdateFields
          v-model="draft"
          opening
          :timezone="site.timezone"
          :languages="site.languages"
          :errors="updateErrors"
      />
      <div class="buttons">
        <SRButton
            type="submit"
            variant="primary"
            :loading="saving"
        >
          {{ t("incidentEditor.create") }}
        </SRButton>
        <RouterLink
            :to="`/sites/${id}/incidents`"
            class="button"
        >
          {{ t("common.cancel") }}
        </RouterLink>
      </div>
    </form>
  </template>
</template>

<style scoped>
.sr-form {
  max-width: 60rem;
}
</style>
