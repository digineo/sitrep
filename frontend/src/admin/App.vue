<script setup lang="ts">
import { computed, onMounted } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import SRConfirmDialog from "../shared/components/SRConfirmDialog.vue"
import AdminLayout from "./components/AdminLayout.vue"
import LoginScreen from "./components/LoginScreen.vue"
import ToastList from "./components/ToastList.vue"
import { useSession } from "./stores/session"
import NotFoundView from "./views/NotFoundView.vue"

const { t } = useI18n()
const session = useSession()
const route = useRoute()
onMounted(session.check)

// Routes beyond the user's roles look like missing ones.
const allowed = computed(() => session.can(
  typeof route.params.site === "string" ? route.params.site : "",
  route.meta.role ?? "",
))
</script>

<template>
  <div
      v-if="session.state === 'loading'"
      aria-busy="true"
  />
  <main
      v-else-if="session.state === 'unreachable'"
      class="section"
  >
    <h1 class="title">
      SitRep
    </h1>
    <p class="notification is-danger">
      {{ t("admin.unreachable") }}
    </p>
  </main>
  <LoginScreen v-else-if="session.state === 'anonymous'" />
  <AdminLayout v-else>
    <RouterView v-if="allowed" />
    <NotFoundView v-else />
  </AdminLayout>
  <SRConfirmDialog />
  <ToastList />
</template>
