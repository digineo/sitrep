import { createPinia, setActivePinia } from "pinia"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { useOverview } from "./overview"
import { type User, useSession } from "./session"

describe("overview", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("reloads the roles with the sites", async() => {
    const user: User = {
      id:          "a-1",
      displayName: "Ann",
      role:        "",
      sites:       { "s-1": "responder" },
    }
    const provider = {
      id:        "test",
      method:    "credentials",
      available: true,
      login:     "email",
    }
    vi.stubGlobal("fetch", vi.fn(async(path: string) => path === "/auth/session"
      ? Response.json({ user, provider })
      : Response.json([{ id: "s-1" }])))

    const session = useSession()
    session.state = "signed-in"
    session.user = { ...user, sites: {} }
    await useOverview().refresh()
    expect(session.can("s-1", "responder")).toBe(true)
  })
})
