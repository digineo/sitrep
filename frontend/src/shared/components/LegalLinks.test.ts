import { mount, RouterLinkStub } from "@vue/test-utils"
import { describe, expect, it } from "vitest"

import { createAppI18n } from "../i18n"
import LegalLinks from "./LegalLinks.vue"

function render(props: InstanceType<typeof LegalLinks>["$props"]) {
  return mount(LegalLinks, {
    props,
    global: {
      plugins: [createAppI18n("en")],
      stubs:   { RouterLink: RouterLinkStub },
    },
  })
}

describe("LegalLinks", () => {
  it("links the privacy statement, then the imprint", () => {
    const wrapper = render({
      links: {
        imprint: { mode: "text" },
        privacy: {
          mode: "url",
          url:  "https://example.com/privacy",
        },
      },
      path: kind => `/en/${kind}`,
    })
    expect(wrapper.text()).toBe("Privacy statement · Imprint")
    const privacy = wrapper.get("a[href='https://example.com/privacy']")
    expect(privacy.text()).toBe("Privacy statement")
    expect(wrapper.getComponent(RouterLinkStub).props("to")).toBe("/en/imprint")
  })

  it("links text pages without the router when plain", () => {
    const wrapper = render({
      links: { imprint: { mode: "text" } },
      path:  () => "/impressum",
      plain: true,
    })
    expect(wrapper.findComponent(RouterLinkStub).exists()).toBe(false)
    expect(wrapper.get("a").attributes("href")).toBe("/impressum")
    expect(wrapper.text()).toBe("Imprint")
  })

  it("renders nothing without pages", () => {
    const wrapper = render({
      links: {},
      path:  () => "",
    })
    expect(wrapper.find("nav").exists()).toBe(false)
  })
})
