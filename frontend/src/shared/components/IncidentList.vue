<script setup lang="ts">
import type { RouteLocationRaw } from "vue-router"

import type { Incident } from "../payload"
import IncidentEntry from "./IncidentEntry.vue"

defineProps<{
  title?:    string
  incidents: Incident[]
  timezone:  string
  /** to returns the location of an incident's page. */
  to:        (id: string) => RouteLocationRaw
}>()
</script>

<template>
  <section class="block">
    <h2
        v-if="title"
        class="title is-5"
    >
      {{ title }}
    </h2>
    <ul>
      <li
          v-for="incident in incidents"
          :key="incident.id"
          class="block"
      >
        <IncidentEntry
            :incident
            :to="to(incident.id)"
            :timezone
        />
      </li>
    </ul>
  </section>
</template>
