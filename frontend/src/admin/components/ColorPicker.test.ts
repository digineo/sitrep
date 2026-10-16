import { mount } from "@vue/test-utils"
import { afterEach, describe, expect, it } from "vitest"

import { createAppI18n } from "../../shared/i18n"
import ColorPicker from "./ColorPicker.vue"

function setup(color?: string) {
  const wrapper = mount(ColorPicker, {
    props: {
      "modelValue":          color,
      "onUpdate:modelValue": (v?: string) => wrapper.setProps({ modelValue: v }),
      "label":               "Color",
    },
    global:   { plugins: [createAppI18n("en")] },
    attachTo: document.body,
  })
  const trigger = wrapper.get("button")
  return {
    wrapper,
    trigger,
    model: () => wrapper.props("modelValue"),
  }
}

afterEach(() => {
  document.body.innerHTML = ""
})

describe("ColorPicker", () => {
  it("names the chosen color", () => {
    const name = (color?: string) => setup(color).trigger.attributes("aria-label")
    expect(name()).toBe("Color: Automatic")
    expect(name("#a67706")).toBe("Color: Amber")
    expect(name("#ffc65d")).toBe("Color: Amber, light")
    expect(name("#00308d")).toBe("Color: Blue, dark")
    expect(name("#123456")).toBe("Color: Custom, #123456")
  })

  it("chooses swatches, the automatic and a custom color", async() => {
    const { wrapper, model } = setup()
    await wrapper.get("input[aria-label='Blue, dark']").setValue(true)
    expect(model()).toBe("#00308d")

    await wrapper.findAll("input[type=radio]")[0]!.setValue(true)
    expect(model()).toBeUndefined()

    await wrapper.get("input[type=color]").setValue("#123456")
    expect(model()).toBe("#123456")
    expect(wrapper.get("input[type=color]").classes()).toContain("is-checked")
  })

  it("opens on the chosen color and closes with Escape", async() => {
    const { wrapper, trigger } = setup("#058da0")
    await trigger.trigger("click")
    expect(wrapper.classes()).toContain("is-active")
    expect(trigger.attributes("aria-expanded")).toBe("true")
    expect(document.activeElement?.getAttribute("aria-label")).toBe("Cyan")

    await wrapper.get("input[aria-label='Cyan']").trigger("keydown", { key: "Escape" })
    expect(wrapper.classes()).not.toContain("is-active")
    expect(document.activeElement).toBe(trigger.element)
  })
})
