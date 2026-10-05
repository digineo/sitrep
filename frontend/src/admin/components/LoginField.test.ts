import { mount } from "@vue/test-utils"
import { createPinia, setActivePinia } from "pinia"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { createAppI18n } from "../../shared/i18n"
import { useSession } from "../stores/session"
import LoginField from "./LoginField.vue"

function setup(login: "email" | "username", email?: string) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const session = useSession()
  session.provider = {
    id:        "test",
    method:    "redirect",
    available: true,
    login,
  }
  session.user = {
    id:          "a-1",
    displayName: "Ann",
    email,
    role:        "owner",
    sites:       {},
  }

  const wrapper = mount(LoginField, {
    props: {
      "modelValue":          "",
      "onUpdate:modelValue": (v: string) => wrapper.setProps({ modelValue: v }),
    },
    global: { plugins: [pinia, createAppI18n("en")] },
  })
  return wrapper
}

describe("LoginField", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(Response.json(["ann", "bob"]))))
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("warns about another email domain than the user's", async() => {
    const wrapper = setup("email", "ann@example.com")
    const input = wrapper.find("input")
    expect(input.attributes("type")).toBe("email")

    await input.setValue("bob@Example.com")
    expect(wrapper.find(".help").text()).not.toContain("example.com")

    await input.setValue("bob@example.org")
    expect(wrapper.find(".help").text()).toContain("not at example.com")
  })

  it("does not warn without an own email", async() => {
    const wrapper = setup("email")
    await wrapper.find("input").setValue("bob@example.org")
    expect(wrapper.find(".help").text()).not.toContain("not at")
  })

  it("suggests the provider's users", async() => {
    const wrapper = setup("username")
    await vi.waitFor(() => {
      expect(wrapper.findAll("datalist option").map(o => o.attributes("value")))
        .toEqual(["ann", "bob"])
    })
    expect(fetch).toHaveBeenCalledWith("/api/admin/directory", expect.anything())
    expect(wrapper.find("input").attributes("list"))
      .toBe(wrapper.find("datalist").attributes("id"))
  })
})
