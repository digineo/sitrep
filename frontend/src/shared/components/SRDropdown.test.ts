import { mount } from "@vue/test-utils"
import { afterEach, describe, expect, it } from "vitest"
import { nextTick } from "vue"

import SRDropdown from "./SRDropdown.vue"

function setup() {
  const wrapper = mount(SRDropdown, {
    props: {
      "label": "Color scheme: Dark",
      "items": [
        { value: "light", label: "Light" },
        { value: "dark", label: "Dark" },
        { value: "system", label: "System" },
      ],
      "modelValue":          "dark",
      "onUpdate:modelValue": (v: string) => wrapper.setProps({ modelValue: v }),
    },
    attachTo: document.body,
  })
  return wrapper
}

describe("SRDropdown", () => {
  afterEach(() => {
    document.body.innerHTML = ""
  })

  it("shows the selected item and marks it checked", () => {
    const wrapper = setup()
    const trigger = wrapper.get("button[aria-haspopup=menu]")
    expect(trigger.text()).toBe("Dark")
    expect(trigger.attributes("aria-label")).toBe("Color scheme: Dark")
    expect(trigger.attributes("aria-expanded")).toBe("false")
    const checked = wrapper.findAll("[role=menuitemradio]")
      .map(i => i.attributes("aria-checked"))
    expect(checked).toEqual(["false", "true", "false"])
  })

  it("opens on click with the focus on the selected item", async() => {
    const wrapper = setup()
    await wrapper.get("button[aria-haspopup=menu]").trigger("click")
    await nextTick()
    expect(wrapper.get("button[aria-haspopup=menu]").attributes("aria-expanded"))
      .toBe("true")
    expect(document.activeElement?.textContent?.trim()).toBe("Dark")
  })

  it("moves with arrow keys, selects with a click and returns the focus", async() => {
    const wrapper = setup()
    const trigger = wrapper.get("button[aria-haspopup=menu]")
    await trigger.trigger("keydown", { key: "ArrowDown" })
    await nextTick()
    expect(document.activeElement?.textContent?.trim()).toBe("Light")

    await wrapper.get("[role=menu]").trigger("keydown", { key: "ArrowUp" })
    expect(document.activeElement?.textContent?.trim()).toBe("System")

    await wrapper.get("[role=menu]").trigger("keydown", { key: "ArrowDown" })
    expect(document.activeElement?.textContent?.trim()).toBe("Light")

    ;(document.activeElement as HTMLButtonElement).click()
    await nextTick()
    expect(wrapper.props("modelValue")).toBe("light")
    expect(trigger.attributes("aria-expanded")).toBe("false")
    expect(document.activeElement).toBe(trigger.element)
  })

  it("closes on Escape without selecting", async() => {
    const wrapper = setup()
    const trigger = wrapper.get("button[aria-haspopup=menu]")
    await trigger.trigger("keydown", { key: "ArrowUp" })
    await nextTick()

    await wrapper.get("[role=menu]").trigger("keydown", { key: "Escape" })
    expect(trigger.attributes("aria-expanded")).toBe("false")
    expect(document.activeElement).toBe(trigger.element)
    expect(wrapper.props("modelValue")).toBe("dark")
  })
})
