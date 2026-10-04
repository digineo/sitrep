<script setup lang="ts">
import { computed, inject, onMounted, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import { type Bootstrap } from "../../shared/bootstrap"
import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import SRLocalizedInput from "../../shared/components/SRLocalizedInput.vue"
import { useConfirm } from "../../shared/composables/useConfirm"
import { watchDebounced } from "../../shared/composables/watchDebounced"
import { formatMiB } from "../../shared/format"
import { endonym } from "../../shared/i18n"
import StatPanel from "../../shared/panels/StatPanel.vue"
import StatusTable from "../../shared/panels/StatusTable.vue"
import TimeseriesPanel from "../../shared/panels/TimeseriesPanel.vue"
import type { PanelData, PanelInfo, Warning } from "../../shared/payload"
import { api, errorCode, fieldErrors, localizedErrors } from "../api"
import JsonTree from "../components/JsonTree.vue"
import QueryEditor from "../components/QueryEditor.vue"
import ThresholdsField from "../components/ThresholdsField.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { parseDuration, resolveText } from "../rules"
import { useNotices } from "../stores/notices"
import type { DataSource, DataSourceType, Panel, PanelType, Site } from "../types"

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const notices = useNotices()
const { confirm } = useConfirm()
const bootstrap = inject<Bootstrap>("bootstrap")!

const siteId = route.params.site as string
const panelId = route.params.panel as string | undefined
usePageTitle(() => panelId ? t("panelEditor.edit") : t("panelEditor.new"))

const site = ref<Site | null>(null)
const sources = ref<DataSource[]>([])
const types = ref<DataSourceType[]>([])
const form = ref<Panel>({
  type:       "stat",
  title:      {},
  datasource: "",
  query:      "",
  thresholds: [],
})
const errors = ref<Record<string, string>>({})
const saving = ref(false)
const saved = ref("")
useUnsavedChanges(
  () => saved.value !== "" && JSON.stringify(form.value) !== saved.value,
)

const source = computed(
  () => sources.value.find(ds => ds.id === form.value.datasource),
)
const sourceType = computed(
  () => types.value.find(ty => ty.id === source.value?.type),
)
const panelTypes = computed(() => sourceType.value?.panelTypes ?? [])
const languages = computed(() => site.value?.languages ?? {
  enabled: [bootstrap.primary],
  primary: bootstrap.primary,
})

onMounted(async() => {
  try {
    const [s, ds, ty] = await Promise.all([
      api<Site>("GET", `/api/admin/sites/${siteId}`),
      api<DataSource[]>("GET", "/api/admin/datasources"),
      api<DataSourceType[]>("GET", "/api/admin/datasource-types"),
    ])
    site.value = s
    const collator = new Intl.Collator(locale.value)
    sources.value = ds.sort((a, b) => collator.compare(a.name, b.name))
    types.value = ty

    if (panelId) {
      form.value = {
        thresholds: [],
        ...await api<Panel>("GET", `/api/admin/sites/${siteId}/panels/${panelId}`),
      }
    } else if (sources.value.length === 1) {
      form.value.datasource = sources.value[0]!.id
    }

    previewLang.value = languages.value.enabled.includes(locale.value)
      ? locale.value
      : languages.value.primary
    saved.value = JSON.stringify(form.value)
  } catch(err) {
    notices.loadFailed(err)
  }
})

// Options of a newly chosen type start with their defaults.
watch(() => form.value.type, () => {
  form.value.reduce ??= "first"
  form.value.style ??= "line"
}, { immediate: true })

// A data source that cannot feed the panel's type asks for a new type.
watch(() => form.value.datasource, () => {
  if (sourceType.value && !panelTypes.value.includes(form.value.type)) {
    form.value.type = "" as PanelType
  }
})

/** body returns the panel to submit: only the options of its type. */
function body(): Panel {
  const f = form.value
  const decimals = typeof f.decimals === "number" && !Number.isNaN(f.decimals)
    ? f.decimals
    : undefined
  const common = {
    type:        f.type,
    title:       f.title,
    description: f.description,
    datasource:  f.datasource,
    query:       f.query,
    refresh:     f.refresh || undefined,
  }
  switch (f.type) {
    case "stat":
      return {
        ...common,
        reduce: f.reduce,
        decimals,
        unit:   f.unit,
      }
    case "status":
      return {
        ...common,
        reduce:     f.reduce,
        thresholds: f.thresholds,
      }
    default:
      return {
        ...common,
        range:   f.range,
        step:    f.step || undefined,
        style:   f.style,
        minZero: f.minZero,
        decimals,
        unit:    f.unit,
        legend:  f.legend,
      }
  }
}

const error = (path: string) =>
  errors.value[path] && t(`error.${errors.value[path]}`)

async function save() {
  saving.value = true
  errors.value = {}
  try {
    const path = `/api/admin/sites/${siteId}/panels`
    form.value = {
      thresholds: [],
      ...panelId
        ? await api<Panel>("PUT", `${path}/${panelId}`, body())
        : await api<Panel>("POST", path, body()),
    }
    saved.value = JSON.stringify(form.value)
    notices.success(t("panelEditor.saved"))
    await router.push(`/sites/${siteId}`)
  } catch(err) {
    errors.value = fieldErrors(err)
    notices.failure(t("toast.saveFailed"), err)
  } finally {
    saving.value = false
  }
}

async function remove() {
  const title = resolveText(form.value.title, locale.value, languages.value)
  if (!await confirm({
    title:   t("panelEditor.deleteTitle"),
    message: t("panelEditor.deleteMessage", { title }),
    confirm: t("panelEditor.delete"),
    danger:  true,
  })) {
    return
  }

  try {
    await api("DELETE", `/api/admin/sites/${siteId}/panels/${panelId}`)
    saved.value = ""
    await router.push(`/sites/${siteId}`)
  } catch(err) {
    notices.failure(t("toast.deleteFailed"), err)
  }
}

// Previews evaluate the form as it is, 500ms after each change while open.
const showData = ref(false)
const showWidget = ref(false)
const previewLang = ref(bootstrap.primary)
const preview = ref<{
  data?:    PanelData["data"]
  warnings: Warning[]
  error?:   string
} | null>(null)
const previewError = ref("")
let previewSeq = 0

async function runPreview() {
  if (!showData.value && !showWidget.value) {
    preview.value = null
    previewError.value = ""
    return
  }

  const seq = ++previewSeq
  try {
    const path = `/api/admin/sites/${siteId}/panel-preview`
    const res = await api<NonNullable<typeof preview.value>>("POST", path, {
      panel: body(),
      lang:  previewLang.value,
    })
    if (seq === previewSeq) {
      preview.value = res
      previewError.value = res.error ?? ""
    }
  } catch(err) {
    if (seq === previewSeq) {
      preview.value = null
      const fields = Object.entries(fieldErrors(err))
        .map(([path, code]) => `${path}: ${t(`error.${code}`)}`)
      previewError.value = fields.length
        ? fields.join("; ")
        : t(`error.${errorCode(err)}`)
    }
  }
}

watchDebounced(() => [form.value, previewLang.value], runPreview, 500)
watch([showData, showWidget], runPreview)

/** widget is the panel as the public page presents it in the preview language. */
const widget = computed<PanelInfo>(() => {
  const f = form.value
  const text = (v?: Record<string, string>) =>
    resolveText(v, previewLang.value, languages.value)
  const decimals = typeof f.decimals === "number" && !Number.isNaN(f.decimals)
    ? f.decimals
    : f.type === "timeseries" ? 2 : 0
  return {
    id:          "preview",
    type:        f.type,
    title:       text(f.title),
    description: text(f.description),
    unit:        text(f.unit),
    decimals,
    style:       f.style,
    minZero:     f.minZero,
    range:       parseDuration(f.range ?? "") ?? undefined,
  }
})
const widgetData = computed<PanelData>(() => ({
  state: "fresh",
  data:  preview.value?.data,
}))
</script>

<template>
  <h1 class="title">
    {{ panelId ? t("panelEditor.edit") : t("panelEditor.new") }}
  </h1>
  <form
      v-if="site"
      class="sr-form"
      @submit.prevent="save"
  >
    <SRField
        v-slot="{ id, describedby, invalid }"
        :label="t('panelEditor.dataSource')"
        :error="error('datasource')"
    >
      <div class="select">
        <select
            :id
            v-model="form.datasource"
            required
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
          <option
              value=""
              disabled
          >
            {{ t("panelEditor.chooseDataSource") }}
          </option>
          <option
              v-for="ds in sources"
              :key="ds.id"
              :value="ds.id"
          >
            {{ ds.name }}
          </option>
        </select>
      </div>
    </SRField>
    <SRField
        v-slot="{ id, describedby, invalid }"
        :label="t('panelEditor.type')"
        :error="error('type')"
    >
      <div class="select">
        <select
            :id
            v-model="form.type"
            required
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
          <option
              value=""
              disabled
          >
            {{ t("panelEditor.chooseType") }}
          </option>
          <option
              v-for="type in panelTypes"
              :key="type"
              :value="type"
          >
            {{ t(`panelEditor.types.${type}`) }}
          </option>
        </select>
      </div>
    </SRField>
    <SRLocalizedInput
        v-model="form.title"
        :label="t('panelEditor.title')"
        :languages
        required
        :maxlength="200"
        :errors="localizedErrors(errors, 'title', t)"
    />
    <SRLocalizedInput
        v-model="form.description"
        :label="t('panelEditor.description')"
        :languages
        multiline
        :maxlength="2000"
        :errors="localizedErrors(errors, 'description', t)"
    />
    <SRField
        v-slot="{ id, labelId, describedby, invalid }"
        :label="t('panelEditor.query')"
        :error="error('query')"
    >
      <QueryEditor
          :id
          :key="form.datasource"
          v-model="form.query"
          :editor="sourceType?.editor"
          :datasource="form.datasource"
          :label-id
          :describedby
          :invalid
      />
    </SRField>
    <SRField
        v-slot="{ id, describedby, invalid }"
        :label="t('panelEditor.refresh')"
        :help="t('panelEditor.refreshHelp', { default: bootstrap.defaultRefresh })"
        :error="error('refresh')"
    >
      <input
          :id
          v-model="form.refresh"
          class="input"
          :placeholder="bootstrap.defaultRefresh"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
      >
    </SRField>

    <SRField
        v-if="form.type === 'stat' || form.type === 'status'"
        v-slot="{ id, describedby, invalid }"
        :label="t('panelEditor.reduce')"
        :help="t('panelEditor.reduceHelp')"
        :error="error('reduce')"
    >
      <div class="select">
        <select
            :id
            v-model="form.reduce"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
          <option
              v-for="r in ['first', 'last', 'sum', 'avg', 'min', 'max']"
              :key="r"
              :value="r"
          >
            {{ t(`panelEditor.reducers.${r}`) }}
          </option>
        </select>
      </div>
    </SRField>
    <ThresholdsField
        v-if="form.type === 'status'"
        v-model="form.thresholds!"
        :errors
    />
    <template v-if="form.type === 'timeseries'">
      <SRField
          v-slot="{ id, describedby, invalid }"
          :label="t('panelEditor.range')"
          :help="t('panelEditor.rangeHelp')"
          :error="error('range')"
      >
        <input
            :id
            v-model="form.range"
            class="input"
            required
            placeholder="1h"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
      </SRField>
      <SRField
          v-slot="{ id, describedby, invalid }"
          :label="t('panelEditor.step')"
          :help="t('panelEditor.stepHelp')"
          :error="error('step')"
      >
        <input
            :id
            v-model="form.step"
            class="input"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
      </SRField>
      <SRField
          v-slot="{ id, describedby }"
          :label="t('panelEditor.style')"
      >
        <div class="select">
          <select
              :id
              v-model="form.style"
              :aria-describedby="describedby"
          >
            <option value="line">
              {{ t("panelEditor.styles.line") }}
            </option>
            <option value="area">
              {{ t("panelEditor.styles.area") }}
            </option>
          </select>
        </div>
      </SRField>
      <div class="field">
        <label class="checkbox">
          <input
              v-model="form.minZero"
              type="checkbox"
          >
          {{ t("panelEditor.minZero") }}
        </label>
      </div>
    </template>
    <template v-if="form.type === 'stat' || form.type === 'timeseries'">
      <SRField
          v-slot="{ id, describedby, invalid }"
          :label="t('panelEditor.decimals')"
          :error="error('decimals')"
      >
        <input
            :id
            v-model.number="form.decimals"
            class="input sr-short"
            type="number"
            min="0"
            max="6"
            step="1"
            :placeholder="form.type === 'timeseries' ? '2' : '0'"
            :aria-describedby="describedby"
            :aria-invalid="invalid"
        >
      </SRField>
      <SRLocalizedInput
          v-model="form.unit"
          :label="t('panelEditor.unit')"
          :languages
          :maxlength="32"
          :errors="localizedErrors(errors, 'unit', t)"
      />
    </template>
    <SRLocalizedInput
        v-if="form.type === 'timeseries'"
        v-model="form.legend"
        :label="t('panelEditor.legend')"
        :help="t('panelEditor.legendHelp', { example: '{{ instance }}' })"
        :languages
        :maxlength="200"
        :errors="localizedErrors(errors, 'legend', t)"
    />

    <div
        v-if="previewError"
        class="notification is-danger"
        role="alert"
    >
      {{ t("panelEditor.previewFailed", { error: previewError }) }}
    </div>
    <div class="field is-grouped is-grouped-multiline">
      <SRButton
          type="submit"
          variant="primary"
          :loading="saving"
      >
        {{ t("common.save") }}
      </SRButton>
      <RouterLink
          :to="`/sites/${siteId}`"
          class="button"
      >
        {{ t("common.cancel") }}
      </RouterLink>
      <button
          type="button"
          class="button"
          :class="{ 'is-link is-light': showData }"
          :aria-pressed="showData"
          @click="showData = !showData"
      >
        {{ t("panelEditor.dataPreview") }}
      </button>
      <button
          type="button"
          class="button"
          :class="{ 'is-link is-light': showWidget }"
          :aria-pressed="showWidget"
          @click="showWidget = !showWidget"
      >
        {{ t("panelEditor.widgetPreview") }}
      </button>
      <div
          v-if="(showData || showWidget) && languages.enabled.length > 1"
          class="select"
      >
        <select
            v-model="previewLang"
            :aria-label="t('preview.language')"
        >
          <option
              v-for="l in languages.enabled"
              :key="l"
              :value="l"
              :lang="l"
          >
            {{ endonym(l) }}
          </option>
        </select>
      </div>
      <SRButton
          v-if="panelId"
          variant="danger"
          class="ml-auto"
          @click="remove"
      >
        {{ t("panelEditor.delete") }}
      </SRButton>
    </div>
  </form>
  <section
      v-if="showData && preview?.data"
      class="box"
      :aria-label="t('panelEditor.dataPreview')"
  >
    <p
        v-for="w in preview.warnings"
        :key="w.code"
        class="notification is-warning"
    >
      {{ t(`warnings.${w.code}`, { count: w.count, size: formatMiB(w.size ?? 0, locale) }) }}
    </p>
    <JsonTree
        :value="preview.data"
        name="data"
        open
    />
  </section>
  <section
      v-if="showWidget && preview?.data"
      :aria-label="t('panelEditor.widgetPreview')"
      class="block"
  >
    <StatusTable
        v-if="form.type === 'status'"
        :panels="[widget]"
        :data="{ preview: widgetData }"
        :timezone="site!.timezone"
    />
    <StatPanel
        v-else-if="form.type === 'stat'"
        :panel="widget"
        :data="widgetData"
        :timezone="site!.timezone"
    />
    <TimeseriesPanel
        v-else
        :panel="widget"
        :data="widgetData"
        :timezone="site!.timezone"
    />
  </section>
</template>

<style scoped>
.sr-short {
  width: 8rem;
}
</style>
