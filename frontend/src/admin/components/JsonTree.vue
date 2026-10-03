<script setup lang="ts">
defineProps<{
  value: unknown
  name:  string
  open?: boolean
}>()
</script>

<template>
  <details
      v-if="value !== null && typeof value === 'object'"
      :open
  >
    <summary>
      <code>{{ name }}</code>
      <span class="sr-muted"> {{ Array.isArray(value) ? `[${value.length}]` : "{ }" }}</span>
    </summary>
    <ul class="sr-json">
      <li
          v-for="(v, k) in (value as Record<string, unknown>)"
          :key="k"
      >
        <JsonTree
            :value="v"
            :name="String(k)"
        />
      </li>
    </ul>
  </details>
  <span v-else><code>{{ name }}</code>: <code>{{ JSON.stringify(value) }}</code></span>
</template>

<style scoped>
.sr-json {
  margin-left: 1.25rem;
}

summary {
  cursor: pointer;
}
</style>
