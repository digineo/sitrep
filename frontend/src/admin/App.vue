<script setup lang="ts">
import { onMounted } from "vue"
import { useI18n } from "vue-i18n"

import SRConfirmDialog from "../shared/components/SRConfirmDialog.vue"
import AdminLayout from "./components/AdminLayout.vue"
import LoginScreen from "./components/LoginScreen.vue"
import ToastList from "./components/ToastList.vue"
import { useSession } from "./stores/session"

const { t } = useI18n()
const session = useSession()
onMounted(session.check)
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
    <RouterView />
  </AdminLayout>
  <SRConfirmDialog />
  <ToastList />
</template>
