<script setup lang="ts">
defineProps<{
  variant?:  "primary" | "danger"
  type?:     "button" | "submit"
  loading?:  boolean
  disabled?: boolean
  href?:     string
}>()
</script>

<template>
  <a
      v-if="href"
      :href
      class="button"
      :class="variant && `is-${variant}`"
  ><slot /></a>
  <button
      v-else
      :type="type ?? 'button'"
      class="button"
      :class="[variant && `is-${variant}`, { 'is-loading': loading }]"
      :disabled="loading || disabled || undefined"
  >
    <slot />
  </button>
</template>
