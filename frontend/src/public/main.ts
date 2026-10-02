import "../shared/styles/main.scss"

import { createHead } from "@unhead/vue/client"
import { createApp } from "vue"
import { createRouter, createWebHistory } from "vue-router"

import { readBootstrap } from "../shared/bootstrap"
import { createAppI18n } from "../shared/i18n"
import { focusHeadingOnNavigation } from "../shared/routeFocus"
import App from "./App.vue"
import { pageRoutes } from "./urls"
import LandingView from "./views/LandingView.vue"
import NotFoundView from "./views/NotFoundView.vue"

const bootstrap = readBootstrap()
const router = createRouter({
  history: createWebHistory(bootstrap.basePath),
  routes:  [
    ...(bootstrap.mode === "landing"
      ? pageRoutes(bootstrap.languages, { overview: LandingView })
      : []),
    {
      path:      "/:path(.*)*",
      component: NotFoundView,
    },
  ],
})
focusHeadingOnNavigation(router)

createApp(App)
  .provide("bootstrap", bootstrap)
  .use(createAppI18n(bootstrap.lang))
  .use(createHead())
  .use(router)
  .mount("#app")
