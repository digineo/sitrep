<script setup lang="ts">
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import logo from "../../shared/assets/logo.svg"
import LanguageSwitcher from "../../shared/components/LanguageSwitcher.vue"
import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import ThemeSwitcher from "../../shared/components/ThemeSwitcher.vue"
import { useLanguage } from "../../shared/composables/useLanguage"
import { supported } from "../../shared/i18n"
import { api, ApiError } from "../api"
import { usePageTitle } from "../composables/usePageTitle"
import { useSession } from "../stores/session"

const { t } = useI18n()
const { locale, choose } = useLanguage()
const session = useSession()
const route = useRoute()
const router = useRouter()
usePageTitle(() => t("login.title"))

const signedOut = ref(false)
const username = ref("")
const password = ref("")
const busy = ref(false)
const error = ref("")

const provider = computed(() => session.provider!)
const ssoHref = computed(() => `/auth/${provider.value.id}/login?return=${
  encodeURIComponent(router.resolve(route.fullPath).href)
}`)

onMounted(async() => {
  if ("signed-out" in route.query) {
    signedOut.value = true
    const query = { ...route.query }
    delete query["signed-out"]
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
            v-if="signedOut"
            class="notification is-success"
        >
          <button
              type="button"
              class="delete"
              :aria-label="t('common.dismiss')"
              @click="signedOut = false"
          />
          {{ t("login.signedOut") }}
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

.sr-login-brand {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}
</style>
