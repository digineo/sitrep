<script setup lang="ts">
import { useHead } from "@unhead/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import logo from "../../shared/assets/logo.svg"
import SRButton from "../../shared/components/SRButton.vue"
import { useLoad } from "../../shared/composables/useLoad"
import { getJSON } from "../siteData"

const { t, locale } = useI18n()
useHead({ title: computed(() => t("page.landing")) })
const { value: landing, error } = useLoad(
  () => locale.value,
  signal => getJSON<{ html: string }>(
    `/api/public/landing?lang=${locale.value}`,
    signal,
  ),
)
</script>

<template>
  <div
      v-if="error"
      class="notification is-danger mt-5"
      role="alert"
  >
    {{ t("legal.loadFailed") }}
  </div>
  <div
      v-else-if="!landing"
      aria-busy="true"
  />
  <template v-else-if="landing.html">
    <h1 class="sr-visually-hidden">
      {{ t("page.landing") }}
    </h1>
    <!-- eslint-disable vue/no-v-html -- server-rendered Markdown -->
    <div
        class="box content mt-5"
        v-html="landing.html"
    />
    <!-- eslint-enable vue/no-v-html -->
  </template>
  <div
      v-else
      class="sr-landing"
  >
    <img
        :src="logo"
        alt=""
        width="96"
        height="96"
    >
    <h1 class="title is-2">
      SitRep
    </h1>
    <p class="subtitle">
      {{ t("landing.tagline") }}
    </p>
    <SRButton
        variant="primary"
        href="/admin/"
    >
      {{ t("landing.toAdmin") }}
    </SRButton>
  </div>
</template>

<style scoped>
.sr-landing {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 4rem 1rem;
  text-align: center;
}
</style>
