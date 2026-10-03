import { nextTick } from "vue"
import { type Router, START_LOCATION } from "vue-router"

/**
 * focusHeadingOnNavigation moves the focus to the main heading and scrolls
 * to the top after each route change, so that screen readers announce the
 * new view. The main heading is the one marked with data-main-heading, or
 * else the first h1.
 */
export function focusHeadingOnNavigation(router: Router) {
  router.afterEach(async(_to, from, failure) => {
    if (failure || from === START_LOCATION) {
      return
    }

    await nextTick()
    window.scrollTo(0, 0)
    const heading = document.querySelector<HTMLElement>("[data-main-heading]")
      ?? document.querySelector("h1")
    if (heading) {
      heading.tabIndex = -1
      heading.focus()
    }
  })
}
