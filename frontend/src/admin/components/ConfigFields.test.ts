import { mount } from "@vue/test-utils"
import { describe, expect, it } from "vitest"

import { createAppI18n } from "../../shared/i18n"
import type { DataSourceType } from "../types"
import ConfigFields from "./ConfigFields.vue"

const type: DataSourceType = {
  id:         "prometheus",
  panelTypes: ["stat"],
  fields:     [
    {
      name:     "url",
      kind:     "url",
      required: true,
    },
    {
      name:     "token",
      kind:     "secret",
      required: true,
    },
  ],
}

function setup(url: string) {
  return mount(ConfigFields, {
    props: {
      type,
      stored: {
        config:  { url: "https://prom.example.com" },
        secrets: ["token"],
      },
      errors:  {},
      values:  { url },
      secrets: {},
    },
    global: { plugins: [createAppI18n("en")] },
  })
}

describe("ConfigFields", () => {
  it("keeps stored secrets while the URL is unchanged", () => {
    const wrapper = setup("https://prom.example.com/")
    expect(wrapper.text()).toContain("A value is stored.")
    const buttons = wrapper.findAll("button").map(b => b.text())
    expect(buttons).toEqual(["Replace", "Remove"])
  })

  it("says that stored secrets are removed when the URL changes", () => {
    const wrapper = setup("https://other.example.com")
    expect(wrapper.text()).toContain(
      "the stored value is removed when you save, unless you replace it",
    )
    expect(wrapper.findAll("button").map(b => b.text())).toEqual(["Replace"])
  })
})
