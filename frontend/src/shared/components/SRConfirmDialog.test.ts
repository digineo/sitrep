import { flushPromises, mount } from "@vue/test-utils"
import { afterEach, describe, expect, it } from "vitest"

import { useConfirm } from "../composables/useConfirm"
import { createAppI18n } from "../i18n"
import SRConfirmDialog from "./SRConfirmDialog.vue"

async function open() {
  const wrapper = mount(SRConfirmDialog, {
    global:   { plugins: [createAppI18n("en")] },
    attachTo: document.body,
  })

  const trigger = document.createElement("button")
  document.body.append(trigger)
  trigger.focus()

  const answer = useConfirm().confirm({
    title:   "Delete status page?",
    message: "It cannot be restored.",
    confirm: "Delete status page",
    danger:  true,
  })
  await flushPromises()
  return {
    wrapper,
    dialog: wrapper.get("dialog"),
    trigger,
    answer,
  }
}

describe("SRConfirmDialog", () => {
  afterEach(() => {
    document.body.innerHTML = ""
  })

  it("shows the options with the focus on cancel", async() => {
    const { dialog } = await open()
    expect(dialog.element.open).toBe(true)
    expect(dialog.text()).toContain("Delete status page?")
    expect(dialog.text()).toContain("It cannot be restored.")
    const [cancel, ok] = dialog.findAll("button")
    expect(cancel!.text()).toBe("Cancel")
    expect(document.activeElement).toBe(cancel!.element)
    expect(ok!.text()).toBe("Delete status page")
    expect(ok!.classes()).toContain("is-danger")
  })

  it("resolves true on confirm and returns the focus", async() => {
    const { dialog, trigger, answer } = await open()
    await dialog.findAll("button")[1]!.trigger("click")
    await expect(answer).resolves.toBe(true)
    expect(dialog.element.open).toBe(false)
    expect(document.activeElement).toBe(trigger)
  })

  it("resolves false on Escape", async() => {
    const { dialog, trigger, answer } = await open()
    dialog.element.dispatchEvent(new Event("cancel", { cancelable: true }))
    await expect(answer).resolves.toBe(false)
    expect(document.activeElement).toBe(trigger)
  })

  it("resolves false on a backdrop click", async() => {
    const { dialog, answer } = await open()
    await dialog.trigger("click")
    await expect(answer).resolves.toBe(false)
  })
})
