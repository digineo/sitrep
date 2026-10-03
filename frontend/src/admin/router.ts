import { createRouter, createWebHistory } from "vue-router"

import DataSourceFormView from "./views/DataSourceFormView.vue"
import DataSourcesView from "./views/DataSourcesView.vue"
import HomeView from "./views/HomeView.vue"
import IncidentCreateView from "./views/IncidentCreateView.vue"
import IncidentEditView from "./views/IncidentEditView.vue"
import IncidentsView from "./views/IncidentsView.vue"
import NotFoundView from "./views/NotFoundView.vue"
import PanelEditorView from "./views/PanelEditorView.vue"
import SettingsView from "./views/SettingsView.vue"
import SiteCreateView from "./views/SiteCreateView.vue"
import SitePreviewView from "./views/SitePreviewView.vue"
import SiteSettingsView from "./views/SiteSettingsView.vue"

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
      path:      "/datasources",
      component: DataSourcesView,
    },
    {
      path:      "/datasources/new",
      component: DataSourceFormView,
    },
    {
      path:      "/datasources/:id",
      component: DataSourceFormView,
    },
    {
      path:      "/sites/new",
      component: SiteCreateView,
    },
    {
      path:      "/sites/:site",
      component: SitePreviewView,
    },
    {
      path:      "/sites/:site/settings",
      component: SiteSettingsView,
    },
    {
      path:      "/sites/:site/panels/new",
      component: PanelEditorView,
    },
    {
      path:      "/sites/:site/panels/:panel",
      component: PanelEditorView,
    },
    {
      path:      "/sites/:site/incidents",
      component: IncidentsView,
    },
    {
      path:      "/sites/:site/incidents/new",
      component: IncidentCreateView,
    },
    {
      path:      "/sites/:site/incidents/:incident",
      component: IncidentEditView,
    },
    {
      path:      "/:path(.*)*",
      component: NotFoundView,
    },
  ],
})
