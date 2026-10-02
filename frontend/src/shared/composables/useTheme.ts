import { readonly, ref } from "vue"

export type Theme = "light" | "dark" | "system"

// The server stamps data-theme for the first paint: the visitor's choice,
// else the default. Without a stamp, the system's color scheme applies.
function initial(): Theme {
  const cookie = document.cookie
    .match(/(?:^|; )theme=(light|dark|system)(?:;|$)/)?.[1]
  return (cookie ?? document.documentElement.dataset.theme ?? "system") as Theme
}

const theme = ref<Theme>(initial())

/** useTheme reads and changes the color scheme. */
export function useTheme() {
  /** setTheme applies and remembers the visitor's choice. */
  function setTheme(t: Theme) {
    theme.value = t
    document.cookie = `theme=${t}; Path=/; SameSite=Lax; Max-Age=31536000`
    if (t === "system") {
      delete document.documentElement.dataset.theme
    } else {
      document.documentElement.dataset.theme = t
    }
  }
  return {
    theme: readonly(theme),
    setTheme,
  }
}
