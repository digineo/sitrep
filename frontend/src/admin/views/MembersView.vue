<script setup lang="ts">
import { Trash2 } from "@lucide/vue"
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"
import { useRoute } from "vue-router"

import SRTag from "../../shared/components/SRTag.vue"
import { useConfirm } from "../../shared/composables/useConfirm"
import { api } from "../api"
import GrantDialog from "../components/GrantDialog.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { type Role, siteRoles } from "../roles"
import { resolveText } from "../rules"
import { useNotices } from "../stores/notices"
import { useOverview } from "../stores/overview"
import { useSession } from "../stores/session"
import type { Member } from "../types"

const { t, locale } = useI18n()
const route = useRoute()
const notices = useNotices()
const overview = useOverview()
const session = useSession()
const { confirm } = useConfirm()

const id = route.params.site as string
const base = `/api/admin/sites/${id}/members`
const site = computed(() => {
  const s = overview.sites?.find(x => x.id === id)
  return s ? resolveText(s.name, locale.value, s.languages) : ""
})
usePageTitle(() => t("members.titleOf", { site: site.value }))

const members = ref<Member[] | null>(null)
const sorted = computed(() => {
  const collator = new Intl.Collator(locale.value)
  return [...members.value ?? []]
    .sort((a, b) => collator.compare(a.displayName, b.displayName))
})

async function load() {
  try {
    members.value = await api<Member[]>("GET", base)
  } catch(err) {
    notices.loadFailed(err)
  }
}

onMounted(load)

async function change(member: Member, event: Event) {
  const next = (event.target as HTMLSelectElement).value as Role
  try {
    Object.assign(member, await api<Member>("PUT", `${base}/${member.id}`, { role: next }))
    notices.success(t("members.saved"))
  } catch(err) {
    notices.failure(t("toast.saveFailed"), err)
    await load()
  }
}

async function remove(member: Member) {
  const ok = await confirm({
    title:   t("members.removeTitle"),
    message: t("members.removeMessage", {
      name: member.displayName,
      site: site.value,
    }),
    confirm: t("members.remove"),
    danger:  true,
  })
  if (!ok) {
    return
  }

  try {
    await api("DELETE", `${base}/${member.id}`)
    await load()
  } catch(err) {
    notices.failure(t("toast.deleteFailed"), err)
  }
}
</script>

<template>
  <div class="level">
    <h1 class="title mb-0">
      {{ t("members.titleOf", { site }) }}
    </h1>
    <GrantDialog
        :title="t('members.add')"
        :path="base"
        :roles="siteRoles"
        role="responder"
        :role-label="t('members.role')"
        :role-help="t('members.roleHelp')"
        :success="t('members.added')"
        @added="load"
    />
  </div>
  <p class="block">
    {{ t("members.intro") }}
  </p>
  <p
      v-if="members?.length === 0"
      class="box"
  >
    {{ t("members.empty") }}
  </p>
  <div
      v-else-if="members"
      class="table-container"
  >
    <table class="table is-fullwidth">
      <thead>
        <tr>
          <th>{{ t("members.name") }}</th>
          <th>{{ session.provider?.login === "username" ? t("loginField.username") : t("loginField.email") }}</th>
          <th>{{ t("members.role") }}</th>
          <th><span class="sr-visually-hidden">{{ t("members.remove") }}</span></th>
        </tr>
      </thead>
      <tbody>
        <tr
            v-for="member in sorted"
            :key="member.id"
        >
          <td>
            {{ member.displayName }}
            <SRTag
                v-if="member.pending"
                class="ml-2"
            >
              {{ t("accounts.pending") }}
            </SRTag>
          </td>
          <td>{{ member.login }}</td>
          <td>
            <div class="select is-small">
              <select
                  :value="member.role"
                  :disabled="member.id === session.user?.id"
                  :aria-label="t('members.roleOf', { name: member.displayName })"
                  @change="change(member, $event)"
              >
                <option
                    v-for="r in siteRoles"
                    :key="r"
                    :value="r"
                >
                  {{ t(`roles.${r}`) }}
                </option>
              </select>
            </div>
          </td>
          <td>
            <button
                v-if="member.id !== session.user?.id"
                type="button"
                class="button is-small"
                :aria-label="t('members.removeOf', { name: member.displayName })"
                :title="t('members.remove')"
                @click="remove(member)"
            >
              <span class="icon"><Trash2 aria-hidden="true" /></span>
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
