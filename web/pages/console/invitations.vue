<script setup lang="ts">
import { useClipboard } from '@vueuse/core'
import { Copy, UserPlus } from 'lucide-vue-next'
import { endpoints, type Invitation } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()
const { data, pending, error, refresh } = useResource(
  () => endpoints.getAccountInvitations(),
  { enabled: false, code: '', inviter_reward: '0', invitee_reward: '0', data: [] as Invitation[] },
)

useHead({ title: () => `${t('nav.invitations')} · ${settings.value.name}` })

const invitationLink = computed(() => {
  if (!import.meta.client || !data.value.code) return ''
  return `${window.location.origin}/auth?mode=register&invite=${encodeURIComponent(data.value.code)}`
})
const totalReward = computed(() => data.value.data.reduce((sum, item) => sum + Number(item.reward), 0))

const { copy } = useClipboard({ legacy: true })

function copyValue(value: string) {
  copy(value)
  toast.success(t('console.invitationCopied'))
}
</script>

<template>
  <div class="space-y-4">
    <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>
    <UiSkeleton v-else-if="pending" :rows="5" class="h-12" />
    <template v-else>
      <UiAlert v-if="!data.enabled" tone="warn" :title="t('console.invitationsDisabledTitle')">
        {{ t('console.invitationsDisabledBody') }}
      </UiAlert>
      <div class="grid gap-4 lg:grid-cols-3">
        <UiCard :title="t('console.invitationCode')" class="lg:col-span-2">
          <p class="text-[13px] text-muted">{{ t('console.invitationLead', { inviter: formatMoney(data.inviter_reward), invitee: formatMoney(data.invitee_reward) }) }}</p>
          <div class="mt-4 flex gap-2">
            <UiInput :model-value="invitationLink" readonly mono class="flex-1" />
            <UiButton size="icon" variant="secondary" :aria-label="t('console.copyInvitationLink')" @click="copyValue(invitationLink)">
              <Copy class="size-4" />
            </UiButton>
          </div>
          <div class="mt-3 flex items-center gap-2 text-[13px] text-muted">
            <span>{{ t('console.invitationCode') }}:</span>
            <code class="font-mono text-ink">{{ data.code }}</code>
            <button type="button" :aria-label="t('console.copyInvitationCode')" class="text-clay" @click="copyValue(data.code)">{{ t('common.copy') }}</button>
          </div>
        </UiCard>
        <ConsoleUserStatCard :label="t('console.invitationTotalReward')" :value="formatMoney(totalReward)" :hint="t('console.invitationCount', { count: data.data.length })" :icon="UserPlus" />
      </div>

      <UiCard :title="t('console.invitationHistory')" flush>
        <template #actions><UiButton variant="secondary" size="sm" @click="refresh">{{ t('common.refresh') }}</UiButton></template>
        <div class="px-5 py-4">
          <ConsoleUserDataState :pending="false" :error="null" :empty="!data.data.length" :rows="4" :empty-icon="UserPlus" :empty-title="t('console.invitationEmptyTitle')" :empty-description="t('console.invitationEmptyBody')">
            <UiTable>
              <thead><tr><th>{{ t('console.invitedUser') }}</th><th>{{ t('auth.email') }}</th><th class="num">{{ t('console.invitationReward') }}</th><th>{{ t('console.invitedAt') }}</th></tr></thead>
              <tbody><tr v-for="item in data.data" :key="item.id"><td>{{ item.name }}</td><td class="text-muted">{{ item.email }}</td><td class="num">{{ formatMoney(item.reward) }}</td><td class="text-muted">{{ formatDateTime(item.created_at) }}</td></tr></tbody>
            </UiTable>
          </ConsoleUserDataState>
        </div>
      </UiCard>
    </template>
  </div>
</template>
