<script setup lang="ts">
import { Menu } from "@lucide/vue"
import {
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  useTemplateRef,
  watch,
} from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import logo from "../../shared/assets/logo.svg"
import { useNotices } from "../stores/notices"
import AdminSidebar from "./AdminSidebar.vue"

const { t } = useI18n()
const route = useRoute()
const notices = useNotices()

// Below 1024px the sidebar is an off-canvas drawer.
const drawerOpen = ref(false)
const sidebar = useTemplateRef<HTMLElement>("sidebar")
const menuButton = useTemplateRef<HTMLButtonElement>("menuButton")
const desktop = window.matchMedia("(min-width: 1024px)")

const focusableSelector = "a[href], button:not([disabled], [tabindex=\"-1\"])"
const focusable = () => [
  ...sidebar.value!.querySelectorAll<HTMLElement>(focusableSelector),
]

async function openDrawer() {
  drawerOpen.value = true
  await nextTick()
  focusable()[0]?.focus()
}

function closeDrawer() {
  if (drawerOpen.value) {
    drawerOpen.value = false
    menuButton.value?.focus()
  }
}

// While the drawer is open, Tab cycles within it and Escape closes it.
function onKeydown(event: KeyboardEvent) {
  if (!drawerOpen.value) {
    return
  }
  if (event.key === "Escape") {
    closeDrawer()
    return
  }

  const all = focusable()
  if (event.key !== "Tab" || all.length === 0) {
    return
  }

  const [first, last] = event.shiftKey
    ? [all.at(-1)!, all[0]!]
    : [all[0]!, all.at(-1)!]
  if (document.activeElement === last
    || !sidebar.value!.contains(document.activeElement)) {
    event.preventDefault()
    first.focus()
  }
}

function onResize() {
  if (desktop.matches) {
    drawerOpen.value = false
  }
}

onMounted(() => desktop.addEventListener("change", onResize))
onBeforeUnmount(() => desktop.removeEventListener("change", onResize))

watch(() => route.fullPath, () => {
  drawerOpen.value = false
  notices.loadError = null
})
</script>

<template>
  <div
      class="sr-admin"
      @keydown="onKeydown"
  >
    <header class="sr-topbar">
      <button
          ref="menuButton"
          type="button"
          class="button is-ghost"
          :aria-label="t('nav.menu')"
          :aria-expanded="drawerOpen"
          aria-controls="sr-sidebar"
          @click="drawerOpen ? closeDrawer() : openDrawer()"
      >
        <span class="icon"><Menu aria-hidden="true" /></span>
      </button>
      <RouterLink to="/">
        <img
            :src="logo"
            alt="SitRep"
            width="32"
            height="32"
        >
      </RouterLink>
    </header>
    <div
        v-if="drawerOpen"
        class="sr-backdrop"
        @click="closeDrawer"
    />
    <aside
        id="sr-sidebar"
        ref="sidebar"
        class="sr-sidebar"
        :class="{ 'is-open': drawerOpen }"
    >
      <AdminSidebar />
    </aside>
    <main class="sr-content">
      <div
          v-if="notices.loadError"
          class="notification is-danger"
          role="alert"
      >
        {{ t("notice.loadFailed", { message: t(`error.${notices.loadError}`) }) }}
      </div>
      <slot />
    </main>
  </div>
</template>

<style scoped>
.sr-sidebar {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: 30;
  display: flex;
  flex-direction: column;
  width: 18rem;
  overflow-y: auto;
  background: var(--bulma-scheme-main-bis);
  border-right: 1px solid var(--bulma-border-weak);
}

.sr-content {
  margin-left: 18rem;
  padding: 2rem;
}

.sr-topbar {
  display: none;
}

@media (width < 1024px) {
  .sr-topbar {
    position: sticky;
    top: 0;
    z-index: 20;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    background: var(--bulma-scheme-main);
    border-bottom: 1px solid var(--bulma-border-weak);
  }

  /* Hidden drawers leave the tab order. Visibility switches at once on
     opening, so that the drawer can take the focus, and after the slide on
     closing. */
  .sr-sidebar {
    visibility: hidden;
    transform: translateX(-100%);
    transition: transform 0.2s, visibility 0s 0.2s;
  }

  .sr-sidebar.is-open {
    visibility: visible;
    transform: none;
    transition: transform 0.2s;
  }

  .sr-backdrop {
    position: fixed;
    inset: 0;
    z-index: 25;
    background: rgb(10 10 10 / 50%);
  }

  .sr-content {
    margin-left: 0;
    padding: 1rem;
  }
}
</style>
