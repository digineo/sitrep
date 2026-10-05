import { defineStore } from "pinia"
import { ref } from "vue"

import { api } from "../api"
import type { DataSource, SiteSummary } from "../types"
import { useNotices } from "./notices"
import { useSession } from "./session"

/**
 * useOverview holds the user's sites for the sidebar and home view, and
 * for admins the data sources.
 */
export const useOverview = defineStore("overview", () => {
  const notices = useNotices()
  const session = useSession()
  const sites = ref<SiteSummary[] | null>(null)
  const dataSources = ref<DataSource[] | null>(null)
  let failed = false

  async function refresh() {
    try {
      // The roles first, so they match the sites listed for them.
      await session.load()
      ;[sites.value, dataSources.value] = await Promise.all([
        api<SiteSummary[]>("GET", "/api/admin/sites"),
        session.can("", "admin")
          ? api<DataSource[]>("GET", "/api/admin/datasources")
          : null,
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
