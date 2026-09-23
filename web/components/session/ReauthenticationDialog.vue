<script setup lang="ts">
import { ApiError, endpoints } from '~/src/api'

const { t } = useI18n()
const { open, challengeId, cancel, finish } = useReauthentication()
const route = useRoute()
const password = ref('')
const busy = ref(false)
const error = ref('')
let controller: AbortController | null = null

function clear() {
  controller?.abort()
  controller = null
  password.value = ''
  error.value = ''
  busy.value = false
}

watch(open, clear, { flush: 'sync' })
watch(() => route.fullPath, cancel)
onBeforeUnmount(() => {
  cancel()
  clear()
})

async function submit() {
  if (busy.value || !open.value) return
  if (!password.value) {
    error.value = t('auth.reauthenticationPasswordRequired')
    return
  }
  const id = challengeId.value
  const attempt = new AbortController()
  controller = attempt
  busy.value = true
  error.value = ''
  const request = endpoints.reauthenticate(password.value, attempt.signal)
  password.value = ''
  try {
    await request
    if (!attempt.signal.aborted) finish(true, id)
  } catch (cause) {
    if (!attempt.signal.aborted && challengeId.value === id) {
      error.value = cause instanceof ApiError && cause.status === 401
        ? t('auth.reauthenticationInvalidPassword')
        : cause instanceof Error && cause.message ? cause.message : t('common.requestFailed')
    }
  } finally {
    if (controller === attempt) {
      controller = null
      busy.value = false
    }
  }
}
</script>

<template>
  <UiDialog
    :open="open"
    :title="t('auth.reauthenticationTitle')"
    :description="t('auth.reauthenticationDescription')"
    size="sm"
    @update:open="value => { if (!value) cancel() }"
  >
    <form id="reauthentication-form" class="space-y-4" @submit.prevent="submit">
      <UiField :label="t('auth.password')" for="reauthentication-password" required>
        <UiInput
          id="reauthentication-password"
          v-model="password"
          type="password"
          autocomplete="current-password"
          :disabled="busy"
          :invalid="Boolean(error)"
        />
      </UiField>
      <UiAlert v-if="error" tone="danger" role="alert">{{ error }}</UiAlert>
    </form>
    <template #footer>
      <UiButton variant="secondary" @click="cancel">{{ t('common.cancel') }}</UiButton>
      <UiButton type="submit" form="reauthentication-form" :loading="busy">{{ t('auth.reauthenticationConfirm') }}</UiButton>
    </template>
  </UiDialog>
</template>
