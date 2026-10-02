<script setup lang="ts">
import { useHead } from "@unhead/vue"
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import logo from "../../shared/assets/logo.svg"
import SRButton from "../../shared/components/SRButton.vue"

const { t } = useI18n()
useHead({ title: computed(() => t("page.landing")) })
</script>

<template>
  <div class="sr-landing">
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
