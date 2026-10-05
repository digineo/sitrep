<script setup lang="ts">
import { LayoutDashboard, Plus, Settings, Siren } from "@lucide/vue"
import { computed, inject } from "vue"
import { useI18n } from "vue-i18n"

import { type Bootstrap } from "../../shared/bootstrap"
import SRTag from "../../shared/components/SRTag.vue"
import StatusBanner from "../../shared/components/StatusBanner.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { resolveText, routeLabel } from "../rules"
import { useOverview } from "../stores/overview"
import { useSession } from "../stores/session"

const { t, locale } = useI18n()
const overview = useOverview()
const session = useSession()
const bootstrap = inject<Bootstrap>("bootstrap")!
usePageTitle(() => t("home.title"))

const sites = computed(() => {
  const collator = new Intl.Collator(locale.value)
  return (overview.sites ?? [])
    .map(site => ({
      ...site,
      label: resolveText(site.name, locale.value, site.languages),
    }))
    .sort((a, b) => collator.compare(a.label, b.label))
})
</script>

<template>
  <div class="level">
    <h1 class="title mb-0">
      {{ t("home.title") }}
    </h1>
    <RouterLink
        v-if="sites.length && session.can('', 'admin')"
        to="/sites/new"
        class="button is-primary"
    >
      <span class="icon"><Plus aria-hidden="true" /></span>
      <span>{{ t("nav.newSite") }}</span>
    </RouterLink>
  </div>
  <div
      v-if="sites.length"
      class="sr-cards"
  >
    <article
        v-for="site in sites"
        :key="site.id"
        class="card"
    >
      <div class="card-content">
        <div class="sr-card-head">
          <div class="sr-card-name">
            <h2 class="title is-5 mb-1">
              <RouterLink :to="`/sites/${site.id}`">
                {{ site.label }}
              </RouterLink>
            </h2>
            <p class="sr-muted is-size-7">
              {{ routeLabel(site.route, bootstrap.baseDomains ?? [], site.landing) }}
            </p>
          </div>
          <div class="buttons">
            <RouterLink
                :to="`/sites/${site.id}/incidents`"
                class="button is-small"
                :aria-label="t('home.incidentsOf', { site: site.label })"
                :title="t('nav.incidents')"
            >
              <span class="icon"><Siren aria-hidden="true" /></span>
            </RouterLink>
            <RouterLink
                :to="`/sites/${site.id}`"
                class="button is-small"
                :aria-label="t('home.panelsOf', { site: site.label })"
                :title="t('nav.panels')"
            >
              <span class="icon"><LayoutDashboard aria-hidden="true" /></span>
            </RouterLink>
            <RouterLink
                v-if="session.can(site.id, 'maintainer')"
                :to="`/sites/${site.id}/settings`"
                class="button is-small"
                :aria-label="t('home.settingsOf', { site: site.label })"
                :title="t('nav.siteSettings')"
            >
              <span class="icon"><Settings aria-hidden="true" /></span>
            </RouterLink>
          </div>
        </div>
        <div
            v-if="site.availability !== 'online' || site.missing"
            class="tags"
        >
          <SRTag v-if="site.availability !== 'online'">
            {{ t(`availability.${site.availability}`) }}
          </SRTag>
          <SRTag
              v-if="site.missing"
              color="warning"
          >
            {{ t("home.missing", site.missing) }}
          </SRTag>
        </div>
        <StatusBanner :status="site.status.overall" />
      </div>
    </article>
  </div>
  <p
      v-else-if="overview.sites && !session.can('', 'admin')"
      class="box"
  >
    {{ t("home.noAccess") }}
  </p>
  <div
      v-else-if="overview.sites && overview.dataSources?.length === 0"
      class="box"
  >
    <p class="block">
      {{ t("home.noDataSource") }}
    </p>
    <RouterLink
        to="/datasources/new"
        class="button is-primary"
    >
      {{ t("dataSources.new") }}
    </RouterLink>
  </div>
  <div
      v-else-if="overview.sites"
      class="box"
  >
    <p class="block">
      {{ t("home.empty") }}
    </p>
    <RouterLink
        to="/sites/new"
        class="button is-primary"
    >
      {{ t("nav.newSite") }}
    </RouterLink>
  </div>
</template>

<style scoped>
.sr-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 1rem;
}

/* The grid spaces the cards; Bulma's bottom margin would make all but the
   last one shorter. */
.sr-cards > .card {
  margin-bottom: 0;
}

.sr-card-head {
  display: flex;
  gap: 0.75rem;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}

.sr-card-name {
  min-width: 0;
  overflow-wrap: anywhere;
}

.sr-card-head .buttons {
  flex: none;
}
</style>
