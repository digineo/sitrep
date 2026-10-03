/**
 * luminance returns the relative luminance of a "#rrggbb" color, as WCAG
 * defines it.
 */
function luminance(color: string): number {
  const channel = (i: number) => {
    const c = Number.parseInt(color.slice(i, i + 2), 16) / 255
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  }
  return 0.2126 * channel(1) + 0.7152 * channel(3) + 0.0722 * channel(5)
}

/**
 * textOn returns black or white, whichever has the higher contrast ratio on
 * the background color "#rrggbb".
 */
export function textOn(background: string): "#000000" | "#ffffff" {
  const l = luminance(background)
  return (l + 0.05) / 0.05 >= 1.05 / (l + 0.05) ? "#000000" : "#ffffff"
}

/**
 * bandStyle returns the colors of a header or footer band in a brand color,
 * for the sr-band class.
 */
export function bandStyle(
  color: string | undefined,
): Record<string, string> | undefined {
  return color
    ? {
      "--sr-band":      color,
      "--sr-band-text": textOn(color),
    }
    : undefined
}
