import "../shared/styles/main.scss"

import { createHead } from "@unhead/vue/client"
import { createApp } from "vue"
import { createRouter, createWebHistory } from "vue-router"

import { readBootstrap } from "../shared/bootstrap"
import { createAppI18n } from "../shared/i18n"
import { focusHeadingOnNavigation } from "../shared/routeFocus"
import App from "./App.vue"
import { pageRoutes } from "./urls"
import ArchiveView from "./views/ArchiveView.vue"
import IncidentView from "./views/IncidentView.vue"
import LandingView from "./views/LandingView.vue"
import LegalView from "./views/LegalView.vue"
import NotFoundView from "./views/NotFoundView.vue"
import SiteOverview from "./views/SiteOverview.vue"

const bootstrap = readBootstrap()
const views = bootstrap.mode === "landing"
  ? {
    overview: LandingView,
    imprint:  LegalView,
    privacy:  LegalView,
  }
  : {
    overview: SiteOverview,
    archive:  ArchiveView,
    incident: IncidentView,
    imprint:  LegalView,
    privacy:  LegalView,
  }
const router = createRouter({
  history: createWebHistory(bootstrap.basePath),
  routes:  [
    ...pageRoutes(bootstrap.languages, views),
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
