import { mount } from "@vue/test-utils"
import { describe, expect, it } from "vitest"

import { createAppI18n } from "../i18n"
import StatusBanner from "./StatusBanner.vue"

describe("StatusBanner", () => {
  it.each([
    ["operational", "is-success", "All systems operational"],
    ["degraded", "is-warning", "Some systems are degraded"],
    ["down", "is-danger", "Major outage"],
    ["unknown", "sr-is-unknown", "Status not fully available"],
  ] as const)("shows %s", (status, color, text) => {
    const wrapper = mount(StatusBanner, {
      props:  { status },
      global: { plugins: [createAppI18n("en")] },
    })
    const banner = wrapper.get("[role=status]")
    expect(banner.classes()).toContain(color)
    expect(banner.text()).toBe(text)
    expect(banner.get("svg").attributes("aria-hidden")).toBe("true")
  })

  it("speaks the active language", () => {
    const wrapper = mount(StatusBanner, {
      props:  { status: "down" },
      global: { plugins: [createAppI18n("de")] },
    })
    expect(wrapper.text()).toBe("Größere Störung")
  })
})
