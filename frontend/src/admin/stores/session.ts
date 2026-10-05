import { defineStore } from "pinia"
import { ref } from "vue"

import { can as rolesCan, type Role, type Roles } from "../roles"

export interface Provider {
  id:        string
  method:    "redirect" | "credentials"
  available: boolean
  /** login says how accounts are named. */
  login:     "email" | "username"
}

export interface User extends Roles {
  id:          string
  displayName: string
  email?:      string
}

/** useSession tracks the signed-in admin. */
export const useSession = defineStore("session", () => {
  const state = ref<"loading" | "unreachable" | "anonymous" | "signed-in">("loading")
  const user = ref<User | null>(null)
  const provider = ref<Provider | null>(null)

  /** load fetches the user with their current roles. */
  async function load() {
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
  }

  async function check() {
    try {
      await load()
    } catch {
      state.value = "unreachable"
    }
  }

  function expire() {
    user.value = null
    state.value = "anonymous"
  }

  /** can reports whether the user holds role on the site, or anywhere. */
  function can(site: string, role: Role): boolean {
    return rolesCan(user.value, site, role)
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
    load,
    check,
    expire,
    can,
    logout,
  }
})
