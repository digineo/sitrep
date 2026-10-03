<script setup lang="ts">
defineProps<{
  color?: "info" | "link" | "success" | "warning" | "danger"
  light?: boolean
}>()
</script>

<template>
  <span
      class="tag"
      :class="[color && `is-${color}`, { 'is-light': light }]"
  ><slot /></span>
</template>
