import "../shared/styles/main.scss"

import { createHead } from "@unhead/vue/client"
import { createPinia } from "pinia"
import { createApp } from "vue"

import { readBootstrap } from "../shared/bootstrap"
import { createAppI18n } from "../shared/i18n"
import { focusHeadingOnNavigation } from "../shared/routeFocus"
import App from "./App.vue"
import { router } from "./router"

focusHeadingOnNavigation(router)

createApp(App)
  .use(createPinia())
  .use(createAppI18n(readBootstrap().lang))
  .use(createHead())
  .use(router)
  .mount("#app")
