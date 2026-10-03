<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId } from "vue"
import { useI18n } from "vue-i18n"

import { formatClock } from "../format"

const props = defineProps<{
  state:      "loading" | "ok" | "failed"
  /** updatedAt is the time of the latest successful refresh. */
  updatedAt?: Date
  timezone:   string
  /** pulse changes with every successful refresh. */
  pulse:      number
  /** error is the localized error of a failed refresh. */
  error?:     string
  /** detail is the technical detail of a failed refresh. */
  detail?:    string
}>()

const { t, locale } = useI18n()
const popoverId = useId()
const open = ref(false)
let closing: ReturnType<typeof setTimeout> | undefined

const stateText = computed(() => t(`live.state.${props.state}`))

function show() {
  clearTimeout(closing)
  open.value = props.state === "failed"
}

// Leaving the indicator closes the popover after a grace period, so that
// the pointer can move into it.
function hide() {
  clearTimeout(closing)
  closing = setTimeout(() => (open.value = false), 300)
}

onBeforeUnmount(() => clearTimeout(closing))
</script>

<template>
  <span
      class="sr-live"
      @mouseenter="show"
      @mouseleave="hide"
  >
    <button
        type="button"
        class="sr-live-trigger"
        :aria-expanded="state === 'failed' ? open : undefined"
        :aria-controls="state === 'failed' ? popoverId : undefined"
        :disabled="state !== 'failed' || undefined"
        @click="open = !open && state === 'failed'"
    >
      <span
          :key="pulse"
          class="sr-live-dot"
          :class="`is-${state}`"
          aria-hidden="true"
      />
      <span class="sr-visually-hidden">{{ stateText }}</span>
      <span v-if="updatedAt">{{ t("live.updated", { time: formatClock(updatedAt, locale, timezone) }) }}</span>
    </button>
    <span
        v-if="open && state === 'failed'"
        :id="popoverId"
        class="box sr-live-popover"
        role="status"
    >
      <span class="is-block">{{ t("live.lastFailed", { error }) }}</span>
      <code
          v-if="detail"
          class="is-block mt-2"
      >{{ detail }}</code>
    </span>
  </span>
</template>

<style scoped>
.sr-live {
  position: relative;
  display: inline-block;
}

.sr-live-trigger {
  display: inline-flex;
  gap: 0.5rem;
  align-items: center;
  padding: 0;
  font: inherit;
  color: inherit;
  background: none;
  border: none;
}

.sr-live-trigger:not(:disabled) {
  cursor: pointer;
}

.sr-live-dot {
  position: relative;
  width: 0.625rem;
  height: 0.625rem;
  background: #8892a0;
  border-radius: 50%;
}

.sr-live-dot.is-ok {
  background: #2f9e6b;
}

.sr-live-dot.is-failed {
  background: #e5484d;
}

/* One expanding pulse per successful refresh. */
.sr-live-dot.is-ok::after {
  position: absolute;
  inset: 0;
  content: "";
  border: 2px solid #2f9e6b;
  border-radius: 50%;
  opacity: 0;
  animation: sr-pulse 1s ease-out;
}

@keyframes sr-pulse {
  from {
    opacity: 1;
    transform: scale(1);
  }

  to {
    opacity: 0;
    transform: scale(2.5);
  }
}

@media (prefers-reduced-motion: reduce) {
  .sr-live-dot.is-ok::after {
    animation: none;
  }
}

.sr-live-popover {
  position: absolute;
  top: calc(100% + 0.5rem);
  left: 0;
  z-index: 10;
  width: max-content;
  max-width: min(24rem, 90vw);
  user-select: text;
}

code {
  white-space: pre-wrap;
}
</style>
