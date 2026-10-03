import { describe, expect, it } from "vitest"

import {
  formatClock,
  formatDateTime,
  formatMiB,
  formatNumber,
  formatRange,
  formatTick,
  fromZonedInput,
  toZonedInput,
} from "./format"

const noon = new Date(Date.UTC(2026, 9, 2, 12, 5))

describe("formatNumber", () => {
  it("uses the language's separators and exactly the decimals", () => {
    expect(formatNumber(1234.5, "en", 2)).toBe("1,234.50")
    expect(formatNumber(1234.5, "de", 2)).toBe("1.234,50")
    expect(formatNumber(1234.5, "de", 0)).toBe("1.235")
    expect(formatNumber(0.1234567, "en", 6)).toBe("0.123457")
  })

  it("formats sizes in mebibytes", () => {
    expect(formatMiB(1.5 * 2 ** 20, "de")).toBe("1,5")
    expect(formatMiB(10 * 2 ** 20, "en")).toBe("10.0")
  })
})

describe("times", () => {
  it("shows the time in the site's zone with the zone", () => {
    expect(formatClock(noon, "de", "Europe/Berlin")).toBe("14:05 MESZ")
    expect(formatClock(noon, "en", "UTC")).toBe("12:05 PM UTC")
    expect(formatDateTime(noon, "de", "Europe/Berlin"))
      .toBe("2. Okt. 2026, 14:05 MESZ")
    expect(formatDateTime(noon, "en", "America/New_York"))
      .toBe("Oct 2, 2026, 8:05 AM EDT")
  })

  it("labels ticks with the time of day or the date", () => {
    expect(formatTick(noon, "de", "UTC", 3600)).toBe("12:05")
    expect(formatTick(noon, "en", "Asia/Tokyo", 7 * 86400)).toBe("Oct 2")
  })
})

describe("formatRange", () => {
  it("uses the largest whole unit", () => {
    expect(formatRange(7200, "en")).toBe("2 hours")
    expect(formatRange(86400 * 7, "de")).toBe("7 Tage")
    expect(formatRange(5400, "en")).toBe("90 minutes")
    expect(formatRange(61, "de")).toBe("61 Sekunden")
  })
})

describe("zoned inputs", () => {
  it("shows a time in the zone, to the minute", () => {
    expect(toZonedInput(new Date(Date.UTC(2026, 9, 2, 12, 5, 42)), "Europe/Berlin"))
      .toBe("2026-10-02T14:05")
    expect(toZonedInput(new Date(Date.UTC(2026, 9, 2, 23, 30)), "UTC"))
      .toBe("2026-10-02T23:30")
    expect(toZonedInput(new Date(Date.UTC(2026, 9, 2, 0, 0)), "America/New_York"))
      .toBe("2026-10-01T20:00")
  })

  it("reads a value in the zone", () => {
    expect(fromZonedInput("2026-10-02T14:05", "Europe/Berlin").toISOString())
      .toBe("2026-10-02T12:05:00.000Z")
    expect(fromZonedInput("2026-01-15T14:05", "Europe/Berlin").toISOString())
      .toBe("2026-01-15T13:05:00.000Z")
    expect(fromZonedInput("2026-10-01T20:00", "America/New_York").toISOString())
      .toBe("2026-10-02T00:00:00.000Z")
    expect(fromZonedInput("2026-10-02T14:05", "Asia/Kolkata").toISOString())
      .toBe("2026-10-02T08:35:00.000Z")
  })

  it("handles daylight saving changes", () => {
    // Berlin skips 02:00 to 03:00 on 2026-03-29 and repeats
    // 02:00 to 03:00 on 2026-10-25.
    expect(fromZonedInput("2026-03-29T01:30", "Europe/Berlin").toISOString())
      .toBe("2026-03-29T00:30:00.000Z")
    expect(fromZonedInput("2026-03-29T03:30", "Europe/Berlin").toISOString())
      .toBe("2026-03-29T01:30:00.000Z")
    expect(fromZonedInput("2026-03-29T02:30", "Europe/Berlin").toISOString())
      .toBe("2026-03-29T01:30:00.000Z")
    expect(fromZonedInput("2026-10-25T02:30", "Europe/Berlin").toISOString())
      .toBe("2026-10-25T00:30:00.000Z")
  })
})
