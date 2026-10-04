<script setup lang="ts">
import { ExternalLink, GripVertical, Pencil, Plus } from "@lucide/vue"
import {
  computed,
  inject,
  nextTick,
  onBeforeUnmount,
  reactive,
  ref,
  shallowRef,
  watch,
} from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import { type Bootstrap } from "../../shared/bootstrap"
import IncidentList from "../../shared/components/IncidentList.vue"
import LiveIndicator from "../../shared/components/LiveIndicator.vue"
import SRField from "../../shared/components/SRField.vue"
import StatusBanner from "../../shared/components/StatusBanner.vue"
import { usePolling } from "../../shared/composables/usePolling"
import { formatMiB } from "../../shared/format"
import { endonym } from "../../shared/i18n"
import PanelGroups from "../../shared/panels/PanelGroups.vue"
import type { PanelInfo, Payload } from "../../shared/payload"
import { api, errorCode } from "../api"
import { coalesce } from "../coalesce"
import { usePageTitle } from "../composables/usePageTitle"
import { moveBy, moveTo, ordered } from "../order"
import { siteURL } from "../rules"
import { useNotices } from "../stores/notices"
import { useOverview } from "../stores/overview"

const { t, locale } = useI18n()
const route = useRoute()
const notices = useNotices()
const overview = useOverview()
const bootstrap = inject<Bootstrap>("bootstrap")!

const id = route.params.site as string
const summary = computed(() => overview.sites?.find(s => s.id === id))
const publicURL = computed(() => summary.value && siteURL(
  summary.value.route,
  bootstrap.baseDomains ?? [],
  window.location,
  summary.value.landing,
))
const lang = ref(locale.value)
const payload = shallowRef<Payload | null>(null)
const live = reactive({
  state:     "loading" as "loading" | "ok" | "failed",
  updatedAt: undefined as Date | undefined,
  pulse:     0,
  error:     "",
  detail:    "",
})

// The preview shows the site in a language it has.
watch(summary, (s) => {
  if (s && !s.languages.enabled.includes(lang.value)) {
    lang.value = s.languages.primary
  }
}, { immediate: true })

usePageTitle(() => payload.value?.site?.name ?? "")

const incidentLink = (incident: string) => `/sites/${id}/incidents/${incident}`

/** order holds the panel order while it differs from the stored one. */
const order = ref<string[] | null>(null)
const dragging = ref<string | null>(null)
const announcement = ref("")
const panels = computed(() => {
  const all = payload.value?.site?.panels ?? []
  return order.value ? ordered(all, order.value) : all
})

const loadPreview = (signal: AbortSignal) => api<Payload>(
  "GET",
  `/api/admin/sites/${id}/preview?lang=${lang.value}`,
  undefined,
  signal,
)
const { refresh } = usePolling(loadPreview, {
  interval: () => 15_000,
  paused:   () => dragging.value !== null,
  onResult: (p) => {
    payload.value = p
    if (!saving.value) {
      order.value = null
    }

    Object.assign(live, {
      state:     "ok",
      updatedAt: new Date(),
      pulse:     live.pulse + 1,
      error:     "",
      detail:    "",
    })
  },
  onError: (err) => {
    Object.assign(live, {
      state:  "failed",
      error:  t(`error.${errorCode(err)}`),
      detail: err instanceof Error ? err.message : "",
    })
  },
})
watch(lang, () => void refresh())

const saving = ref(false)
const saveOrder = coalesce(async(ids: string[]) => {
  await api("PUT", `/api/admin/sites/${id}/panel-order`, { panels: ids })
})

/** store saves the order; on failure, the stored order returns. */
async function store(ids: string[]) {
  order.value = ids
  saving.value = true
  try {
    await saveOrder(ids)
  } catch(err) {
    order.value = null
    notices.failure(t("toast.saveFailed"), err)
  } finally {
    saving.value = false
    void refresh()
  }
}

async function onGripKey(event: KeyboardEvent, panel: PanelInfo) {
  if (event.key === "Escape") {
    cancelDrag()
    return
  }

  const by = {
    ArrowUp:    -1,
    ArrowLeft:  -1,
    ArrowDown:  1,
    ArrowRight: 1,
  }[event.key]
  if (by === undefined) {
    return
  }

  event.preventDefault()
  const ids = moveBy(panels.value, panel.id, by)
  const group = ordered(panels.value, ids).filter(p => p.type === panel.type)
  void store(ids)
  announcement.value = t("preview.moved", {
    title:    panel.title,
    position: group.findIndex(p => p.id === panel.id) + 1,
    count:    group.length,
  })
  await nextTick()
  document.querySelector<HTMLElement>(`[data-grip="${panel.id}"]`)?.focus()
}

// Dragging a grip moves its panel live onto other panels of its group.
// Moving elements loses pointer capture, so the window follows the pointer.
/** warningText joins the texts of a panel's warnings. */
function warningText(panel: string): string {
  return (payload.value?.panels?.[panel]?.warnings ?? [])
    .map(w => t(`warnings.${w.code}`, {
      count: w.count,
      size:  formatMiB(w.size ?? 0, locale.value),
    }))
    .join(" ")
}

function onGripDown(event: PointerEvent, panel: PanelInfo) {
  if (event.button !== 0) {
    return
  }

  event.preventDefault()
  dragging.value = panel.id
  order.value = panels.value.map(p => p.id)
  window.addEventListener("pointermove", onDragMove)
  window.addEventListener("pointerup", onDragEnd)
  window.addEventListener("pointercancel", cancelDrag)
}

function onDragMove(event: PointerEvent) {
  const element = document.elementFromPoint(event.clientX, event.clientY)
  const target = element?.closest<HTMLElement>("[data-panel-id]")?.dataset.panelId
  if (dragging.value && target && target !== dragging.value) {
    order.value = moveTo(panels.value, dragging.value, target)
  }
}

function stopListening() {
  window.removeEventListener("pointermove", onDragMove)
  window.removeEventListener("pointerup", onDragEnd)
  window.removeEventListener("pointercancel", cancelDrag)
}

function onDragEnd() {
  stopListening()
  if (dragging.value) {
    dragging.value = null
    void store(order.value!)
  }
}

function cancelDrag() {
  stopListening()
  if (dragging.value) {
    dragging.value = null
    order.value = null
  }
}

onBeforeUnmount(stopListening)
</script>

<template>
  <div class="sr-preview-head">
    <h1 class="title mb-0">
      {{ payload?.site?.name }}
    </h1>
    <a
        v-if="publicURL"
        :href="publicURL"
        target="_blank"
        rel="noopener"
        class="button is-small"
    >
      <span class="icon"><ExternalLink aria-hidden="true" /></span>
      <span>{{ t("preview.public") }}</span>
    </a>
    <SRField
        v-if="summary && summary.languages.enabled.length > 1"
        v-slot="{ id: fieldId }"
        :label="t('preview.language')"
        class="sr-inline-field mb-0"
    >
      <div class="select is-small">
        <select
            :id="fieldId"
            v-model="lang"
        >
          <option
              v-for="l in summary.languages.enabled"
              :key="l"
              :value="l"
              :lang="l"
          >
            {{ endonym(l) }}
          </option>
        </select>
      </div>
    </SRField>
    <LiveIndicator
        :state="live.state"
        :updated-at="live.updatedAt"
        :timezone="payload?.site?.timezone ?? 'UTC'"
        :pulse="live.pulse"
        :error="live.error"
        :detail="live.detail"
    />
    <RouterLink
        :to="`/sites/${id}/panels/new`"
        class="button is-primary ml-auto"
    >
      <span class="icon"><Plus aria-hidden="true" /></span>
      <span>{{ t("preview.addPanel") }}</span>
    </RouterLink>
  </div>
  <template v-if="payload?.site">
    <StatusBanner :status="payload.status" />
    <template v-if="payload.incidents">
      <IncidentList
          v-if="payload.incidents.ongoing.length"
          :title="t('incidents.current')"
          :incidents="payload.incidents.ongoing"
          :timezone="payload.site.timezone"
          :to="incidentLink"
      />
      <IncidentList
          v-if="payload.incidents.upcoming.length"
          :title="t('incidents.upcoming')"
          :incidents="payload.incidents.upcoming"
          :timezone="payload.site.timezone"
          :to="incidentLink"
      />
    </template>
    <p
        v-if="!panels.length"
        class="box"
    >
      {{ t("preview.empty") }}
    </p>
    <PanelGroups
        :panels
        :data="payload.panels ?? {}"
        :timezone="payload.site.timezone"
        :spans="payload.incidents?.spans"
        :incident-link
        :class="{ 'sr-dragging': dragging }"
    >
      <template #actions="{ panel }">
        <span class="sr-panel-actions">
          <span
              v-if="payload.panels?.[panel.id]?.error"
              class="tag is-danger"
              :title="payload.panels[panel.id]!.error"
          >
            {{ t("preview.pollFailed") }}
            <span class="sr-visually-hidden">: {{ payload.panels[panel.id]!.error }}</span>
          </span>
          <span
              v-if="payload.panels?.[panel.id]?.warnings?.length"
              class="tag is-warning"
              :title="warningText(panel.id)"
          >
            {{ t("preview.warning") }}
            <span class="sr-visually-hidden">: {{ warningText(panel.id) }}</span>
          </span>
          <button
              type="button"
              class="button is-small sr-grip"
              :data-grip="panel.id"
              :aria-label="t('preview.move', { title: panel.title })"
              :title="t('preview.moveHint')"
              @keydown="onGripKey($event, panel)"
              @pointerdown="onGripDown($event, panel)"
          >
            <span class="icon"><GripVertical aria-hidden="true" /></span>
          </button>
          <RouterLink
              :to="`/sites/${id}/panels/${panel.id}`"
              class="button is-small"
              :aria-label="t('preview.edit', { title: panel.title })"
              :title="t('preview.editHint')"
          >
            <span class="icon"><Pencil aria-hidden="true" /></span>
          </RouterLink>
        </span>
      </template>
    </PanelGroups>
    <IncidentList
        v-if="payload.incidents?.finished.length"
        :title="t('incidents.recent')"
        :incidents="payload.incidents.finished.slice(0, 5)"
        :timezone="payload.site.timezone"
        :to="incidentLink"
    />
  </template>
  <p
      class="sr-visually-hidden"
      aria-live="polite"
  >
    {{ announcement }}
  </p>
</template>

<style scoped>
.sr-preview-head {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
  margin-bottom: 1.5rem;
}

.sr-inline-field {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.sr-inline-field :deep(.label) {
  margin: 0;
  font-weight: normal;
}

.sr-panel-actions {
  display: inline-flex;
  gap: 0.25rem;
  align-items: center;
  margin-left: auto;
}

.sr-grip {
  cursor: grab;
  touch-action: none;
}

.sr-dragging .sr-grip {
  cursor: grabbing;
}
</style>
