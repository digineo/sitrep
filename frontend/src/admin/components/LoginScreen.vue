<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import logo from "../../shared/assets/logo.svg"
import LanguageSwitcher from "../../shared/components/LanguageSwitcher.vue"
import LegalLinks from "../../shared/components/LegalLinks.vue"
import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import ThemeSwitcher from "../../shared/components/ThemeSwitcher.vue"
import { useLanguage } from "../../shared/composables/useLanguage"
import { useLoad } from "../../shared/composables/useLoad"
import { catalogs, supported } from "../../shared/i18n"
import type { LegalKind, LegalLinks as Links } from "../../shared/payload"
import { api, ApiError } from "../api"
import { usePageTitle } from "../composables/usePageTitle"
import { useSession } from "../stores/session"

const { t } = useI18n()
const { locale, choose } = useLanguage()
const session = useSession()
const route = useRoute()
const router = useRouter()
usePageTitle(() => t("login.title"))

// A notice passed in the query: after logout, or a sign-in error code
// returned by a redirect provider.
const loginErrors = new Map([
  ["denied", "login.denied"],
  ["not_member", "login.notMember"],
  ["idp_error", "login.idpError"],
  ["unavailable", "login.unavailable"],
])
const notice = ref<{
  key:     string
  success: boolean
}>()
const username = ref("")
const password = ref("")
const busy = ref(false)
const error = ref("")

const provider = computed(() => session.provider!)

// The instance's legal pages are pages of the landing page; their paths
// in the console's language redirect to the right language.
const { value: legal } = useLoad(
  () => locale.value,
  signal => api<Links>(
    "GET",
    `/api/public/legal?lang=${locale.value}`,
    undefined,
    signal,
  ),
)
const legalPath = (kind: LegalKind) =>
  `/${catalogs[locale.value]!.legal[kind].slug}`
const ssoHref = computed(() => `/auth/${provider.value.id}/login?return=${
  encodeURIComponent(router.resolve(route.fullPath).href)
}`)

onMounted(async() => {
  const query = { ...route.query }
  const error = loginErrors.get(String(query["login-error"]))
  if ("signed-out" in query) {
    notice.value = {
      key:     "login.signedOut",
      success: true,
    }
  } else if (error) {
    notice.value = {
      key:     error,
      success: false,
    }
  }
  if ("signed-out" in query || "login-error" in query) {
    delete query["signed-out"]
    delete query["login-error"]
    await router.replace({ query })
  }
})

async function submit() {
  busy.value = true
  error.value = ""
  try {
    await api("POST", `/auth/${provider.value.id}/login`, {
      username: username.value,
      password: password.value,
    })
    await session.check()
  } catch(err) {
    error.value = err instanceof ApiError && err.status === 429
      ? t("login.throttled")
      : t("login.failed")
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="sr-login">
    <div class="sr-corner">
      <ThemeSwitcher right />
      <LanguageSwitcher
          :model-value="locale"
          :languages="supported"
          right
          @update:model-value="choose"
      />
    </div>
    <main class="sr-login-main">
      <div class="box">
        <div class="sr-login-brand">
          <img
              :src="logo"
              alt=""
              width="48"
              height="48"
          >
          <h1 class="title is-3">
            SitRep
          </h1>
        </div>
        <p class="block">
          {{ t("login.continue") }}
        </p>
        <div
            v-if="notice"
            class="notification"
            :class="notice.success ? 'is-success' : 'is-danger'"
            :role="notice.success ? undefined : 'alert'"
        >
          <button
              type="button"
              class="delete"
              :aria-label="t('common.dismiss')"
              @click="notice = undefined"
          />
          {{ t(notice.key) }}
        </div>
        <div
            v-if="!provider.available"
            class="notification is-warning"
        >
          {{ t("login.unavailable") }}
        </div>
        <SRButton
            v-else-if="provider.method === 'redirect'"
            variant="primary"
            :href="ssoHref"
        >
          {{ t("login.sso") }}
        </SRButton>
        <form
            v-else
            @submit.prevent="submit"
        >
          <SRField
              v-slot="{ id }"
              :label="t('login.username')"
          >
            <input
                :id
                v-model="username"
                class="input"
                autocomplete="username"
                required
            >
          </SRField>
          <SRField
              v-slot="{ id }"
              :label="t('login.password')"
          >
            <input
                :id
                v-model="password"
                class="input"
                type="password"
                autocomplete="current-password"
                required
            >
          </SRField>
          <p
              v-if="error"
              class="notification is-danger"
              role="alert"
          >
            {{ error }}
          </p>
          <SRButton
              type="submit"
              variant="primary"
              :loading="busy"
          >
            {{ t("login.submit") }}
          </SRButton>
        </form>
      </div>
    </main>
    <footer
        v-if="legal"
        class="sr-login-legal"
    >
      <LegalLinks
          :links="legal"
          :path="legalPath"
          plain
      />
    </footer>
  </div>
</template>

<style scoped>
.sr-login-main {
  display: flex;
  justify-content: center;
  padding: 2rem 1rem;
}

.box {
  width: min(24rem, 100%);
}

.sr-login-legal {
  display: flex;
  justify-content: center;
  padding: 0 1rem 2rem;
}

.sr-login-brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}
</style>
