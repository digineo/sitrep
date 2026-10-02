<script setup lang="ts">
import { computed, useId } from "vue"

const props = defineProps<{
  label:  string
  help?:  string
  error?: string
}>()

const id = useId()
const describedby = computed(() => [
  props.help && `${id}-help`,
  props.error && `${id}-error`,
].filter(Boolean).join(" ") || undefined)
</script>

<template>
  <div class="field">
    <label
        class="label"
        :for="id"
    >{{ label }}</label>
    <div class="control">
      <slot
          :id
          :describedby
          :invalid="error ? true : undefined"
      />
    </div>
    <p
        v-if="help"
        :id="`${id}-help`"
        class="help"
    >
      {{ help }}
    </p>
    <p
        v-if="error"
        :id="`${id}-error`"
        class="help is-danger"
    >
      {{ error }}
    </p>
  </div>
</template>
