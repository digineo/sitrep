<script setup lang="ts">
import { nextTick, ref, useTemplateRef } from "vue"
import { useI18n } from "vue-i18n"

import { watchDebounced } from "../../shared/composables/watchDebounced"
import { api } from "../api"

// Attributes such as id, required and aria-* belong to the textarea.
defineOptions({ inheritAttrs: false })

const model = defineModel<string>({ required: true })

const { t } = useI18n()
const textarea = useTemplateRef<HTMLTextAreaElement>("textarea")
const mode = ref<"edit" | "preview">("edit")
const html = ref("")
const failed = ref(false)
let latest = 0

/** render fetches the preview of the text; only the latest answer is shown. */
async function render(text: string) {
  const id = ++latest
  try {
    const res = text.trim()
      ? await api<{ html: string }>("POST", "/api/admin/markdown", { text })
      : { html: "" }
    if (id === latest) {
      html.value = res.html
      failed.value = false
    }
  } catch {
    if (id === latest) {
      failed.value = true
    }
  }
}

void render(model.value)
watchDebounced(model, render, 300)

// A failed check shows the textarea, so that it can take the focus.
async function onInvalid() {
  if (mode.value !== "edit") {
    mode.value = "edit"
    await nextTick()
    textarea.value?.focus()
  }
}
</script>

<template>
  <div
      class="sr-markdown"
      :data-mode="mode"
  >
    <div
        class="buttons has-addons are-small sr-markdown-toggle"
        role="group"
        :aria-label="t('markdown.view')"
    >
      <button
          type="button"
          class="button"
          :class="{ 'is-selected is-link': mode === 'edit' }"
          :aria-pressed="mode === 'edit'"
          @click="mode = 'edit'"
      >
        {{ t("markdown.edit") }}
      </button>
      <button
          type="button"
          class="button"
          :class="{ 'is-selected is-link': mode === 'preview' }"
          :aria-pressed="mode === 'preview'"
          @click="mode = 'preview'"
      >
        {{ t("markdown.preview") }}
      </button>
    </div>
    <div class="sr-markdown-body">
      <textarea
          ref="textarea"
          v-bind="$attrs"
          class="textarea sr-autogrow sr-markdown-edit"
          :value="model"
          @input="model = ($event.target as HTMLTextAreaElement).value"
          @invalid="onInvalid"
      />
      <p
          v-if="failed"
          class="box sr-markdown-preview sr-muted"
      >
        {{ t("markdown.previewFailed") }}
      </p>
      <!-- eslint-disable vue/no-v-html -- server-rendered Markdown -->
      <div
          v-else
          class="content box sr-markdown-preview"
          :aria-label="t('markdown.preview')"
          role="region"
          v-html="html"
      />
      <!-- eslint-enable vue/no-v-html -->
    </div>
  </div>
</template>

<style scoped>
.sr-markdown {
  container-type: inline-size;
}

.sr-markdown-preview {
  min-height: 4.5em;
  margin: 0;
  overflow-wrap: anywhere;
}

.sr-markdown[data-mode="edit"] .sr-markdown-preview,
.sr-markdown[data-mode="preview"] .sr-markdown-edit {
  display: none;
}

/* With enough room, text and preview sit side by side. */
@container (width >= 40rem) {
  .sr-markdown-toggle {
    display: none;
  }

  .sr-markdown-body {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    gap: 0.75rem;
  }

  .sr-markdown[data-mode] .sr-markdown-edit,
  .sr-markdown[data-mode] .sr-markdown-preview {
    display: block;
  }
}
</style>
