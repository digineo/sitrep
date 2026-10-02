import { mount } from "@vue/test-utils"
import { describe, expect, it } from "vitest"

import { createAppI18n } from "../../shared/i18n"
import type { Languages } from "../types"
import LanguagesField from "./LanguagesField.vue"

function setup(languages: Languages) {
  const wrapper = mount(LanguagesField, {
    props: {
      "modelValue":          languages,
      "onUpdate:modelValue": (v: Languages) => wrapper.setProps({ modelValue: v }),
    },
    global: { plugins: [createAppI18n("en")] },
  })
  const model = () => wrapper.props("modelValue")
  return {
    wrapper,
    model,
  }
}

describe("LanguagesField", () => {
  it("lists enabled languages first, in their order", () => {
    const { wrapper } = setup({
      enabled: ["en"],
      primary: "en",
    })
    const labels = wrapper.findAll("li").map(li => li.text())
    expect(labels).toEqual(["English", "Deutsch"])
  })

  it("enables and disables languages", async() => {
    const { wrapper, model } = setup({
      enabled: ["en"],
      primary: "en",
    })
    await wrapper.findAll("input[type=checkbox]")[1]!.setValue(true)
    expect(model()).toEqual({
      enabled: ["en", "de"],
      primary: "en",
    })

    await wrapper.findAll("input[type=checkbox]")[0]!.setValue(false)
    // the primary language moves to an enabled one
    expect(model()).toEqual({
      enabled: ["de"],
      primary: "de",
    })
  })

  it("keeps the last enabled language", () => {
    const { wrapper } = setup({
      enabled: ["de"],
      primary: "de",
    })
    const [de, en] = wrapper.findAll("input[type=checkbox]")
    expect(de!.attributes("disabled")).toBeDefined()
    expect(en!.attributes("disabled")).toBeUndefined()
  })

  it("reorders enabled languages", async() => {
    const { wrapper, model } = setup({
      enabled: ["en", "de"],
      primary: "en",
    })
    const down = wrapper.get("button[aria-label='Move English down']")
    const up = wrapper.get("button[aria-label='Move English up']")
    expect(up.attributes("disabled")).toBeDefined()

    await down.trigger("click")
    expect(model()).toEqual({
      enabled: ["de", "en"],
      primary: "en",
    })
  })

  it("offers only enabled languages as primary", async() => {
    const { wrapper, model } = setup({
      enabled: ["en", "de"],
      primary: "en",
    })
    await wrapper.get("select").setValue("de")
    expect(model().primary).toBe("de")

    await wrapper.setProps({
      modelValue: {
        enabled: ["de"],
        primary: "de",
      },
    })
    expect(wrapper.findAll("option").map(o => o.text())).toEqual(["Deutsch"])
  })

  it("shows errors", () => {
    const wrapper = mount(LanguagesField, {
      props: {
        modelValue: {
          enabled: ["en"],
          primary: "en",
        },
        errors: { primary: "primary_not_enabled" },
      },
      global: { plugins: [createAppI18n("en")] },
    })
    expect(wrapper.text()).toContain("The primary language must be enabled.")
    expect(wrapper.get("select").attributes("aria-invalid")).toBe("true")
  })
})
