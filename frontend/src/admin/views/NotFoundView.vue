<script setup lang="ts">
import { useI18n } from "vue-i18n"

import { usePageTitle } from "../composables/usePageTitle"

const { t } = useI18n()
usePageTitle(() => t("page.notFound"))
</script>

<template>
  <h1 class="title">
    {{ t("page.notFound") }}
  </h1>
  <p class="block">
    {{ t("notFound.text") }}
  </p>
  <RouterLink to="/">
    {{ t("notFound.toAdminHome") }}
  </RouterLink>
</template>
