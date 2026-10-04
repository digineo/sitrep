<script setup lang="ts">
import { computed, ref, useId } from "vue"
import { useI18n } from "vue-i18n"

import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import { bandStyle } from "../../shared/contrast"
import { api, fieldErrors } from "../api"

const props = defineProps<{
  /** name is the site's name in the brand preview. */
  name:   string
  /** errors maps "brandColor" and "logo" to server error codes. */
  errors: Record<string, string>
}>()

const color = defineModel<string | undefined>("color", { required: true })
/**
 * The logo is always the sanitized source the server returned, never the raw
 * upload.
 */
const logo = defineModel<string | undefined>("logo", { required: true })

const { t } = useI18n()
const fileId = useId()
const lastColor = ref(color.value ?? "#2f6feb")
const uploadError = ref("")
const uploading = ref(false)

const band = computed(() => bandStyle(color.value))
const logoURL = computed(
  () => logo.value && `data:image/svg+xml,${encodeURIComponent(logo.value)}`,
)
const logoError = computed(() => {
  const code = uploadError.value || props.errors.logo
  return code ? t(`error.${code}`) : undefined
})

function setColor(value: string) {
  lastColor.value = value
  color.value = value
}

/** upload has the server sanitize the chosen file and keeps the result. */
async function upload(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ""
  if (!file) {
    return
  }

  uploadError.value = ""
  uploading.value = true
  try {
    const svg = await file.text()
    logo.value = (await api<{ svg: string }>("POST", "/api/admin/svg", { svg })).svg
  } catch(err) {
    uploadError.value = fieldErrors(err).svg ?? "invalid_svg"
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <SRField
      v-slot="{ id, describedby, invalid }"
      :label="t('brand.color')"
      :help="t('brand.colorHelp')"
      :error="errors.brandColor && t(`error.${errors.brandColor}`)"
  >
    <div class="sr-color">
      <input
          :id
          class="sr-color-input"
          type="color"
          :value="color ?? lastColor"
          :disabled="!color"
          :aria-describedby="describedby"
          :aria-invalid="invalid"
          @input="setColor(($event.target as HTMLInputElement).value)"
      >
      <label class="checkbox">
        <input
            type="checkbox"
            :checked="!color"
            @change="color = ($event.target as HTMLInputElement).checked ? undefined : lastColor"
        >
        {{ t("brand.none") }}
      </label>
    </div>
  </SRField>
  <div class="field">
    <p
        :id="`${fileId}-label`"
        class="label"
    >
      {{ t("brand.logo") }}
    </p>
    <div class="sr-color">
      <label
          class="button sr-file"
          :class="{ 'is-loading': uploading }"
      >
        <input
            class="sr-visually-hidden"
            type="file"
            accept=".svg,image/svg+xml"
            :aria-describedby="`${fileId}-label ${fileId}-help`"
            @change="upload"
        >
        {{ t("brand.upload") }}
      </label>
      <SRButton
          v-if="logo"
          @click="logo = undefined"
      >
        {{ t("brand.remove") }}
      </SRButton>
    </div>
    <p
        :id="`${fileId}-help`"
        class="help"
    >
      {{ t("brand.logoHelp") }}
    </p>
    <p
        v-if="logoError"
        class="help is-danger"
    >
      {{ logoError }}
    </p>
  </div>
  <div
      v-if="color || logo"
      class="field"
  >
    <p class="label">
      {{ t("brand.preview") }}
    </p>
    <div
        class="sr-band sr-preview"
        :class="{ 'is-branded': band }"
        :style="band"
    >
      <img
          v-if="logoURL"
          :src="logoURL"
          alt=""
          class="sr-logo"
      >
      <div>
        <p class="title is-4 mb-1">
          {{ name }}
        </p>
        <p class="sr-band-muted">
          {{ t("site.currentStatus") }}
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sr-color {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: center;
}

/* As tall as the swatch, the checkbox's line keeps its text in the middle
   and gives the field its baseline. */
.sr-color > .checkbox {
  align-self: baseline;
  line-height: 2.5rem;
}

.sr-color-input {
  width: 4rem;
  height: 2.5rem;
  padding: 0.125rem;
  cursor: pointer;
  border: 1px solid var(--bulma-border);
  border-radius: var(--bulma-radius);
}

.sr-preview {
  display: flex;
  gap: 1rem;
  align-items: center;
  padding: 1.5rem 1rem;
  border: 1px solid var(--bulma-border);
  border-radius: var(--bulma-radius);
}

.sr-logo {
  width: auto;
  max-width: 12rem;
  height: 3.25rem;
  object-fit: contain;
}
</style>
