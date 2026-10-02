import { createPinia, setActivePinia } from "pinia"
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"

import { api, ApiError } from "../api"
import { useNotices } from "./notices"
import { useSession } from "./session"

describe("notices", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it("hides success toasts after 4 seconds", () => {
    const notices = useNotices()
    notices.success("Settings saved.")
    expect(notices.toasts).toMatchObject([{
      kind: "success",
      text: "Settings saved.",
    }])

    vi.advanceTimersByTime(3999)
    expect(notices.toasts).toHaveLength(1)

    vi.advanceTimersByTime(1)
    expect(notices.toasts).toHaveLength(0)
  })

  it("keeps failure toasts until they are closed", () => {
    const notices = useNotices()
    notices.failure("Saving failed", new ApiError(400, "invalid"))
    vi.advanceTimersByTime(60_000)
    expect(notices.toasts).toMatchObject([{
      kind: "failure",
      text: "Saving failed",
      code: "invalid",
    }])

    notices.close(notices.toasts[0]!.id)
    expect(notices.toasts).toHaveLength(0)
  })

  it("maps unknown errors to a generic code", () => {
    const notices = useNotices()
    notices.failure("Saving failed", new ApiError(418, "teapot"))
    notices.failure("Saving failed", new Error("boom"))
    notices.loadFailed(new ApiError(0, "network"))
    expect(notices.toasts.map(t => t.code)).toEqual(["internal", "internal"])
    expect(notices.loadError).toBe("network")
  })

  it("shows no toast or banner for a 401", () => {
    const notices = useNotices()
    notices.failure("Saving failed", new ApiError(401, "unauthorized"))
    notices.loadFailed(new ApiError(401, "unauthorized"))
    expect(notices.toasts).toHaveLength(0)
    expect(notices.loadError).toBeNull()
  })
})

describe("api", () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("ends the session on a 401", async() => {
    const body = {
      error: {
        code:    "unauthorized",
        message: "",
      },
    }
    vi.stubGlobal("fetch", vi.fn(async() => Response.json(body, { status: 401 })))

    const session = useSession()
    session.state = "signed-in"
    session.user = { displayName: "Ann" }

    await expect(api("GET", "/api/admin/settings")).rejects.toMatchObject({
      status: 401,
      code:   "unauthorized",
    })
    expect(session.state).toBe("anonymous")
    expect(session.user).toBeNull()
  })

  it("reports validation fields", async() => {
    const fields = [{
      path: "languages.primary",
      code: "required",
    }]
    const body = {
      error: {
        code:    "invalid",
        message: "",
        fields,
      },
    }
    vi.stubGlobal("fetch", vi.fn(async() => Response.json(body, { status: 400 })))

    await expect(api("PUT", "/api/admin/settings", {})).rejects.toMatchObject({
      status: 400,
      code:   "invalid",
      fields,
    })
    expect(useSession().state).toBe("loading")
  })

  it("sends JSON", async() => {
    const fetch = vi.fn(async() => new Response(null, { status: 204 }))
    vi.stubGlobal("fetch", fetch)
    await expect(api("POST", "/x", { a: 1 })).resolves.toBeUndefined()
    expect(fetch).toHaveBeenCalledWith("/x", expect.objectContaining({
      method:  "POST",
      body:    "{\"a\":1}",
      headers: expect.objectContaining({ "Content-Type": "application/json" }),
    }))
  })
})
