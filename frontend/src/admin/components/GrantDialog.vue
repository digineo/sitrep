<script setup lang="ts">
import { Plus } from "@lucide/vue"
import { ref, useId, useTemplateRef } from "vue"
import { useI18n } from "vue-i18n"

import SRButton from "../../shared/components/SRButton.vue"
import SRField from "../../shared/components/SRField.vue"
import { api, errorCode, fieldErrors } from "../api"
import type { Role } from "../roles"
import { useNotices } from "../stores/notices"
import LoginField from "./LoginField.vue"

const props = defineProps<{
  /** title labels the button and the dialog, e.g. "Add member". */
  title:     string
  /** path receives the login and the role. */
  path:      string
  roles:     Role[]
  /** role is preselected. */
  role:      Role
  roleLabel: string
  roleHelp:  string
  /** success is the toast after adding. */
  success:   string
}>()
const emit = defineEmits<{ added: [] }>()

const { t } = useI18n()
const notices = useNotices()
const dialog = useTemplateRef<HTMLDialogElement>("dialog")
const trigger = useTemplateRef<InstanceType<typeof SRButton>>("trigger")
const titleId = useId()

const login = ref("")
const role = ref(props.role)
const errors = ref<Record<string, string>>({})
/** failure is the code of an error no field shows; toasts stay behind the dialog. */
const failure = ref("")
const adding = ref(false)

/** reset empties the form whenever the dialog closes. */
function reset() {
  login.value = ""
  role.value = props.role
  errors.value = {}
  failure.value = ""
  trigger.value?.$el.focus()
}

async function add() {
  adding.value = true
  errors.value = {}
  failure.value = ""
  try {
    await api("POST", props.path, {
      login: login.value,
      role:  role.value,
    })
    notices.success(props.success)
    dialog.value?.close()
    emit("added")
  } catch(err) {
    errors.value = fieldErrors(err)
    if (!Object.keys(errors.value).length) {
      failure.value = errorCode(err)
    }
  } finally {
    adding.value = false
  }
}
</script>

<template>
  <SRButton
      ref="trigger"
      variant="primary"
      @click="dialog?.showModal()"
  >
    <span class="icon"><Plus aria-hidden="true" /></span>
    <span>{{ title }}</span>
  </SRButton>
  <dialog
      ref="dialog"
      class="sr-dialog"
      :aria-labelledby="titleId"
      @close="reset"
  >
    <form
        class="box"
        @submit.prevent="add"
    >
      <h2
          :id="titleId"
          class="title is-5"
      >
        {{ title }}
      </h2>
      <p
          v-if="failure"
          class="notification is-danger"
          role="alert"
      >
        {{ t(`error.${failure}`) }}
      </p>
      <LoginField
          v-model="login"
          :error="errors.login && t(`error.${errors.login}`)"
      />
      <SRField
          v-slot="{ id, describedby, invalid }"
          :label="roleLabel"
          :help="roleHelp"
          :error="errors.role && t(`error.${errors.role}`)"
      >
        <div class="select">
          <select
              :id
              v-model="role"
              :aria-describedby="describedby"
              :aria-invalid="invalid"
          >
            <option
                v-for="r in roles"
                :key="r"
                :value="r"
            >
              {{ t(`roles.${r || "none"}`) }}
            </option>
          </select>
        </div>
      </SRField>
      <div class="buttons is-right">
        <SRButton @click="dialog?.close()">
          {{ t("common.cancel") }}
        </SRButton>
        <SRButton
            type="submit"
            variant="primary"
            :loading="adding"
        >
          {{ title }}
        </SRButton>
      </div>
    </form>
  </dialog>
</template>

<style scoped>
.sr-dialog {
  width: 100%;
}
</style>
