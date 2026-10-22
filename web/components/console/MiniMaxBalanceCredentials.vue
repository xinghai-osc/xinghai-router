<script setup lang="ts">
import { endpoints, type MiniMaxBalanceCredentials } from '~/src/api'

const props = defineProps<{ channelId: string }>()
const { t } = useI18n()
const { toast } = useToast()
const { busy, error: actionError, run } = useAction()
const { data, pending, error, refresh } = useResource(
  () => endpoints.getMiniMaxBalanceCredentials(props.channelId),
  { configured: false, group_id: '' } as MiniMaxBalanceCredentials,
)
const cookie = ref('')
const groupId = ref('')
const validationError = ref('')

watch(data, value => { groupId.value = value.group_id })
onBeforeUnmount(() => { cookie.value = '' })

async function saveCredentials() {
  validationError.value = ''
  const group = groupId.value.trim()
  const secret = cookie.value.trim()
  if (!/^\d{1,64}$/.test(group) || (!data.value.configured && !secret)) {
    validationError.value = t('admin.minimaxBalanceRequired')
    return
  }
  const ok = await run(() => endpoints.saveMiniMaxBalanceCredentials(props.channelId, { group_id: group, cookie: secret }))
  cookie.value = ''
  if (!ok) return
  toast.success(t('admin.minimaxBalanceSaved'))
  await refresh()
}

async function clearCredentials() {
  validationError.value = ''
  const ok = await run(() => endpoints.deleteMiniMaxBalanceCredentials(props.channelId))
  if (!ok) return
  cookie.value = ''
  toast.success(t('admin.minimaxBalanceCleared'))
  await refresh()
}
</script>

<template>
  <section class="space-y-3 rounded-control border border-line bg-sunken p-4" :aria-label="t('admin.minimaxBalanceTitle')">
    <h3 class="text-sm font-medium text-ink">{{ t('admin.minimaxBalanceTitle') }}</h3>
    <p class="text-xs leading-5 text-muted">{{ t('admin.minimaxBalanceHint') }}</p>
    <UiSkeleton v-if="pending" :rows="3" />
    <UiAlert v-else-if="error" tone="danger">
      <p>{{ error }}</p>
      <UiButton class="mt-2" variant="secondary" size="sm" @click="refresh()">{{ t('common.retry') }}</UiButton>
    </UiAlert>
    <template v-else>
      <UiBadge :tone="data.configured ? 'success' : 'neutral'">{{ t(data.configured ? 'admin.minimaxBalanceConfigured' : 'admin.minimaxBalanceUnconfigured') }}</UiBadge>
      <UiAlert v-if="validationError || actionError" tone="danger">{{ validationError || actionError }}</UiAlert>
      <UiField :label="t('admin.minimaxBalanceGroupId')" required>
        <UiInput v-model="groupId" mono :disabled="busy" inputmode="numeric" maxlength="64" autocomplete="off" />
      </UiField>
      <UiField :label="t('admin.minimaxBalanceCookie')" :hint="t('admin.minimaxBalanceCookieHint')" :required="!data.configured">
        <UiInput v-model="cookie" type="password" :disabled="busy" autocomplete="new-password" />
      </UiField>
      <div class="flex flex-wrap gap-2">
        <UiButton variant="secondary" size="sm" :loading="busy" @click="saveCredentials">{{ t('admin.minimaxBalanceSave') }}</UiButton>
        <UiButton v-if="data.configured" variant="danger" size="sm" :disabled="busy" @click="clearCredentials">{{ t('admin.minimaxBalanceClear') }}</UiButton>
      </div>
    </template>
  </section>
</template>
