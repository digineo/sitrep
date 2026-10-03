<script setup lang="ts">
import { defineAsyncComponent } from "vue"

// The PromQL editor is loaded only when a Prometheus data source needs it.
const PromQLEditor = defineAsyncComponent(() => import("./PromQLEditor.vue"))

defineProps<{
  /** editor is the data source type's editor hint. */
  editor?:      string
  datasource:   string
  id:           string
  labelId:      string
  describedby?: string
  invalid?:     boolean
}>()

const model = defineModel<string>({ required: true })
</script>

<template>
  <PromQLEditor
      v-if="editor === 'promql'"
      v-model="model"
      :datasource
      :label-id
      :describedby
  />
  <textarea
      v-else
      :id
      v-model="model"
      class="textarea is-family-monospace sr-autogrow"
      rows="2"
      required
      maxlength="10000"
      spellcheck="false"
      :aria-describedby="describedby"
      :aria-invalid="invalid"
  />
</template>
