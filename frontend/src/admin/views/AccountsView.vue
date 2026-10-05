<script setup lang="ts">
import { Pencil, Trash2 } from "@lucide/vue"
import { computed, onMounted, ref } from "vue"
import { useI18n } from "vue-i18n"

import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import SRTag from "../../shared/components/SRTag.vue"
import { useConfirm } from "../../shared/composables/useConfirm"
import { formatDateTime } from "../../shared/format"
import { api } from "../api"
import GrantDialog from "../components/GrantDialog.vue"
import { usePageTitle } from "../composables/usePageTitle"
import { instanceRoles, type Role, siteRoles } from "../roles"
import { resolveText } from "../rules"
import { useNotices } from "../stores/notices"
import { useOverview } from "../stores/overview"
import { useSession } from "../stores/session"
import type { Account } from "../types"

const { t, locale } = useI18n()
const notices = useNotices()
const overview = useOverview()
const session = useSession()
const { confirm } = useConfirm()
usePageTitle(() => t("accounts.title"))

const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone
const accounts = ref<Account[] | null>(null)
const sorted = computed(() => {
  const collator = new Intl.Collator(locale.value)
  return [...accounts.value ?? []]
    .sort((a, b) => collator.compare(a.displayName, b.displayName))
})
const sites = computed(() => (overview.sites ?? []).map(s => ({
  id:    s.id,
  label: resolveText(s.name, locale.value, s.languages),
})))

/** editing holds the roles of the account being edited. */
const editing = ref<{
  account: Account
  role:    Role
  sites:   Record<string, Role>
} | null>(null)
const saving = ref(false)

async function load() {
  try {
    accounts.value = await api<Account[]>("GET", "/api/admin/accounts")
  } catch(err) {
    notices.loadFailed(err)
  }
}

onMounted(load)

/** loginOf shows how an account is named: its email, else its subject. */
const loginOf = (a: Account) => a.email || a.subject || ""

function edit(account: Account) {
  editing.value = {
    account,
    role:  account.role ?? "",
    sites: {
      ...Object.fromEntries(sites.value.map(s => [s.id, ""])),
      ...account.sites,
    },
  }
}

async function save() {
  const e = editing.value!
  saving.value = true
  try {
    // A site without role is left out.
    const roles = Object.fromEntries(Object.entries(e.sites).filter(([, r]) => r))
    await api("PUT", `/api/admin/accounts/${e.account.id}`, {
      role:  e.role,
      sites: roles,
    })
    editing.value = null
    notices.success(t("accounts.saved"))
    await load()
  } catch(err) {
    notices.failure(t("toast.saveFailed"), err)
  } finally {
    saving.value = false
  }
}

async function remove(account: Account) {
  const ok = await confirm({
    title:   t("accounts.deleteTitle"),
    message: t("accounts.deleteMessage", { name: account.displayName }),
    confirm: t("accounts.delete"),
    danger:  true,
  })
  if (!ok) {
    return
  }

  try {
    await api("DELETE", `/api/admin/accounts/${account.id}`)
    await load()
  } catch(err) {
    notices.failure(t("toast.deleteFailed"), err)
  }
}
</script>

<template>
  <div class="level">
    <h1 class="title mb-0">
      {{ t("accounts.title") }}
    </h1>
    <GrantDialog
        :title="t('accounts.add')"
        path="/api/admin/accounts"
        :roles="['admin', 'owner']"
        role="admin"
        :role-label="t('accounts.role')"
        :role-help="t('accounts.roleHelp')"
        :success="t('accounts.added')"
        @added="load"
    />
  </div>
  <p class="block">
    {{ t("accounts.intro") }}
  </p>
  <div
      v-if="accounts"
      class="table-container"
  >
    <table class="table is-fullwidth">
      <thead>
        <tr>
          <th>{{ t("members.name") }}</th>
          <th>{{ t("accounts.login") }}</th>
          <th>{{ t("accounts.role") }}</th>
          <th>{{ t("accounts.sites") }}</th>
          <th>{{ t("accounts.lastSignIn") }}</th>
          <th><span class="sr-visually-hidden">{{ t("accounts.actions") }}</span></th>
        </tr>
      </thead>
      <tbody>
        <tr
            v-for="account in sorted"
            :key="account.id"
        >
          <td>
            {{ account.displayName }}
            <SRTag
                v-if="account.pending"
                class="ml-2"
            >
              {{ t("accounts.pending") }}
            </SRTag>
            <SRTag
                v-if="account.stale"
                color="warning"
                class="ml-2"
            >
              {{ t("accounts.stale") }}
            </SRTag>
          </td>
          <td>{{ loginOf(account) }}</td>
          <td>{{ t(`roles.${account.role || "none"}`) }}</td>
          <td>{{ Object.keys(account.sites ?? {}).length }}</td>
          <td>
            <time
                v-if="account.lastSignIn"
                :datetime="account.lastSignIn"
            >{{ formatDateTime(new Date(account.lastSignIn), locale, timeZone) }}</time>
          </td>
          <td>
            <span
                v-if="account.id !== session.user?.id"
                class="buttons is-flex-wrap-nowrap"
            >
              <button
                  type="button"
                  class="button is-small"
                  :aria-label="t('accounts.editOf', { name: account.displayName })"
                  :title="t('accounts.edit')"
                  @click="edit(account)"
              >
                <span class="icon"><Pencil aria-hidden="true" /></span>
              </button>
              <button
                  type="button"
                  class="button is-small"
                  :aria-label="t('accounts.deleteOf', { name: account.displayName })"
                  :title="t('accounts.delete')"
                  @click="remove(account)"
              >
                <span class="icon"><Trash2 aria-hidden="true" /></span>
              </button>
            </span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
  <form
      v-if="editing"
      class="box sr-form"
      @submit.prevent="save"
  >
    <h2 class="title is-5">
      {{ t("accounts.editOf", { name: editing.account.displayName }) }}
    </h2>
    <SRField
        v-slot="{ id }"
        :label="t('accounts.role')"
        :help="t('accounts.roleHelp')"
    >
      <div class="select">
        <select
            :id
            v-model="editing.role"
        >
          <option
              v-for="r in instanceRoles"
              :key="r"
              :value="r"
          >
            {{ t(`roles.${r || "none"}`) }}
          </option>
        </select>
      </div>
    </SRField>
    <SRField
        v-for="site in sites"
        :key="site.id"
        v-slot="{ id }"
        :label="site.label"
    >
      <div class="select">
        <select
            :id
            v-model="editing.sites[site.id]"
        >
          <option value="">
            {{ t("roles.none") }}
          </option>
          <option
              v-for="r in siteRoles"
              :key="r"
              :value="r"
          >
            {{ t(`roles.${r}`) }}
          </option>
        </select>
      </div>
    </SRField>
    <div class="field is-grouped">
      <SRButton
          type="submit"
          variant="primary"
          :loading="saving"
      >
        {{ t("common.save") }}
      </SRButton>
      <SRButton @click="editing = null">
        {{ t("common.cancel") }}
      </SRButton>
    </div>
  </form>
</template>
