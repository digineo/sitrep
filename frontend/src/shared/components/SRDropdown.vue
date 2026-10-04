<script setup lang="ts" generic="T extends string">
import { ChevronDown } from "@lucide/vue"
import { type Component, computed, nextTick, ref, useId, useTemplateRef } from "vue"

export interface DropdownItem<V> {
  value: V
  label: string
  icon?: Component
  lang?: string
}

const props = defineProps<{
  items:    DropdownItem<T>[]
  /** label is the trigger's accessible name. */
  label:    string
  /** compact shows only the icon of the selected item. */
  compact?: boolean
  up?:      boolean
  right?:   boolean
}>()

const model = defineModel<T>({ required: true })

const menuId = useId()
const open = ref(false)
const root = useTemplateRef<HTMLElement>("root")
const trigger = useTemplateRef<HTMLButtonElement>("trigger")
const buttons = useTemplateRef<HTMLButtonElement[]>("buttons")
const selected = computed(
  () => props.items.find(item => item.value === model.value),
)

async function show(index: number) {
  open.value = true
  await nextTick()
  const all = buttons.value ?? []
  all.at(index === -1 ? all.length - 1 : index)?.focus()
}

function close(refocus: boolean) {
  open.value = false
  if (refocus) {
    trigger.value?.focus()
  }
}

function select(value: T) {
  model.value = value
  close(true)
}

function onMenuKey(event: KeyboardEvent) {
  const all = buttons.value ?? []
  const current = all.indexOf(document.activeElement as HTMLButtonElement)
  const moves: Record<string, number> = {
    ArrowDown: (current + 1) % all.length,
    ArrowUp:   (current - 1 + all.length) % all.length,
    Home:      0,
    End:       all.length - 1,
  }
  if (event.key in moves) {
    event.preventDefault()
    all[moves[event.key]!]?.focus()
  } else if (event.key === "Escape") {
    event.preventDefault()
    close(true)
  } else if (event.key === "Tab") {
    close(false)
  }
}

function onFocusOut(event: FocusEvent) {
  if (!root.value?.contains(event.relatedTarget as Node | null)) {
    close(false)
  }
}
</script>

<template>
  <div
      ref="root"
      class="dropdown"
      :class="{ 'is-active': open, 'is-up': up, 'is-right': right }"
      @focusout="onFocusOut"
  >
    <div class="dropdown-trigger">
      <button
          ref="trigger"
          type="button"
          class="button"
          :class="{ 'is-small': compact }"
          :aria-label="label"
          aria-haspopup="menu"
          :aria-expanded="open"
          :aria-controls="menuId"
          @click="open ? close(false) : show(selected ? items.indexOf(selected) : 0)"
          @keydown.down.prevent="show(0)"
          @keydown.up.prevent="show(-1)"
      >
        <span
            v-if="selected?.icon"
            class="icon is-small"
        ><component
            :is="selected.icon"
            aria-hidden="true"
        /></span>
        <span
            v-if="selected && !compact"
            :lang="selected.lang"
        >{{ selected.label }}</span>
        <span class="icon is-small"><ChevronDown aria-hidden="true" /></span>
      </button>
    </div>
    <div
        :id="menuId"
        class="dropdown-menu"
        role="menu"
        :aria-label="label"
        @keydown="onMenuKey"
    >
      <div class="dropdown-content">
        <button
            v-for="item in items"
            :key="item.value"
            ref="buttons"
            type="button"
            role="menuitemradio"
            :aria-checked="item.value === model"
            class="dropdown-item"
            :class="{ 'is-active': item.value === model }"
            :lang="item.lang"
            tabindex="-1"
            @click="select(item.value)"
        >
          <span
              v-if="item.icon"
              class="icon is-small"
          ><component
              :is="item.icon"
              aria-hidden="true"
          /></span>
          <span>{{ item.label }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Lucide leaves a twelfth of an icon empty around its drawing; lowering the
   icon by as much puts the drawing on the label's baseline. */
.dropdown-item .icon {
  margin-inline-end: var(--bulma-icon-text-spacing);
  vertical-align: calc(var(--bulma-icon-dimensions-small) / -12);
}
</style>
