import { mount, RouterLinkStub } from "@vue/test-utils"
import { describe, expect, it } from "vitest"

import { createAppI18n } from "../i18n"
import SRPagination from "./SRPagination.vue"

function links(page: number, pages: number) {
  const wrapper = mount(SRPagination, {
    props: {
      page,
      pages,
      to: (n: number) => `/incidents?page=${n}`,
    },
    global: {
      plugins: [createAppI18n("en")],
      stubs:   { RouterLink: RouterLinkStub },
    },
  })
  return {
    wrapper,
    links: wrapper.findAllComponents(RouterLinkStub)
      .map(l => [l.text(), l.props("to")]),
  }
}

describe("SRPagination", () => {
  it("is hidden with a single page", () => {
    expect(links(1, 1).wrapper.find("nav").exists()).toBe(false)
  })

  it("links newer and older pages where they exist", () => {
    expect(links(1, 3).links).toEqual([["Older", "/incidents?page=2"]])
    expect(links(2, 3).links).toEqual([
      ["Newer", "/incidents?page=1"],
      ["Older", "/incidents?page=3"],
    ])
    expect(links(3, 3).links).toEqual([["Newer", "/incidents?page=2"]])
  })

  it("names the page and the navigation", () => {
    const { wrapper } = links(2, 3)
    expect(wrapper.text()).toContain("Page 2 of 3")
    expect(wrapper.find("nav").attributes("aria-label")).toBe("Pages")
  })
})
