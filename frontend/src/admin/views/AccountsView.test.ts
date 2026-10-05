import { createHead } from "@unhead/vue/client"
import { flushPromises, mount } from "@vue/test-utils"
import { createPinia, setActivePinia } from "pinia"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { createAppI18n } from "../../shared/i18n"
import { useOverview } from "../stores/overview"
import { useSession } from "../stores/session"
import type { SiteSummary } from "../types"
import AccountsView from "./AccountsView.vue"

describe("AccountsView", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn(async() => Response.json([{
      id:          "a-2",
      provider:    "test",
      displayName: "Bob",
      createdAt:   "2026-10-01T00:00:00Z",
      pending:     false,
      stale:       false,
    }])))
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("edits an account without roles as having none", async() => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useSession().provider = {
      id:        "test",
      method:    "credentials",
      available: true,
      login:     "email",
    }
    useOverview().sites = [{
      id:        "s-1",
      name:      { en: "Acme" },
      languages: {
        enabled: ["en"],
        primary: "en",
      },
    } as unknown as SiteSummary]

    const wrapper = mount(AccountsView, {
      global: { plugins: [pinia, createAppI18n("en"), createHead()] },
    })
    await flushPromises()
    await wrapper.get("button[aria-label='Roles of Bob']").trigger("click")

    const selects = wrapper.get("form").findAll("select")
    expect(selects).toHaveLength(2)
    for (const select of selects) {
      const el = select.element as HTMLSelectElement
      expect(el.selectedIndex).toBe(0)
      expect(el.selectedOptions[0]?.textContent?.trim()).toBe("No role")
    }
  })
})
