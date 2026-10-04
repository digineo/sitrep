<script setup lang="ts">
import { computed } from "vue"
import { useI18n } from "vue-i18n"

import type { LegalKind, LegalLinks } from "../payload"

const props = defineProps<{
  links:  LegalLinks
  /** path returns the path of a page in text mode. */
  path:   (kind: LegalKind) => string
  /**
   * plain links pages in text mode without the router, from outside their
   * application.
   */
  plain?: boolean
}>()

const { t } = useI18n()
const allKinds = ["privacy", "imprint"] as const
const kinds = computed(() => allKinds.filter(kind => props.links[kind]))
</script>

<template>
  <nav
      v-if="kinds.length"
      class="has-text-centered"
      :aria-label="t('legal.label')"
  >
    <template
        v-for="(kind, i) in kinds"
        :key="kind"
    >
      <span
          v-if="i"
          aria-hidden="true"
      > · </span>
      <a
          v-if="links[kind]!.mode === 'url'"
          :href="links[kind]!.url"
      >{{ t(`legal.${kind}.title`) }}</a>
      <a
          v-else-if="plain"
          :href="path(kind)"
      >{{ t(`legal.${kind}.title`) }}</a>
      <RouterLink
          v-else
          :to="path(kind)"
      >
        {{ t(`legal.${kind}.title`) }}
      </RouterLink>
    </template>
  </nav>
</template>
