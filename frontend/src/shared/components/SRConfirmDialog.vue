<script setup lang="ts">
import { nextTick, useId, useTemplateRef, watch } from "vue"
import { useI18n } from "vue-i18n"

import { useConfirm } from "../composables/useConfirm"
import SRButton from "./SRButton.vue"

const { t } = useI18n()
const { pending } = useConfirm()
const dialog = useTemplateRef<HTMLDialogElement>("dialog")
const cancel = useTemplateRef<InstanceType<typeof SRButton>>("cancel")
const titleId = useId()
let trigger: HTMLElement | null = null

watch(pending, async(p) => {
  if (p && !dialog.value?.open) {
    trigger = document.activeElement as HTMLElement | null
    await nextTick()
    dialog.value?.showModal()
    cancel.value?.$el.focus()
  }
})

function close(confirmed: boolean) {
  const p = pending.value
  pending.value = null
  dialog.value?.close()
  trigger?.focus()
  p?.resolve(confirmed)
}

function onClick(event: MouseEvent) {
  // Clicks on the backdrop target the dialog itself.
  if (event.target === dialog.value) {
    close(false)
  }
}
</script>

<template>
  <dialog
      ref="dialog"
      class="sr-dialog"
      :aria-labelledby="titleId"
      @cancel.prevent="close(false)"
      @click="onClick"
  >
    <div
        v-if="pending"
        class="box"
    >
      <h2
          :id="titleId"
          class="title is-5"
      >
        {{ pending.title }}
      </h2>
      <p class="block">
        {{ pending.message }}
      </p>
      <div class="buttons is-right">
        <SRButton
            ref="cancel"
            @click="close(false)"
        >
          {{ t("common.cancel") }}
        </SRButton>
        <SRButton
            :variant="pending.danger ? 'danger' : 'primary'"
            @click="close(true)"
        >
          {{ pending.confirm }}
        </SRButton>
      </div>
    </div>
  </dialog>
</template>
