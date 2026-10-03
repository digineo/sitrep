<script setup lang="ts">
import { computed, inject, watch } from "vue"
import { useI18n } from "vue-i18n"

import { type Bootstrap } from "../../shared/bootstrap"
import LanguageSwitcher from "../../shared/components/LanguageSwitcher.vue"
import LiveIndicator from "../../shared/components/LiveIndicator.vue"
import ThemeSwitcher from "../../shared/components/ThemeSwitcher.vue"
import { usePolling } from "../../shared/composables/usePolling"
import { applySite, failSite, live, loadSite, siteData } from "../siteData"
import { pagePath } from "../urls"

const emit = defineEmits<{ switchLanguage: [lang: string] }>()

const bootstrap = inject<Bootstrap>("bootstrap")!
const { t, te, locale } = useI18n()

const { refresh } = usePolling(
  signal => loadSite(bootstrap.siteId!, locale.value, signal),
  {
    interval: () => 15_000,
    onResult: applySite,
    onError:  failSite,
  },
)
// Another language needs a full payload in that language.
watch(locale, () => void refresh())

const site = computed(() => siteData.value?.site)
const multi = bootstrap.languages.length > 1
const overview = computed(() => pagePath({ kind: "overview" }, locale.value, multi))
const errorText = computed(() => t(te(live.error) ? live.error : "error.internal"))
</script>

<template>
  <header class="sr-band">
    <div class="container px-4 sr-header">
      <div>
        <h1 class="title is-3 mb-1">
          <RouterLink
              :to="overview"
              class="has-text-inherit"
          >
            {{ site?.name }}
          </RouterLink>
        </h1>
        <p class="sr-subline">
          <span>{{ t("site.currentStatus") }}</span>
          <LiveIndicator
              :state="live.state"
              :updated-at="live.updatedAt"
              :timezone="site?.timezone ?? 'UTC'"
              :pulse="live.pulse"
              :error="errorText"
              :detail="live.detail"
          />
        </p>
      </div>
      <div class="buttons sr-header-actions">
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
    <RouterView />
  </main>
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

.sr-subline {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  align-items: center;
  color: var(--bulma-text-weak);
}

.sr-header-actions {
  margin: 0;
}

.has-text-inherit {
  color: inherit;
}

@media (width < 769px) {
  .sr-header {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
