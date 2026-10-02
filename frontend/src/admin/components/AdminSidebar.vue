<script setup lang="ts">
import { LogOut, Settings } from "@lucide/vue"
import { useI18n } from "vue-i18n"

import logo from "../../shared/assets/logo.svg"
import LanguageSwitcher from "../../shared/components/LanguageSwitcher.vue"
import SRButton from "../../shared/components/SRButton.vue"
import ThemeSwitcher from "../../shared/components/ThemeSwitcher.vue"
import { useLanguage } from "../../shared/composables/useLanguage"
import { supported } from "../../shared/i18n"
import { useSession } from "../stores/session"

const { t } = useI18n()
const { locale, choose } = useLanguage()
const session = useSession()
</script>

<template>
  <nav
      class="menu sr-nav"
      :aria-label="t('nav.label')"
  >
    <RouterLink
        to="/"
        class="sr-brand"
    >
      <img
          :src="logo"
          alt=""
          width="32"
          height="32"
      >
      <span>SitRep</span>
    </RouterLink>
    <p class="menu-label">
      {{ t("nav.global") }}
    </p>
    <ul class="menu-list">
      <li>
        <RouterLink
            to="/settings"
            active-class="is-active"
        >
          <span class="icon-text">
            <span class="icon"><Settings aria-hidden="true" /></span>
            <span>{{ t("nav.settings") }}</span>
          </span>
        </RouterLink>
      </li>
    </ul>
  </nav>
  <div class="sr-sidebar-footer">
    <p class="block">
      <span class="sr-visually-hidden">{{ t("nav.signedInAs") }}</span>
      <strong>{{ session.user?.displayName }}</strong>
    </p>
    <div class="buttons">
      <SRButton @click="session.logout()">
        <span class="icon"><LogOut aria-hidden="true" /></span>
        <span>{{ t("nav.signOut") }}</span>
      </SRButton>
      <ThemeSwitcher
          compact
          up
      />
      <LanguageSwitcher
          :model-value="locale"
          :languages="supported"
          compact
          up
          @update:model-value="choose"
      />
    </div>
  </div>
</template>

<style scoped>
.sr-nav {
  flex: 1;
  padding: 1rem;
}

.sr-brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1.5rem;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--bulma-text-strong);
}

.sr-sidebar-footer {
  padding: 1rem;
  border-top: 1px solid var(--bulma-border-weak);
}
</style>
