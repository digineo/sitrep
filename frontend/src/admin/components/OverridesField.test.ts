import { mount, type VueWrapper } from "@vue/test-utils"
import { describe, expect, it } from "vitest"

import { createAppI18n } from "../../shared/i18n"
import type { Override } from "../types"
import OverridesField from "./OverridesField.vue"

const unit = { en: "ms" }

const override = (label: string, value: string): Override => ({
  label,
  value,
  decimals: 2,
  axis:     "left",
})

function setup(rows: Override[], errors: Record<string, string> = {}) {
  const wrapper = mount(OverridesField, {
    props: {
      "modelValue":          rows,
      "onUpdate:modelValue": (v: Override[]) => wrapper.setProps({ modelValue: v }),
      errors,
      "languages":           {
        enabled: ["en", "de"],
        primary: "en",
      },
      unit,
      "decimals": 0,
    },
    global: { plugins: [createAppI18n("en")] },
  })
  const model = () => wrapper.props("modelValue")
  return {
    wrapper,
    model,
  }
}

/** control returns the i-th control labelled text. */
function control(wrapper: VueWrapper, text: string, i = 0) {
  const label = wrapper.findAll("label").filter(l => l.text() === text)[i]!
  return wrapper.get(`[id="${label.attributes("for")}"]`)
}

const button = (wrapper: VueWrapper, name: string) =>
  wrapper.get(`button[aria-label="${name}"]`)

describe("OverridesField", () => {
  it("adds overrides with the panel's unit and decimals", async() => {
    const { wrapper, model } = setup([])
    const add = wrapper.findAll("button").find(b => b.text() === "Add override")!
    await add.trigger("click")
    expect(model()).toEqual([{
      label:    "",
      value:    "",
      unit:     { en: "ms" },
      decimals: 0,
      axis:     "left",
    }])
  })

  it("edits the fields of an override", async() => {
    const { wrapper, model } = setup([override("job", "api"), override("job", "web")])
    await control(wrapper, "Label", 1).setValue("region")
    await control(wrapper, "Value", 1).setValue("eu")
    await control(wrapper, "Decimals", 1).setValue("4")
    await control(wrapper, "Axis", 1).setValue("right")
    await wrapper.findAll("input[lang=de]")[1]!.setValue("Anfragen")
    await wrapper.findAll("input[aria-label='Blue, light']")[1]!.setValue(true)
    expect(model()).toEqual([
      override("job", "api"),
      {
        label:    "region",
        value:    "eu",
        unit:     { de: "Anfragen" },
        decimals: 4,
        axis:     "right",
        color:    "#b6d0ff",
      },
    ])
  })

  it("moves and removes overrides", async() => {
    const { wrapper, model } = setup([override("job", "a"), override("job", "b")])
    expect(button(wrapper, "Move override 1 up").attributes("disabled")).toBeDefined()
    expect(button(wrapper, "Move override 2 down").attributes("disabled")).toBeDefined()

    await button(wrapper, "Move override 1 down").trigger("click")
    expect(model().map(o => o.value)).toEqual(["b", "a"])

    await button(wrapper, "Remove override 1").trigger("click")
    expect(model().map(o => o.value)).toEqual(["a"])
  })

  it("marks the fields with errors", () => {
    const { wrapper } = setup([override("1job", "x")], {
      "overrides[0].label":   "invalid_value",
      "overrides[0].unit.de": "too_long",
    })
    expect(control(wrapper, "Label").attributes("aria-invalid")).toBe("true")
    expect(control(wrapper, "Value").attributes("aria-invalid")).toBeUndefined()
    expect(wrapper.get("input[lang=de]").attributes("aria-invalid")).toBe("true")
  })

  it("offers at most 20 overrides", () => {
    const { wrapper } = setup(Array.from({ length: 20 }, (_, i) => override("job", `${i}`)))
    expect(wrapper.findAll("button").some(b => b.text() === "Add override")).toBe(false)
  })
})
