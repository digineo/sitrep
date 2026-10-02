<script setup lang="ts">
import { useI18n } from "vue-i18n"

import { usePageTitle } from "../composables/usePageTitle"

const { t } = useI18n()
usePageTitle(() => t("home.title"))
</script>

<template>
  <h1 class="title">
    {{ t("home.title") }}
  </h1>
  <p>{{ t("home.empty") }}</p>
</template>
