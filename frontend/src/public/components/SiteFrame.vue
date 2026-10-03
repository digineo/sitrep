<script setup lang="ts">
import { History, Rss } from "@lucide/vue"
import { computed, inject, watch, watchEffect } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import degraded from "../../shared/assets/degraded.svg"
import down from "../../shared/assets/down.svg"
import logo from "../../shared/assets/logo.svg"
import operational from "../../shared/assets/operational.svg"
import { type Bootstrap } from "../../shared/bootstrap"
import LanguageSwitcher from "../../shared/components/LanguageSwitcher.vue"
import LegalLinks from "../../shared/components/LegalLinks.vue"
import LiveIndicator from "../../shared/components/LiveIndicator.vue"
import ThemeSwitcher from "../../shared/components/ThemeSwitcher.vue"
import { usePolling } from "../../shared/composables/usePolling"
import { bandStyle } from "../../shared/contrast"
import type { LegalKind, State } from "../../shared/payload"
import {
  applySite,
  failSite,
  live,
  loadSite,
  siteData,
  unavailable,
} from "../siteData"
import { pagePath } from "../urls"

const emit = defineEmits<{ switchLanguage: [lang: string] }>()

const bootstrap = inject<Bootstrap>("bootstrap")!
const route = useRoute()
const { t, te, locale } = useI18n()

// An unavailable site is checked again every five minutes.
const { refresh } = usePolling(
  signal => loadSite(bootstrap.siteId!, locale.value, signal),
  {
    interval: () => unavailable.value ? 300_000 : 15_000,
    onResult: applySite,
    onError:  failSite,
  },
)
// Another language needs a full payload in that language.
watch(locale, () => void refresh())

const site = computed(() => siteData.value?.site ?? unavailable.value)
const band = computed(() => bandStyle(site.value?.brandColor))
const legal = computed(() => site.value?.legal ?? {})
const showMain = computed(() => !unavailable.value
  || route.meta.page === "imprint" || route.meta.page === "privacy")
const multi = bootstrap.languages.length > 1
const overview = computed(() => pagePath({ kind: "overview" }, locale.value, multi))
const archive = computed(() => pagePath({ kind: "archive" }, locale.value, multi))
const legalPath = (kind: LegalKind) => pagePath({ kind }, locale.value, multi)
const feed = computed(
  () => `${bootstrap.basePath}${multi ? `/${locale.value}` : ""}/feed.atom`,
)
const errorText = computed(() => t(te(live.error) ? live.error : "error.internal"))

// The favicon shows the status in its color, and the plain logo while the
// status is unknown, the site unavailable or the page loading.
const icons: Partial<Record<State, string>> = {
  operational,
  degraded,
  down,
}
watchEffect(() => {
  const icon = icons[siteData.value?.status ?? "unknown"] ?? logo
  document.querySelector<HTMLLinkElement>("link[rel=icon]")!.href = icon
})
</script>

<template>
  <header
      class="sr-band"
      :class="{ 'is-branded': band }"
      :style="band"
  >
    <div class="container px-4 sr-header">
      <div class="sr-brand">
        <img
            v-if="site?.logo"
            :src="site.logo"
            alt=""
            class="sr-logo"
        >
        <div>
          <h1 class="title is-3 mb-1">
            <RouterLink
                :to="overview"
                class="has-text-inherit"
            >
              {{ site?.name }}
            </RouterLink>
          </h1>
          <p
              v-if="!unavailable"
              class="sr-subline sr-band-muted"
          >
            <span>{{ t("site.currentStatus") }}</span>
            <LiveIndicator
                :state="live.state"
                :updated-at="live.updatedAt"
                :timezone="siteData?.site.timezone ?? 'UTC'"
                :pulse="live.pulse"
                :error="errorText"
                :detail="live.detail"
            />
          </p>
        </div>
      </div>
      <div class="buttons sr-header-actions">
        <template v-if="!unavailable">
          <RouterLink
              :to="archive"
              class="button"
              :aria-label="t('incidents.history')"
              :title="t('incidents.history')"
          >
            <span class="icon"><History aria-hidden="true" /></span>
          </RouterLink>
          <a
              :href="feed"
              class="button"
              type="application/atom+xml"
              :aria-label="t('incidents.feed')"
              :title="t('incidents.feedHint')"
          >
            <span class="icon"><Rss aria-hidden="true" /></span>
          </a>
        </template>
        <LanguageSwitcher
            v-if="bootstrap.languages.length > 1"
            :model-value="locale"
            :languages="bootstrap.languages"
            right
            @update:model-value="emit('switchLanguage', $event)"
        />
        <ThemeSwitcher right />
      </div>
    </div>
  </header>
  <main class="container px-4 py-5">
    <RouterView v-if="showMain" />
    <div
        v-else
        class="notification"
        role="status"
    >
      {{ t("site.unavailable") }}
    </div>
  </main>
  <footer
      v-if="legal.imprint || legal.privacy"
      class="sr-band"
      :class="{ 'is-branded': band }"
      :style="band"
  >
    <div class="container px-4 py-4">
      <LegalLinks
          :links="legal"
          :path="legalPath"
      />
    </div>
  </footer>
</template>

<style scoped>
.sr-header {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
  justify-content: space-between;
  padding-top: 1.5rem;
  padding-bottom: 1.5rem;
}

.sr-brand {
  display: flex;
  gap: 1rem;
  align-items: center;
}

.sr-logo {
  width: auto;
  max-width: 12rem;
  height: 3.25rem;
  object-fit: contain;
}

.sr-subline {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
}

.sr-header-actions {
  margin: 0;
}

.has-text-inherit {
  color: inherit;
}

@media (width < 769px) {
  .sr-header,
  .sr-brand {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
