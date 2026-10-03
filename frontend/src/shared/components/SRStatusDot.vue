<script setup lang="ts">
import type { State } from "../payload"

defineProps<{
  state:  State
  /** label is the dot's accessible name; without it, the dot is decorative. */
  label?: string
}>()
</script>

<template>
  <span
      class="sr-dot"
      :class="`is-${state}`"
      :role="label ? 'img' : undefined"
      :aria-label="label"
      :aria-hidden="label ? undefined : 'true'"
  />
</template>
