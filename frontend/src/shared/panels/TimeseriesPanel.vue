<script setup lang="ts">
import "uplot/dist/uPlot.min.css"

import uPlot from "uplot"
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  useTemplateRef,
  watch,
} from "vue"
import { useI18n } from "vue-i18n"

import {
  fractionDigits,
  latest,
  seriesColor,
  toggled,
  tooltipPosition,
  yRange,
} from "../chart"
import StaleTag from "../components/StaleTag.vue"
import { formatDateTime, formatNumber, formatRange, formatTick } from "../format"
import type { PanelData, PanelInfo, SeriesData } from "../payload"

const props = defineProps<{
  panel:    PanelInfo
  data?:    PanelData
  timezone: string
}>()

const { t, locale } = useI18n()
const wrapper = useTemplateRef<HTMLElement>("wrapper")
const canvas = useTemplateRef<HTMLElement>("canvas")
const tip = useTemplateRef<HTMLElement>("tip")

/** hidden lists the names of series the visitor switched off. */
const hidden = ref<string[]>([])
const tooltip = ref<{
  left: number
  top:  number
  time: string
  rows: {
    color: string
    name:  string
    value: string
  }[]
} | null>(null)
let plot: uPlot | null = null
let observer: ResizeObserver | null = null

const series = computed(
  () => props.data?.state === "pending"
    ? undefined
    : props.data?.data as SeriesData | undefined,
)
const hasData = computed(
  () => !!series.value?.times.length && !!series.value.series.length,
)
const decimals = computed(() => props.panel.decimals ?? 2)

function format(v: number | null | undefined): string {
  const unit = props.panel.unit ? ` ${props.panel.unit}` : ""
  return v === null || v === undefined
    ? "-"
    : formatNumber(v, locale.value, decimals.value) + unit
}

const summary = computed(() => t("panel.chartSummary", {
  title:  props.panel.title,
  range:  formatRange(props.panel.range ?? 0, locale.value),
  values: series.value
    ? latest(series.value)
      .map((v, i) => `${series.value!.series[i]!.name} ${format(v)}`)
      .join(", ")
    : "",
}))

function aligned(s: SeriesData): uPlot.AlignedData {
  return [s.times, ...s.series.map(x => x.values)]
}

const axis = {
  stroke: "#8892a0",
  grid:   {
    stroke: "rgb(136 146 160 / 20%)",
    width:  1,
  },
  ticks: {
    stroke: "rgb(136 146 160 / 20%)",
    width:  1,
  },
}

function options(s: SeriesData): uPlot.Options {
  const lang = locale.value
  const span = (props.panel.range ?? 0) || (s.times.at(-1)! - s.times[0]!)
  return {
    width:  wrapper.value!.clientWidth,
    height: 220,
    legend: { show: false },
    cursor: {
      drag: {
        x: false,
        y: false,
      },
    },
    select: {
      show:   false,
      left:   0,
      top:    0,
      width:  0,
      height: 0,
    },
    tzDate: ts => uPlot.tzDate(new Date(ts * 1000), props.timezone),
    scales: {
      y: {
        range: (_u, min, max) =>
          yRange(min, max, !!props.panel.minZero),
      },
    },
    axes: [
      {
        ...axis,
        values: (_u, splits) => splits.map(
          v => formatTick(new Date(v * 1000), lang, props.timezone, span),
        ),
      },
      {
        ...axis,
        size: (_u, values) =>
          16 + 7 * Math.max(0, ...(values ?? []).map(v => v.length)),
        values: (_u, splits) => {
          const digits = splits.length > 1
            ? fractionDigits(Math.abs(splits[1]! - splits[0]!))
            : 0
          const unit = props.panel.unit ? ` ${props.panel.unit}` : ""
          return splits.map(v => formatNumber(v, lang, digits) + unit)
        },
      },
    ],
    series: [{}, ...s.series.map((x, i) => ({
      label:    x.name,
      stroke:   seriesColor(i),
      width:    2,
      fill:     props.panel.style === "area" ? `${seriesColor(i)}33` : undefined,
      show:     !hidden.value.includes(x.name),
      spanGaps: false,
    }))],
    hooks: { setCursor: [showTooltip] },
  }
}

/** showTooltip shows the values at the cursor next to it. */
async function showTooltip(u: uPlot) {
  const idx = u.cursor.idx
  const s = series.value
  if (idx === null || idx === undefined || !s
    || u.cursor.left === undefined || u.cursor.left < 0) {
    tooltip.value = null
    return
  }

  const rows = s.series.flatMap((x, i) => hidden.value.includes(x.name)
    ? []
    : [{
      color: seriesColor(i),
      name:  x.name,
      value: format(x.values[idx]),
    }])
  tooltip.value = {
    left: 0,
    top:  0,
    time: formatDateTime(
      new Date(s.times[idx]! * 1000),
      locale.value,
      props.timezone,
    ),
    rows,
  }

  await nextTick()
  if (!tip.value || !wrapper.value) {
    return
  }

  const area = wrapper.value.getBoundingClientRect()
  const over = u.over.getBoundingClientRect()
  const pos = tooltipPosition(
    over.left - area.left + u.cursor.left,
    over.top - area.top + (u.cursor.top ?? 0),
    tip.value.getBoundingClientRect(),
    area,
  )
  tooltip.value = {
    ...tooltip.value!,
    ...pos,
  }
}

function create() {
  plot?.destroy()
  plot = null
  tooltip.value = null
  if (hasData.value && wrapper.value && canvas.value) {
    plot = new uPlot(options(series.value!), aligned(series.value!), canvas.value)
  }
}

// New data of the same series is updated in place, so that an open
// tooltip and hidden series survive the refresh.
watch(series, (next, prev) => {
  const same = plot && next && prev && next.series.length === prev.series.length
    && next.series.every((x, i) => x.name === prev.series[i]!.name)
  if (same) {
    plot!.setData(aligned(next))
  } else {
    void nextTick(create)
  }
})
watch(
  [locale, () => props.timezone, () => props.panel],
  () => void nextTick(create),
)

function toggle(name: string, i: number) {
  hidden.value = toggled(hidden.value, name)
  plot?.setSeries(i + 1, { show: !hidden.value.includes(name) })
}

onMounted(() => {
  create()
  observer = new ResizeObserver(() => plot?.setSize({
    width:  wrapper.value!.clientWidth,
    height: 220,
  }))
  observer.observe(wrapper.value!)
})
onBeforeUnmount(() => {
  observer?.disconnect()
  plot?.destroy()
})
</script>

<template>
  <div
      class="box sr-chart"
      :data-panel-id="panel.id"
  >
    <div class="sr-panel-head">
      <h3 class="title is-6 mb-0">
        {{ panel.title }}
      </h3>
      <StaleTag
          v-if="data?.state === 'stale'"
          :fetched-at="data.fetchedAt!"
          :timezone
      />
      <slot name="actions" />
    </div>
    <p
        v-if="panel.description"
        class="sr-muted"
    >
      {{ panel.description }}
    </p>
    <p
        v-if="!hasData"
        class="sr-muted sr-chart-empty"
    >
      {{ series ? t("panel.empty") : t("panel.noData") }}
    </p>
    <div
        v-show="hasData"
        ref="wrapper"
        class="sr-chart-area"
        @mouseleave="tooltip = null"
    >
      <div
          ref="canvas"
          role="img"
          :aria-label="summary"
      />
      <div
          v-if="tooltip"
          ref="tip"
          class="box sr-tooltip"
          :style="{ left: `${tooltip.left}px`, top: `${tooltip.top}px` }"
      >
        <div class="has-text-weight-semibold">
          {{ tooltip.time }}
        </div>
        <div
            v-for="row in tooltip.rows"
            :key="row.name"
            class="sr-tooltip-row"
        >
          <span
              class="sr-key"
              :style="{ background: row.color }"
          />
          <span class="sr-tooltip-value">{{ row.value }}</span>
          <span>{{ row.name }}</span>
        </div>
      </div>
    </div>
    <div
        v-if="hasData && series!.series.length > 1"
        class="buttons are-small mt-2"
    >
      <button
          v-for="(s, i) in series!.series"
          :key="s.name"
          type="button"
          class="button sr-legend-entry"
          :class="{ 'is-off': hidden.includes(s.name) }"
          :aria-pressed="!hidden.includes(s.name)"
          @click="toggle(s.name, i)"
      >
        <span
            class="sr-key"
            :style="{ background: seriesColor(i) }"
        />
        <span>{{ s.name }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.sr-chart {
  margin: 0;
}

.sr-chart-area {
  position: relative;
  min-height: 220px;
  margin-top: 0.75rem;
}

.sr-chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 220px;
}

.sr-tooltip {
  position: absolute;
  z-index: 5;
  margin: 0;
  padding: 0.5rem 0.75rem;
  font-size: 0.875rem;
  white-space: nowrap;
  pointer-events: none;
}

.sr-tooltip-row {
  display: flex;
  gap: 0.5rem;
  align-items: center;
}

.sr-tooltip-value {
  font-variant-numeric: tabular-nums;
}

.sr-key {
  display: inline-block;
  flex: none;
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 2px;
}

.sr-legend-entry {
  gap: 0.375rem;
}

.sr-legend-entry.is-off {
  text-decoration: line-through;
  opacity: 0.5;
}
</style>
