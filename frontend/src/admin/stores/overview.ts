import { defineStore } from "pinia"
import { ref } from "vue"

import { api } from "../api"
import type { DataSource, SiteSummary } from "../types"
import { useNotices } from "./notices"

/** useOverview holds the sites and data sources for the sidebar and home view. */
export const useOverview = defineStore("overview", () => {
  const notices = useNotices()
  const sites = ref<SiteSummary[] | null>(null)
  const dataSources = ref<DataSource[] | null>(null)
  let failed = false

  async function refresh() {
    try {
      [sites.value, dataSources.value] = await Promise.all([
        api<SiteSummary[]>("GET", "/api/admin/sites"),
        api<DataSource[]>("GET", "/api/admin/datasources"),
      ])
      if (failed) {
        notices.loadError = null
        failed = false
      }
    } catch(err) {
      failed = true
      notices.loadFailed(err)
    }
  }
  return {
    sites,
    dataSources,
    refresh,
  }
})
