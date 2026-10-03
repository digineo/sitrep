<script setup lang="ts">
import { Plus } from "@lucide/vue"
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"

import SRTag from "../../shared/components/SRTag.vue"
import { api } from "../api"
import ConnectionTest from "../components/ConnectionTest.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { useNotices } from "../stores/notices"
import type { DataSource } from "../types"

const { t, locale } = useI18n()
const notices = useNotices()
usePageTitle(() => t("dataSources.title"))

const list = ref<DataSource[] | null>(null)
const sorted = computed(() => {
  const collator = new Intl.Collator(locale.value)
  return [...list.value ?? []].sort((a, b) => collator.compare(a.name, b.name))
})

onMounted(async() => {
  try {
    list.value = await api<DataSource[]>("GET", "/api/admin/datasources")
  } catch(err) {
    notices.loadFailed(err)
  }
})
</script>

<template>
  <div class="level">
    <h1 class="title mb-0">
      {{ t("dataSources.title") }}
    </h1>
    <RouterLink
        to="/datasources/new"
        class="button is-primary"
    >
      <span class="icon"><Plus aria-hidden="true" /></span>
      <span>{{ t("dataSources.new") }}</span>
    </RouterLink>
  </div>
  <p
      v-if="list?.length === 0"
      class="box"
  >
    {{ t("dataSources.empty") }}
  </p>
  <div
      v-else-if="list"
      class="table-container"
  >
    <table class="table is-fullwidth">
      <thead>
        <tr>
          <th>{{ t("dataSources.name") }}</th>
          <th>{{ t("dataSources.type") }}</th>
          <th>{{ t("dataSources.summary") }}</th>
          <th>{{ t("dataSources.panels") }}</th>
          <th><span class="sr-visually-hidden">{{ t("dataSources.test") }}</span></th>
        </tr>
      </thead>
      <tbody>
        <tr
            v-for="ds in sorted"
            :key="ds.id"
        >
          <td>
            <RouterLink :to="`/datasources/${ds.id}`">
              {{ ds.name }}
            </RouterLink>
            <SRTag
                v-if="!ds.usable"
                color="danger"
                class="ml-2"
            >
              {{ t("dataSources.unusable") }}
            </SRTag>
          </td>
          <td>{{ t(`datasource.${ds.type}.name`) }}</td>
          <td><code>{{ ds.summary }}</code></td>
          <td>{{ ds.panels }}</td>
          <td><ConnectionTest :path="`/api/admin/datasources/${ds.id}/test`" /></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
