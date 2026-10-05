import { createRouter, createWebHistory } from "vue-router"

import type { Role } from "./roles"
import AccountsView from "./views/AccountsView.vue"
import DataSourceFormView from "./views/DataSourceFormView.vue"
import DataSourcesView from "./views/DataSourcesView.vue"
import HomeView from "./views/HomeView.vue"
import IncidentCreateView from "./views/IncidentCreateView.vue"
import IncidentEditView from "./views/IncidentEditView.vue"
import IncidentsView from "./views/IncidentsView.vue"
import MembersView from "./views/MembersView.vue"
import NotFoundView from "./views/NotFoundView.vue"
import PanelEditorView from "./views/PanelEditorView.vue"
import SettingsView from "./views/SettingsView.vue"
import SiteCreateView from "./views/SiteCreateView.vue"
import SitePreviewView from "./views/SitePreviewView.vue"
import SiteSettingsView from "./views/SiteSettingsView.vue"

declare module "vue-router" {
  interface RouteMeta {
    /** role is required to see the route: on its site, or anywhere. */
    role?: Role
  }
}

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
      meta:      { role: "admin" },
    },
    {
      path:      "/accounts",
      component: AccountsView,
      meta:      { role: "owner" },
    },
    {
      path:      "/datasources",
      component: DataSourcesView,
      meta:      { role: "admin" },
    },
    {
      path:      "/datasources/new",
      component: DataSourceFormView,
      meta:      { role: "admin" },
    },
    {
      path:      "/datasources/:id",
      component: DataSourceFormView,
      meta:      { role: "admin" },
    },
    {
      path:      "/sites/new",
      component: SiteCreateView,
      meta:      { role: "admin" },
    },
    {
      path:      "/sites/:site",
      component: SitePreviewView,
      meta:      { role: "responder" },
    },
    {
      path:      "/sites/:site/settings",
      component: SiteSettingsView,
      meta:      { role: "maintainer" },
    },
    {
      path:      "/sites/:site/members",
      component: MembersView,
      meta:      { role: "maintainer" },
    },
    {
      path:      "/sites/:site/panels/new",
      component: PanelEditorView,
      meta:      { role: "maintainer" },
    },
    {
      path:      "/sites/:site/panels/:panel",
      component: PanelEditorView,
      meta:      { role: "maintainer" },
    },
    {
      path:      "/sites/:site/incidents",
      component: IncidentsView,
      meta:      { role: "responder" },
    },
    {
      path:      "/sites/:site/incidents/new",
      component: IncidentCreateView,
      meta:      { role: "responder" },
    },
    {
      path:      "/sites/:site/incidents/:incident",
      component: IncidentEditView,
      meta:      { role: "responder" },
    },
    {
      path:      "/:path(.*)*",
      component: NotFoundView,
    },
  ],
})
