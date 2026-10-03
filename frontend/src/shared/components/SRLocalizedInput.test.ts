import { mount } from "@vue/test-utils"
import { describe, expect, it } from "vitest"
import { nextTick } from "vue"

import { createAppI18n } from "../i18n"
import SRLocalizedInput from "./SRLocalizedInput.vue"

describe("SRLocalizedInput", () => {
  it("shows one input per enabled language, the primary first", async() => {
    const wrapper = mount(SRLocalizedInput, {
      props: {
        label:     "Name",
        languages: {
          enabled: ["en", "de"],
          primary: "en",
        },
        modelValue: { en: "Shop" },
      },
      global:   { plugins: [createAppI18n("en")] },
      attachTo: document.body,
    })
    const inputs = wrapper.findAll("input")
    expect(inputs).toHaveLength(2)
    expect(inputs[0]!.element.value).toBe("Shop")
    expect(inputs[0]!.isVisible()).toBe(true)
    expect(inputs[1]!.isVisible()).toBe(false)
    const tabs = wrapper.findAll("[role=tab]")
    const labels = tabs.map(t => t.attributes("aria-label"))
    expect(labels).toEqual(["English, Primary", "Deutsch, Missing"])
    expect(inputs[0]!.attributes("aria-labelledby"))
      .toContain(tabs[0]!.attributes("id"))

    await tabs[1]!.trigger("click")
    expect(inputs[1]!.isVisible()).toBe(true)
    expect(tabs[1]!.attributes("aria-selected")).toBe("true")
    wrapper.unmount()
  })

  it("marks no language as missing while the text is empty", () => {
    const wrapper = mount(SRLocalizedInput, {
      props: {
        label:     "Description",
        languages: {
          enabled: ["en", "de"],
          primary: "en",
        },
        modelValue: undefined,
      },
      global: { plugins: [createAppI18n("en")] },
    })
    const labels = wrapper.findAll("[role=tab]")
      .map(t => t.attributes("aria-label"))
    expect(labels).toEqual(["English, Primary", "Deutsch"])
  })

  it("marks a blank primary language as missing while another one has a value", () => {
    const wrapper = mount(SRLocalizedInput, {
      props: {
        label:     "Description",
        languages: {
          enabled: ["en", "de"],
          primary: "en",
        },
        modelValue: { de: "Beschreibung" },
      },
      global: { plugins: [createAppI18n("en")] },
    })
    const labels = wrapper.findAll("[role=tab]")
      .map(t => t.attributes("aria-label"))
    expect(labels).toEqual(["English, Primary, Missing", "Deutsch"])
  })

  it("keeps values of languages that are not enabled", async() => {
    const updates: Record<string, string>[] = []
    const wrapper = mount(SRLocalizedInput, {
      props: {
        "label":     "Name",
        "languages": {
          enabled: ["en", "de"],
          primary: "en",
        },
        "modelValue": {
          en: "Shop",
          fr: "Boutique",
        },
        "onUpdate:modelValue": (v: Record<string, string> | undefined) =>
          updates.push(v!),
      },
      global: { plugins: [createAppI18n("en")] },
    })
    await wrapper.findAll("input")[1]!.setValue("Laden")
    expect(updates).toEqual([{ en: "Shop", fr: "Boutique", de: "Laden" }])
  })

  it("requires the primary language and shows it when the check fails", async() => {
    const wrapper = mount(SRLocalizedInput, {
      props: {
        label:     "Name",
        languages: {
          enabled: ["en", "de"],
          primary: "de",
        },
        modelValue: {},
        required:   true,
      },
      global:   { plugins: [createAppI18n("en")] },
      attachTo: document.body,
    })
    const inputs = wrapper.findAll("input")
    expect(inputs[0]!.attributes("required")).toBeUndefined()
    expect(inputs[1]!.attributes("required")).toBeDefined()

    await wrapper.findAll("[role=tab]")[0]!.trigger("click")
    expect(inputs[1]!.isVisible()).toBe(false)
    await inputs[1]!.trigger("invalid")
    await nextTick()
    expect(inputs[1]!.isVisible()).toBe(true)
    expect(document.activeElement).toBe(inputs[1]!.element)
    wrapper.unmount()
  })

  it("moves between tabs with the arrow keys", async() => {
    const wrapper = mount(SRLocalizedInput, {
      props: {
        label:     "Name",
        languages: {
          enabled: ["en", "de", "fr"],
          primary: "en",
        },
        modelValue: {},
      },
      global:   { plugins: [createAppI18n("en")] },
      attachTo: document.body,
    })
    const tabs = wrapper.findAll("[role=tab]")
    await tabs[0]!.trigger("keydown", { key: "ArrowLeft" })
    await nextTick()
    expect(document.activeElement).toBe(tabs[2]!.element)
    expect(tabs[2]!.attributes("aria-selected")).toBe("true")
    wrapper.unmount()
  })
})
