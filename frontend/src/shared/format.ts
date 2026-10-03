// Formatting helpers. Every number and time is formatted with Intl in the
// active language; times of a site in its time zone, with the zone shown.

/** formatNumber formats with exactly `decimals` fraction digits and grouping. */
export function formatNumber(
  value: number,
  lang: string,
  decimals: number,
): string {
  return new Intl.NumberFormat(lang, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(value)
}

/**
 * formatMiB formats a size in bytes as mebibytes with one fraction digit,
 * without the unit.
 */
export function formatMiB(bytes: number, lang: string): string {
  return formatNumber(bytes / 2 ** 20, lang, 1)
}

/** formatClock returns the time of day with the zone, e.g. "14:05 MESZ". */
export function formatClock(time: Date, lang: string, timeZone: string): string {
  return new Intl.DateTimeFormat(lang, {
    hour:         "numeric",
    minute:       "2-digit",
    timeZone,
    timeZoneName: "short",
  }).format(time)
}

/** formatDateTime returns date and time with the zone. */
export function formatDateTime(time: Date, lang: string, timeZone: string): string {
  return new Intl.DateTimeFormat(lang, {
    year:         "numeric",
    month:        "short",
    day:          "numeric",
    hour:         "numeric",
    minute:       "2-digit",
    timeZone,
    timeZoneName: "short",
  }).format(time)
}

/**
 * formatTick returns an axis label for a time: the time of day for spans
 * up to two days, else the date.
 */
export function formatTick(
  time: Date,
  lang: string,
  timeZone: string,
  span: number,
): string {
  const options: Intl.DateTimeFormatOptions = span <= 2 * 86400
    ? {
      hour:   "numeric",
      minute: "2-digit",
      timeZone,
    }
    : {
      month: "short",
      day:   "numeric",
      timeZone,
    }
  return new Intl.DateTimeFormat(lang, options).format(time)
}

const units = [
  ["day", 86400],
  ["hour", 3600],
  ["minute", 60],
  ["second", 1],
] as const

/** formatRange returns a duration in its largest whole unit, e.g. "2 hours". */
export function formatRange(seconds: number, lang: string): string {
  const [unit, size] = units.find(([, size]) => seconds % size === 0) ?? units[3]
  return new Intl.NumberFormat(lang, {
    style:       "unit",
    unit,
    unitDisplay: "long",
  }).format(seconds / size)
}
