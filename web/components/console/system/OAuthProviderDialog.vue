<script setup lang="ts">
import type { OAuthProvider, OAuthProviderForm } from '~/src/api'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  provider: OAuthProvider | null
  busy?: boolean
}>()

const emit = defineEmits<{ submit: [payload: { provider: string; form: OAuthProviderForm }] }>()

const { t } = useI18n()

const providerId = ref('')
const clientId = ref('')
const clientSecret = ref('')
const enabled = ref(true)
const invalid = ref('')

watch(() => [open.value, props.provider] as const, ([isOpen]) => {
  if (!isOpen) return
  invalid.value = ''
  providerId.value = props.provider?.id ?? ''
  clientId.value = props.provider?.client_id ?? ''
  clientSecret.value = ''
  enabled.value = props.provider?.enabled ?? true
}, { immediate: true })

function submit() {
  if (!providerId.value.trim()) {
    invalid.value = t('system.oauthProviderIdRequired')
    return
  }
  if (!clientId.value.trim()) {
    invalid.value = t('system.oauthClientIdRequired')
    return
  }
  invalid.value = ''
  emit('submit', {
    provider: providerId.value.trim(),
    form: {
      client_id: clientId.value.trim(),
      client_secret: clientSecret.value,
      enabled: enabled.value,
    },
  })
}
</script>

<template>
  <UiDialog
    v-model:open="open"
    size="sm"
    :title="provider ? t('system.oauthEditProvider', { provider: provider.id }) : t('system.oauthAddProvider')"
  >
    <form class="space-y-4" @submit.prevent="submit">
      <UiAlert v-if="invalid" tone="danger">{{ invalid }}</UiAlert>

      <UiField
        :label="t('system.oauthProviderId')"
        :hint="t('system.oauthProviderIdHint')"
        required
        for="oauth-provider-id"
      >
        <UiInput
          id="oauth-provider-id"
          v-model="providerId"
          :disabled="!!provider"
          mono
          :placeholder="t('system.oauthProviderIdPlaceholder')"
        />
      </UiField>

      <UiField :label="t('system.oauthClientId')" required for="oauth-client-id">
        <UiInput id="oauth-client-id" v-model="clientId" mono />
      </UiField>

      <UiField :label="t('system.oauthClientSecret')" :hint="t('system.oauthClientSecretHint')" for="oauth-client-secret">
        <UiInput id="oauth-client-secret" v-model="clientSecret" type="password" mono autocomplete="new-password" />
      </UiField>

      <UiCheckbox v-model="enabled">{{ t('system.oauthEnabled') }}</UiCheckbox>
    </form>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.cancel') }}</UiButton>
      <UiButton :loading="busy" @click="submit">{{ t('common.save') }}</UiButton>
    </template>
  </UiDialog>
</template>
