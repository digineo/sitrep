<script setup lang="ts">
import { computed, onMounted, ref, useId } from "vue"
import { useI18n } from "vue-i18n"

import SRField from "../../shared/components/SRField.vue"
import { api } from "../api"
import { useNotices } from "../stores/notices"
import { useSession } from "../stores/session"

const login = defineModel<string>({ required: true })
defineProps<{ error?: string }>()

const { t } = useI18n()
const session = useSession()
const notices = useNotices()
const listId = useId()
const byUsername = computed(() => session.provider?.login === "username")
const users = ref<string[]>([])

onMounted(async() => {
  if (!byUsername.value) {
    return
  }

  try {
    users.value = await api<string[]>("GET", "/api/admin/directory")
  } catch(err) {
    notices.loadFailed(err)
  }
})

const domain = (email: string) => email.slice(email.lastIndexOf("@") + 1).toLowerCase()

// Accounts are usually colleagues: another domain than the user's own
// may be a typo.
const help = computed(() => {
  if (byUsername.value) {
    return t("loginField.usernameHelp")
  }

  const own = session.user?.email
  const value = login.value.trim()
  if (own && value.includes("@") && domain(value) !== domain(own)) {
    return t("loginField.otherDomain", { domain: domain(own) })
  }
  return t("loginField.emailHelp")
})
</script>

<template>
  <SRField
      v-slot="{ id, describedby, invalid }"
      :label="byUsername ? t('loginField.username') : t('loginField.email')"
      :help
      :error
  >
    <input
        :id
        v-model="login"
        class="input"
        :type="byUsername ? 'text' : 'email'"
        :list="byUsername ? listId : undefined"
        required
        autocomplete="off"
        :aria-describedby="describedby"
        :aria-invalid="invalid"
    >
    <datalist
        v-if="byUsername"
        :id="listId"
    >
      <option
          v-for="user in users"
          :key="user"
          :value="user"
      />
    </datalist>
  </SRField>
</template>
