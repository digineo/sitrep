import { nextTick } from "vue"
import { type Router, START_LOCATION } from "vue-router"

/**
 * focusHeadingOnNavigation moves the focus to the main heading and scrolls
 * to the top after each route change, so that screen readers announce the
 * new view.
 */
export function focusHeadingOnNavigation(router: Router) {
  router.afterEach(async(_to, from, failure) => {
    if (failure || from === START_LOCATION) {
      return
    }

    await nextTick()
    window.scrollTo(0, 0)
    const h1 = document.querySelector("h1")
    if (h1) {
      h1.tabIndex = -1
      h1.focus()
    }
  })
}
