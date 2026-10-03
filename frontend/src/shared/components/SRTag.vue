<script setup lang="ts">
defineProps<{
  color?: "warning" | "danger"
}>()
</script>

<template>
  <span
      class="tag"
      :class="color && `is-${color}`"
  ><slot /></span>
</template>
