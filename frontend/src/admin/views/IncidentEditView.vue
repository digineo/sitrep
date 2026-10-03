<script setup lang="ts">
import { ExternalLink, Pencil, Plus, Trash2 } from "@lucide/vue"
import { computed, inject, ref, shallowRef } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import { type Bootstrap } from "../../shared/bootstrap"
import IncidentTags from "../../shared/components/IncidentTags.vue"
import SRButton from "../../shared/components/SRButton.vue"
import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import UpdateEntry from "../../shared/components/UpdateEntry.vue"
import { useConfirm } from "../../shared/composables/useConfirm"
import { formatDateTime } from "../../shared/format"
import { api, ApiError, fieldErrors, localizedErrors } from "../api"
import UpdateFields from "../components/UpdateFields.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { deletable, draftOf, updateBody, type UpdateDraft } from "../incidents"
import { resolveText, siteURL } from "../rules"
import { useNotices } from "../stores/notices"
import type { Incident, IncidentUpdate, Site, Text } from "../types"
import NotFoundView from "./NotFoundView.vue"

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const notices = useNotices()
const { confirm } = useConfirm()
const bootstrap = inject<Bootstrap>("bootstrap")!

const siteId = route.params.site as string
const incidentId = route.params.incident as string
const base = `/api/admin/sites/${siteId}/incidents/${incidentId}`
const site = shallowRef<Site | null>(null)
const incident = shallowRef<Incident | null>(null)
const notFound = ref(false)

const resolve = (text: Text | undefined) =>
  resolveText(text, locale.value, site.value!.languages)
const title = computed(() => incident.value && site.value
  ? resolve(incident.value.title)
  : "")
usePageTitle(() => title.value)
const publicURL = computed(() => site.value && `${
  siteURL(site.value.route, bootstrap.baseDomains ?? [], window.location)
}incidents/${incidentId}`)
const time = (update: IncidentUpdate) =>
  formatDateTime(new Date(update.at), locale.value, site.value!.timezone)

const titleDraft = ref<Text>({})
const titleErrors = ref<Record<string, string>>({})
const titleChanged = computed(() => !!incident.value
  && JSON.stringify(titleDraft.value) !== JSON.stringify(incident.value.title))

const emptyDraft = (): UpdateDraft => ({
  at:          "",
  status:      "",
  severity:    "",
  description: {},
})
const newDraft = ref(emptyDraft())
const newErrors = ref<Record<string, string>>({})
const newChanged = computed(
  () => JSON.stringify(newDraft.value) !== JSON.stringify(emptyDraft()),
)

/** editing is the update being edited, with its form fields as loaded. */
const editing = ref<{
  id:      string
  initial: UpdateDraft
} | null>(null)
const editDraft = ref(emptyDraft())
const editErrors = ref<Record<string, string>>({})
const editChanged = computed(() => !!editing.value
  && JSON.stringify(editDraft.value) !== JSON.stringify(editing.value.initial))

const busy = ref(false)
useUnsavedChanges(() => titleChanged.value || newChanged.value || editChanged.value)

Promise.all([
  api<Site>("GET", `/api/admin/sites/${siteId}`),
  api<Incident>("GET", base),
]).then(([s, i]) => {
  site.value = s
  show(i)
}).catch((err) => {
  notFound.value = err instanceof ApiError && err.status === 404
  if (!notFound.value) {
    notices.loadFailed(err)
  }
})

/** show displays a stored incident and resets the title form. */
function show(i: Incident) {
  incident.value = i
  titleDraft.value = { ...i.title }
}

/**
 * run sends a change; the answer is the changed incident, or nothing if it is
 * gone.
 */
async function run(
  errors: typeof titleErrors,
  send: () => Promise<Incident | undefined>,
  success: string,
  failure = t("toast.saveFailed"),
): Promise<boolean> {
  busy.value = true
  errors.value = {}
  try {
    const changed = await send()
    notices.success(success)
    if (!changed) {
      incident.value = null
      await router.push(`/sites/${siteId}/incidents`)
      return true
    }

    show(changed)
    return true
  } catch(err) {
    errors.value = fieldErrors(err)
    notices.failure(failure, err)
    return false
  } finally {
    busy.value = false
  }
}

function saveTitle() {
  void run(
    titleErrors,
    () => api<Incident>("PUT", base, { title: titleDraft.value }),
    t("incidentEditor.titleSaved"),
  )
}

async function addUpdate() {
  const posted = await run(
    newErrors,
    () => api<Incident>(
      "POST",
      `${base}/updates`,
      updateBody(newDraft.value, site.value!.timezone),
    ),
    t("incidentEditor.updatePosted"),
  )
  if (posted) {
    newDraft.value = emptyDraft()
  }
}

async function edit(update: IncidentUpdate) {
  if (editChanged.value && !await confirmDiscard()) {
    return
  }

  const initial = draftOf(update, site.value!.timezone)
  editing.value = {
    id: update.id,
    initial,
  }
  editDraft.value = {
    ...initial,
    description: { ...initial.description },
  }
  editErrors.value = {}
}

async function stopEditing() {
  if (!editChanged.value || await confirmDiscard()) {
    editing.value = null
  }
}

function confirmDiscard() {
  return confirm({
    title:   t("unsaved.title"),
    message: t("unsaved.message"),
    confirm: t("unsaved.discard"),
    danger:  true,
  })
}

async function saveEdit() {
  const { id, initial } = editing.value!
  const body = updateBody(editDraft.value, site.value!.timezone, initial.at)
  const saved = await run(
    editErrors,
    () => api<Incident>("PUT", `${base}/updates/${id}`, body),
    t("incidentEditor.updateSaved"),
  )
  if (saved) {
    editing.value = null
  }
}

async function removeUpdate(update: IncidentUpdate) {
  const only = incident.value!.updates.length === 1
  const ok = await confirm(only
    ? {
      title:   t("incidentEditor.deleteLastTitle"),
      message: t("incidentEditor.deleteLastMessage", { title: title.value }),
      confirm: t("incidentEditor.delete"),
      danger:  true,
    }
    : {
      title:   t("incidentEditor.deleteUpdateTitle"),
      message: t("incidentEditor.deleteUpdateMessage", { time: time(update) }),
      confirm: t("incidentEditor.confirmDeleteUpdate"),
      danger:  true,
    })
  if (ok) {
    await run(
      editErrors,
      () => api<Incident | undefined>("DELETE", `${base}/updates/${update.id}`),
      only ? t("incidentEditor.deleted") : t("incidentEditor.updateDeleted"),
      t("toast.deleteFailed"),
    )
  }
}

async function removeIncident() {
  const ok = await confirm({
    title:   t("incidentEditor.deleteTitle"),
    message: t("incidentEditor.deleteMessage", { title: title.value }),
    confirm: t("incidentEditor.delete"),
    danger:  true,
  })
  if (!ok) {
    return
  }

  try {
    await api("DELETE", base)
    incident.value = null
    titleDraft.value = {}
    newDraft.value = emptyDraft()
    editing.value = null
    notices.success(t("incidentEditor.deleted"))
    await router.push(`/sites/${siteId}/incidents`)
  } catch(err) {
    notices.failure(t("toast.deleteFailed"), err)
  }
}

/** history lists the updates newest first, with their position in time. */
const history = computed(() => (incident.value?.updates ?? [])
  .map((update, index) => ({
    update,
    index,
  }))
  .toReversed())
</script>

<template>
  <NotFoundView v-if="notFound" />
  <template v-else-if="site && incident">
    <div class="sr-view-head block">
      <h1 class="title mb-0">
        {{ title }}
      </h1>
      <IncidentTags
          :status="incident.status"
          :severity="incident.severity"
      />
    </div>
    <div class="buttons block">
      <RouterLink
          :to="`/sites/${siteId}/incidents`"
          class="button"
      >
        {{ t("incidentEditor.allIncidents") }}
      </RouterLink>
      <a
          v-if="publicURL"
          :href="publicURL"
          class="button"
          target="_blank"
          rel="noopener"
      >
        <span>{{ t("incidentEditor.public") }}</span>
        <span class="icon"><ExternalLink aria-hidden="true" /></span>
      </a>
      <SRButton
          variant="danger"
          @click="removeIncident"
      >
        {{ t("incidentEditor.delete") }}
      </SRButton>
    </div>

    <form
        class="box sr-form"
        @submit.prevent="saveTitle"
    >
      <SRLocalizedInput
          v-model="titleDraft"
          :label="t('incidentEditor.title')"
          :languages="site.languages"
          required
          :maxlength="200"
          :errors="localizedErrors(titleErrors, 'title', t)"
      />
      <SRButton
          type="submit"
          :loading="busy"
          :disabled="!titleChanged"
      >
        {{ t("incidentEditor.saveTitle") }}
      </SRButton>
    </form>

    <SRButton
        v-if="editing"
        class="block"
        @click="stopEditing"
    >
      <span class="icon"><Plus aria-hidden="true" /></span>
      <span>{{ t("incidentEditor.addUpdate") }}</span>
    </SRButton>
    <form
        v-else
        class="box sr-form"
        @submit.prevent="addUpdate"
    >
      <h2 class="title is-5">
        {{ t("incidentEditor.newUpdate") }}
      </h2>
      <UpdateFields
          v-model="newDraft"
          :opening="false"
          :timezone="site.timezone"
          :languages="site.languages"
          :errors="newErrors"
      />
      <SRButton
          type="submit"
          variant="primary"
          :loading="busy"
      >
        {{ t("incidentEditor.postUpdate") }}
      </SRButton>
    </form>

    <h2 class="title is-5">
      {{ t("incidentEditor.history") }}
    </h2>
    <ol class="sr-timeline">
      <li
          v-for="{ update, index } in history"
          :key="update.id"
      >
        <form
            v-if="editing?.id === update.id"
            class="box sr-form sr-editing"
            @submit.prevent="saveEdit"
        >
          <UpdateFields
              v-model="editDraft"
              :opening="index === 0"
              edit
              :timezone="site.timezone"
              :languages="site.languages"
              :errors="editErrors"
          />
          <div class="buttons">
            <SRButton
                type="submit"
                variant="primary"
                :loading="busy"
            >
              {{ t("common.save") }}
            </SRButton>
            <SRButton @click="stopEditing">
              {{ t("common.cancel") }}
            </SRButton>
          </div>
        </form>
        <UpdateEntry
            v-else
            :update="{ ...update, html: resolve(update.html) }"
            :timezone="site.timezone"
        >
          <p class="is-size-7 sr-muted">
            {{ t("incidentEditor.author", { name: update.author.displayName }) }}
            <template v-if="update.editedBy && update.editedBy.subject !== update.author.subject">
              · {{ t("incidentEditor.editedBy", { name: update.editedBy.displayName }) }}
            </template>
          </p>
          <div class="buttons are-small mt-2">
            <SRButton
                :aria-label="t('incidentEditor.editUpdate', { time: time(update) })"
                @click="edit(update)"
            >
              <span class="icon"><Pencil aria-hidden="true" /></span>
            </SRButton>
            <SRButton
                :aria-label="t('incidentEditor.deleteUpdate', { time: time(update) })"
                :disabled="!deletable(incident.updates, index)"
                :aria-describedby="deletable(incident.updates, index) ? undefined : `locked-${update.id}`"
                @click="removeUpdate(update)"
            >
              <span class="icon"><Trash2 aria-hidden="true" /></span>
            </SRButton>
          </div>
          <p
              v-if="!deletable(incident.updates, index)"
              :id="`locked-${update.id}`"
              class="help"
          >
            {{ t("incidentEditor.openingLocked") }}
          </p>
        </UpdateEntry>
      </li>
    </ol>
  </template>
</template>

<style scoped>
.sr-view-head {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
}

.sr-form {
  max-width: 60rem;
}

.sr-editing {
  outline: 2px solid var(--bulma-link);
}
</style>
