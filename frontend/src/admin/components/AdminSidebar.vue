<script setup lang="ts">
import {
  ChevronRight,
  Database,
  EyeOff,
  LayoutDashboard,
  LogOut,
  Pause,
  Plus,
  Settings,
  Siren,
  Upload,
} from "@lucide/vue"
import { computed, inject, ref, watch } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute, useRouter } from "vue-router"

import logo from "../../shared/assets/logo.svg"
import { type Bootstrap } from "../../shared/bootstrap"
import LanguageSwitcher from "../../shared/components/LanguageSwitcher.vue"
import SRButton from "../../shared/components/SRButton.vue"
import SRStatusDot from "../../shared/components/SRStatusDot.vue"
import ThemeSwitcher from "../../shared/components/ThemeSwitcher.vue"
import { useLanguage } from "../../shared/composables/useLanguage"
import { usePolling } from "../../shared/composables/usePolling"
import { supported } from "../../shared/i18n"
import { resolveText, routeLabel } from "../rules"
import { useNotices } from "../stores/notices"
import { useOverview } from "../stores/overview"
import { useSession } from "../stores/session"
import { importDetail, importSite } from "../transfer"

const { t, locale } = useI18n()
const { choose } = useLanguage()
const session = useSession()
const overview = useOverview()
const route = useRoute()
const router = useRouter()
const notices = useNotices()
const bootstrap = inject<Bootstrap>("bootstrap")!

// Site statuses refresh every 15 seconds and after each navigation.
const { refresh } = usePolling(() => overview.refresh(), {
  interval: () => 15_000,
  onResult: () => {},
  onError:  () => {},
})
watch(() => route.fullPath, () => void refresh())

const sites = computed(() => {
  const collator = new Intl.Collator(locale.value)
  return (overview.sites ?? [])
    .map(site => ({
      ...site,
      label: resolveText(site.name, locale.value, site.languages),
    }))
    .sort((a, b) => collator.compare(a.label, b.label))
})
const unusable = computed(() => overview.dataSources?.some(ds => !ds.usable))

// The current route's site opens; other sites only toggle.
const open = ref(new Set<string>())
watch(() => route.params.site, (site) => {
  if (typeof site === "string") {
    open.value = new Set(open.value).add(site)
  }
}, { immediate: true })

const availabilityIcons = {
  offline: EyeOff,
  paused:  Pause,
}

/** importFile creates a site from the chosen file and opens its preview. */
async function importFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ""
  if (!file) {
    return
  }

  try {
    const site = await importSite(file)
    notices.success(t("site.imported"))
    await router.push(`/sites/${site.id}`)
  } catch(err) {
    notices.failure(t("toast.importFailed"), err, importDetail(err, t))
  }
}

function onToggle(site: string, event: Event) {
  const next = new Set(open.value)
  if ((event.target as HTMLDetailsElement).open) {
    next.add(site)
  } else {
    next.delete(site)
  }

  open.value = next
}
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
      <li>
        <RouterLink
            to="/sites/new"
            active-class="is-active"
        >
          <span class="icon-text">
            <span class="icon"><Plus aria-hidden="true" /></span>
            <span>{{ t("nav.newSite") }}</span>
          </span>
        </RouterLink>
      </li>
      <li>
        <label class="menu-item sr-file">
          <span class="icon-text">
            <span class="icon"><Upload aria-hidden="true" /></span>
            <span>{{ t("nav.import") }}</span>
          </span>
          <input
              type="file"
              accept=".yaml,.yml"
              class="sr-visually-hidden"
              @change="importFile"
          >
        </label>
      </li>
      <li>
        <RouterLink
            to="/datasources"
            active-class="is-active"
        >
          <span class="icon-text">
            <span class="icon"><Database aria-hidden="true" /></span>
            <span>{{ t("nav.dataSources") }}</span>
          </span>
          <SRStatusDot
              v-if="unusable"
              state="degraded"
              :label="t('nav.unusable')"
              class="ml-2"
          />
        </RouterLink>
      </li>
    </ul>
    <template v-if="sites.length">
      <p class="menu-label">
        {{ t("nav.sites") }}
      </p>
      <details
          v-for="site in sites"
          :key="site.id"
          class="sr-site"
          :open="open.has(site.id)"
          @toggle="onToggle(site.id, $event)"
      >
        <summary>
          <span class="sr-site-name">
            <span>{{ site.label }}</span>
            <span class="sr-muted is-size-7">{{ routeLabel(site.route, bootstrap.baseDomains ?? []) }}</span>
          </span>
          <span
              v-if="site.availability !== 'online'"
              class="icon sr-muted"
              role="img"
              :aria-label="t(`availability.${site.availability}`)"
              :title="t(`availability.${site.availability}`)"
          >
            <component
                :is="availabilityIcons[site.availability]"
                aria-hidden="true"
            />
          </span>
          <SRStatusDot
              v-if="site.status.overall !== 'operational' && !open.has(site.id)"
              :state="site.status.overall"
              :label="t('nav.status', { status: t(`status.${site.status.overall}`) })"
          />
          <span class="icon sr-chevron"><ChevronRight aria-hidden="true" /></span>
        </summary>
        <ul class="menu-list">
          <li>
            <RouterLink
                :to="`/sites/${site.id}/incidents`"
                active-class="is-active"
            >
              <span class="icon-text">
                <span class="icon"><Siren aria-hidden="true" /></span>
                <span>{{ t("nav.incidents") }}</span>
              </span>
              <SRStatusDot
                  :state="site.status.incidents"
                  :label="t('nav.status', { status: t(`status.${site.status.incidents}`) })"
                  class="ml-2"
              />
            </RouterLink>
          </li>
          <li>
            <RouterLink
                :to="`/sites/${site.id}`"
                exact-active-class="is-active"
            >
              <span class="icon-text">
                <span class="icon"><LayoutDashboard aria-hidden="true" /></span>
                <span>{{ t("nav.panels") }}</span>
              </span>
              <SRStatusDot
                  v-if="site.availability !== 'paused'"
                  :state="site.status.panels"
                  :label="t('nav.status', { status: t(`status.${site.status.panels}`) })"
                  class="ml-2"
              />
            </RouterLink>
          </li>
          <li>
            <RouterLink
                :to="`/sites/${site.id}/settings`"
                active-class="is-active"
            >
              <span class="icon-text">
                <span class="icon"><Settings aria-hidden="true" /></span>
                <span>{{ t("nav.siteSettings") }}</span>
              </span>
            </RouterLink>
          </li>
        </ul>
      </details>
    </template>
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

.menu-list a,
.menu-list .menu-item {
  display: flex;
  align-items: center;
}

.sr-site summary {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  padding: 0.5em 0.75em;
  cursor: pointer;
  border-radius: var(--bulma-radius-small);
  list-style: none;
}

.sr-site summary::-webkit-details-marker {
  display: none;
}

.sr-site summary:hover {
  background: var(--bulma-scheme-main-ter);
}

.sr-site-name {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  overflow-wrap: anywhere;
}

.sr-chevron {
  transition: transform 0.2s;
}

.sr-site[open] .sr-chevron {
  transform: rotate(90deg);
}

.sr-site .menu-list {
  margin: 0 0 0.5rem 0.75rem;
}

.sr-sidebar-footer {
  padding: 1rem;
  border-top: 1px solid var(--bulma-border-weak);
}
</style>
