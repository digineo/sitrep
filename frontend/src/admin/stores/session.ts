import { defineStore } from "pinia"
import { ref } from "vue"

export interface Provider {
  id:        string
  method:    "redirect" | "credentials"
  available: boolean
}

export interface User {
  displayName: string
  email?:      string
}

/** useSession tracks the signed-in admin. */
export const useSession = defineStore("session", () => {
  const state = ref<"loading" | "unreachable" | "anonymous" | "signed-in">("loading")
  const user = ref<User | null>(null)
  const provider = ref<Provider | null>(null)

  async function check() {
    try {
      const res = await fetch(
        "/auth/session",
        { headers: { Accept: "application/json" } },
      )
      if (!res.ok) {
        throw new Error(res.statusText)
      }

      const data: {
        user:     User | null
        provider: Provider
      } = await res.json()
      user.value = data.user
      provider.value = data.provider
      state.value = data.user ? "signed-in" : "anonymous"
    } catch {
      state.value = "unreachable"
    }
  }

  function expire() {
    user.value = null
    state.value = "anonymous"
  }

  async function logout() {
    const res = await fetch("/auth/logout", {
      method:  "POST",
      headers: { "Content-Type": "application/json" },
      body:    "{}",
    })
    window.location.assign(res.url)
  }
  return {
    state,
    user,
    provider,
    check,
    expire,
    logout,
  }
})
