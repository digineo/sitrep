import { CircleCheck, CircleMinus, CircleQuestionMark, CircleX } from "@lucide/vue"
import type { Component } from "vue"

import type { State } from "./payload"

/** banners maps a status to the banner's color class and icon. */
export const banners: Record<State, {
  color: string
  icon:  Component
}> = {
  operational: {
    color: "is-success",
    icon:  CircleCheck,
  },
  degraded: {
    color: "is-warning",
    icon:  CircleMinus,
  },
  down: {
    color: "is-danger",
    icon:  CircleX,
  },
  unknown: {
    color: "sr-is-unknown",
    icon:  CircleQuestionMark,
  },
}
