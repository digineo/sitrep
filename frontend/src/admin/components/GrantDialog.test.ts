import { flushPromises, mount } from "@vue/test-utils"
import { createPinia, setActivePinia } from "pinia"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { createAppI18n } from "../../shared/i18n"
import { useSession } from "../stores/session"
import GrantDialog from "./GrantDialog.vue"

function setup() {
  const pinia = createPinia()
  setActivePinia(pinia)
  useSession().provider = {
    id:        "test",
    method:    "redirect",
    available: true,
    login:     "email",
  }

  const wrapper = mount(GrantDialog, {
    props: {
      title:     "Add member",
      path:      "/api/admin/sites/s-1/members",
      roles:     ["responder", "maintainer"],
      role:      "responder",
      roleLabel: "Role",
      roleHelp:  "",
      success:   "Member added.",
    },
    global:   { plugins: [pinia, createAppI18n("en")] },
    attachTo: document.body,
  })
  const dialog = wrapper.get("dialog")
  return {
    wrapper,
    dialog,
    async open() {
      await wrapper.get("button").trigger("click")
      expect(dialog.element.open).toBe(true)
    },
  }
}

function answer(status: number, body: unknown) {
  vi.mocked(fetch).mockResolvedValueOnce(Response.json(body, { status }))
}

describe("GrantDialog", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    document.body.innerHTML = ""
  })

  it("resets the form when cancelled", async() => {
    const { dialog, open } = setup()
    await open()
    await dialog.get("input").setValue("bob@example.com")
    await dialog.get("select").setValue("maintainer")
    answer(409, { error: { code: "own_account" } })
    await dialog.get("form").trigger("submit")
    await flushPromises()
    expect(dialog.get("[role=alert]").text())
      .toBe("You cannot change your own roles or delete your own account.")

    await dialog.findAll("button").find(b => b.text() === "Cancel")!.trigger("click")
    expect(dialog.element.open).toBe(false)
    await open()
    expect((dialog.get("input").element as HTMLInputElement).value).toBe("")
    expect((dialog.get("select").element as HTMLSelectElement).value).toBe("responder")
    expect(dialog.find("[role=alert]").exists()).toBe(false)
  })

  it("shows field errors and closes once added", async() => {
    const { wrapper, dialog, open } = setup()
    await open()
    await dialog.get("input").setValue("bob")
    answer(400, {
      error: {
        code:   "invalid",
        fields: [{
          path: "login",
          code: "invalid_email",
        }],
      },
    })
    await dialog.get("form").trigger("submit")
    await flushPromises()
    expect(dialog.text()).toContain("Enter a valid email address.")
    expect(dialog.find("[role=alert]").exists()).toBe(false)

    await dialog.get("input").setValue("bob@example.com")
    answer(200, {})
    await dialog.get("form").trigger("submit")
    await flushPromises()
    expect(fetch).toHaveBeenLastCalledWith(
      "/api/admin/sites/s-1/members",
      expect.objectContaining({
        body: JSON.stringify({
          login: "bob@example.com",
          role:  "responder",
        }),
      }),
    )
    expect(dialog.element.open).toBe(false)
    expect(wrapper.emitted("added")).toHaveLength(1)
    expect((dialog.get("input").element as HTMLInputElement).value).toBe("")
  })
})
