import { createRouter, createWebHistory } from "vue-router"

import HomeView from "./views/HomeView.vue"
import NotFoundView from "./views/NotFoundView.vue"
import SettingsView from "./views/SettingsView.vue"

export const router = createRouter({
  history: createWebHistory("/admin/"),
  routes:  [
    {
      path:      "/",
      component: HomeView,
    },
    {
      path:      "/settings",
      component: SettingsView,
    },
    {
      path:      "/:path(.*)*",
      component: NotFoundView,
    },
  ],
})
