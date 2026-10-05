import { describe, expect, it } from "vitest"

import { can, type Role, type Roles } from "./roles"

describe("can", () => {
  const maintainer: Roles = {
    role:  "",
    sites: {
      a: "maintainer",
      b: "responder",
    },
  }

  it.each([
    ["nobody", null, "", "", false],
    ["no role, signed in", { role: "", sites: {} }, "", "", true],
    ["no role on a site", { role: "", sites: {} }, "a", "responder", false],
    ["maintainer on own site", maintainer, "a", "maintainer", true],
    ["maintainer responds", maintainer, "a", "responder", true],
    ["responder on other site", maintainer, "b", "maintainer", false],
    ["unknown site", maintainer, "c", "responder", false],
    ["maintainer anywhere", maintainer, "", "maintainer", true],
    ["maintainer is no admin", maintainer, "", "admin", false],
    ["admin on every site", { role: "admin", sites: {} }, "c", "maintainer", true],
    ["admin is no owner", { role: "admin", sites: {} }, "", "owner", false],
    ["nobody holds an unknown role", { role: "owner", sites: {} }, "", "root" as Role, false],
  ] as const)("%s", (_, roles, site, role, want) => {
    expect(can(roles, site, role)).toBe(want)
  })
})
