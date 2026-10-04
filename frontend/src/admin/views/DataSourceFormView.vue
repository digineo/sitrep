<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import { useConfirm } from "../../shared/composables/useConfirm"
import { api, ApiError, fieldErrors } from "../api"
import ConfigFields, { type SecretState } from "../components/ConfigFields.vue"
import ConnectionTest from "../components/ConnectionTest.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useUnsavedChanges } from "../composables/useUnsavedChanges"
import { resolveText } from "../rules"
import { useNotices } from "../stores/notices"
import { useOverview } from "../stores/overview"
import type { DataSource, DataSourceType } from "../types"

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const notices = useNotices()
const overview = useOverview()
const { confirm } = useConfirm()

const id = computed(() => route.params.id as string | undefined)
usePageTitle(() => id.value ? t("dataSources.edit") : t("dataSources.new"))

const types = ref<DataSourceType[]>([])
const stored = ref<DataSource | null>(null)
const typeId = ref("")
const name = ref("")
const values = ref<Record<string, string>>({})
const secrets = ref<Record<string, SecretState>>({})
const errors = ref<Record<string, string>>({})
const saving = ref(false)
const saved = ref("")
/** inUse lists the sites whose panels block a deletion. */
const inUse = ref<{
  id:   string
  name: string
}[]>([])

const type = computed(() => types.value.find(ty => ty.id === typeId.value))
const snapshot = () =>
  JSON.stringify([typeId.value, name.value, values.value, secrets.value])
useUnsavedChanges(() => saved.value !== "" && snapshot() !== saved.value)

/** reset fills the form from the stored data source, or the type's defaults. */
function reset() {
  name.value = stored.value?.name ?? name.value
  values.value = Object.fromEntries((type.value?.fields ?? [])
    .filter(f => f.kind !== "secret")
    .map(f => [f.name, stored.value?.config[f.name] ?? f.default ?? ""]))
  secrets.value = {}
  saved.value = snapshot()
}

onMounted(async() => {
  try {
    types.value = await api<DataSourceType[]>("GET", "/api/admin/datasource-types")

    if (id.value) {
      stored.value = await api<DataSource>(
        "GET",
        `/api/admin/datasources/${id.value}`,
      )
      typeId.value = stored.value.type
    } else {
      typeId.value = types.value[0]?.id ?? ""
    }

    reset()
  } catch(err) {
    notices.loadFailed(err)
  }
})

/**
 * config returns the submitted configuration: omitted secrets keep their
 * value, empty ones are cleared.
 */
function config(): Record<string, string> {
  const out: Record<string, string> = {}
  for (const f of type.value?.fields ?? []) {
    if (f.when && values.value[f.when.field] !== f.when.value) {
      continue
    }

    const s = secrets.value[f.name]
    if (f.kind !== "secret") {
      out[f.name] = values.value[f.name] ?? ""
    } else if (s?.action === "clear") {
      out[f.name] = ""
    } else if (s?.action === "replace" && s.value) {
      out[f.name] = s.value
    }
  }
  return out
}

const configErrors = computed(() => Object.fromEntries(Object.entries(errors.value)
  .filter(([path]) => path.startsWith("config."))
  .map(([path, code]) => [path.slice("config.".length), code])))

async function save() {
  saving.value = true
  errors.value = {}
  try {
    const body = {
      name:   name.value,
      type:   typeId.value,
      config: config(),
    }
    stored.value = id.value
      ? await api<DataSource>("PUT", `/api/admin/datasources/${id.value}`, body)
      : await api<DataSource>("POST", "/api/admin/datasources", body)
    reset()
    notices.success(t("dataSources.saved"))
    if (!id.value) {
      await router.push("/datasources")
    }
  } catch(err) {
    errors.value = fieldErrors(err)
    notices.failure(t("toast.saveFailed"), err)
  } finally {
    saving.value = false
  }
}

async function remove() {
  const ok = await confirm({
    title:   t("dataSources.deleteTitle"),
    message: t("dataSources.deleteMessage", { name: stored.value!.name }),
    confirm: t("dataSources.delete"),
    danger:  true,
  })
  if (!ok) {
    return
  }

  inUse.value = []
  try {
    await api("DELETE", `/api/admin/datasources/${id.value}`)
    saved.value = ""
    await router.push("/datasources")
  } catch(err) {
    if (err instanceof ApiError && err.code === "datasource_in_use") {
      const sites = (err.details as { sites: string[] }).sites
      inUse.value = sites.map((site) => {
        const s = overview.sites?.find(x => x.id === site)
        return {
          id:   site,
          name: s ? resolveText(s.name, locale.value, s.languages) : site,
        }
      })
    }

    notices.failure(t("toast.deleteFailed"), err)
  }
}
</script>

<template>
  <h1 class="title">
    {{ id ? t("dataSources.edit") : t("dataSources.new") }}
  </h1>
  <form
      v-if="type"
      class="sr-form"
      @submit.prevent="save"
  >
    <SRField
        v-slot="{ id: fieldId, describedby }"
        :label="t('dataSources.type')"
    >
      <div
          v-if="!id"
          class="select"
      >
        <select
            :id="fieldId"
            v-model="typeId"
            :aria-describedby="describedby"
            @change="reset"
        >
          <option
              v-for="ty in types"
              :key="ty.id"
              :value="ty.id"
          >
            {{ t(`datasource.${ty.id}.name`) }}
          </option>
        </select>
      </div>
      <input
          v-else
          :id="fieldId"
          class="input is-static"
          readonly
          :value="t(`datasource.${type.id}.name`)"
      >
    </SRField>
    <SRField
        v-slot="{ id: fieldId, describedby, invalid }"
        :label="t('dataSources.name')"
        :help="t('dataSources.nameHelp')"
        :error="errors.name && t(`error.${errors.name}`)"
    >
      <input
          :id="fieldId"
          v-model="name"
          class="input"
          required
          maxlength="100"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
      >
    </SRField>
    <ConfigFields
        v-model:values="values"
        v-model:secrets="secrets"
        :type
        :stored="stored ?? undefined"
        :errors="configErrors"
    />
    <div
        v-if="inUse.length"
        class="notification is-danger"
        role="alert"
    >
      <p>{{ t("dataSources.inUse") }}</p>
      <ul>
        <li
            v-for="site in inUse"
            :key="site.id"
        >
          <RouterLink :to="`/sites/${site.id}`">
            {{ site.name }}
          </RouterLink>
        </li>
      </ul>
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
          to="/datasources"
          class="button"
      >
        {{ t("common.cancel") }}
      </RouterLink>
      <ConnectionTest
          path="/api/admin/datasources/test"
          :body="() => ({ id, type: typeId, config: config() })"
      />
      <SRButton
          v-if="id"
          variant="danger"
          class="ml-auto"
          @click="remove"
      >
        {{ t("dataSources.delete") }}
      </SRButton>
    </div>
  </form>
</template>
