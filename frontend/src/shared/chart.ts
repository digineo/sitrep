import type { SeriesData } from "./payload"

/** palette holds the series colors, legible on light and dark backgrounds. */
export const palette = [
  "#2f6feb", "#e0a52b", "#2f9e6b", "#c2459f", "#0f9fb4", "#8b6fe0",
]

/** seriesColor returns the color of the i-th series. */
export function seriesColor(i: number): string {
  return palette[i % palette.length]!
}

/**
 * yRange returns the y-axis range with some headroom. With minZero, the
 * axis starts at 0 unless the data has negative values.
 */
export function yRange(
  min: number | null,
  max: number | null,
  minZero: boolean,
): [number, number] {
  if (min === null || max === null) {
    return [0, 1]
  }

  const pad = (max - min || Math.abs(max) || 1) * 0.05
  return [minZero && min >= 0 ? 0 : min - pad, max + pad]
}

/** fractionDigits returns the fraction digits that tell ticks `step` apart. */
export function fractionDigits(step: number): number {
  for (let d = 0; d < 6; d++) {
    const scaled = step * 10 ** d
    if (Math.abs(scaled - Math.round(scaled)) < 1e-9 * Math.max(1, scaled)) {
      return d
    }
  }
  return 6
}

export interface Size {
  width:  number
  height: number
}

/**
 * tooltipPosition places a box next to the cursor within an area, on the
 * other side of the cursor where it would not fit.
 */
export function tooltipPosition(
  x: number,
  y: number,
  box: Size,
  area: Size,
  gap = 12,
): {
  left: number
  top:  number
} {
  let left = x + gap
  if (left + box.width > area.width) {
    left = x - gap - box.width
  }

  let top = y + gap
  if (top + box.height > area.height) {
    top = y - gap - box.height
  }
  return {
    left: Math.max(0, left),
    top:  Math.max(0, top),
  }
}

/** toggled returns the hidden series names with name toggled. */
export function toggled(hidden: string[], name: string): string[] {
  return hidden.includes(name) ? hidden.filter(n => n !== name) : [...hidden, name]
}

/** latest returns the latest value of each series, or null without any. */
export function latest(data: SeriesData): (number | null)[] {
  return data.series.map(s => s.values.findLast(v => v !== null) ?? null)
}
