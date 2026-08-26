<script setup lang="ts">
import { Repeat } from 'lucide-vue-next'
import { endpoints, type AdminSubscription, type SubscriptionPlan } from '~/src/api'
import { formatDateTime, formatMoney, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.subscriptionsTitle')} · ${settings.value.name}` })

const { data: subsData, pending, error, refresh } = useResource(
  () => endpoints.getAdminSubscriptions(),
  { data: [] as AdminSubscription[] },
)
const { data: plansData } = useResource(
  () => endpoints.getAdminSubscriptionPlans(),
  { data: [] as SubscriptionPlan[] },
)

const STATUS_KEYS = {
  pending: 'system.statusPending',
  active: 'system.statusActive',
  expired: 'system.statusExpired',
  cancelled: 'system.statusCancelled',
} as const

const STATUS_TONES = {
  pending: 'warn',
  active: 'success',
  expired: 'neutral',
  cancelled: 'danger',
} as const

const search = ref('')
const statusFilter = ref('all')

const statusOptions = computed(() => [
  { value: 'all', label: t('common.all') },
  ...Object.entries(STATUS_KEYS).map(([value, key]) => ({ value, label: t(key) })),
])

const subscriptions = computed(() => {
  const term = search.value.trim().toLowerCase()
  return subsData.value.data.filter((item) => {
    if (statusFilter.value !== 'all' && item.status !== statusFilter.value) return false
    if (!term) return true
    return [item.email, item.user_name, item.plan_name]
      .some(field => (field ?? '').toLowerCase().includes(term))
  })
})

const canManage = computed(() => can('system.manage'))

function remainingQuota(item: AdminSubscription): string[] {
  const lines: string[] = []
  if (item.max_requests_per_period !== null) {
    lines.push(t('system.remainingRequests', { remaining: formatNumber(item.remaining_requests), max: formatNumber(item.max_requests_per_period) }))
  }
  if (item.max_credit_per_period !== null) {
    lines.push(t('system.remainingCredit', { remaining: formatMoney(item.remaining_credit), max: formatMoney(item.max_credit_per_period) }))
  }
  if (!lines.length && item.model_quota_count > 0) {
    lines.push(t('system.modelQuotaCount', { count: formatNumber(item.model_quota_count) }))
  }
  if (!lines.length) lines.push(t('system.unlimited'))
  return lines
}

const extendPlanId = ref('all')
const extendDays = ref('30')
const extendStatus = ref<'active' | 'inactive' | 'all'>('active')
const resetQuotasOpen = ref(false)
const resetQuotaStatus = ref<'active' | 'inactive' | 'all'>('active')
const resetQuotaTarget = ref<AdminSubscription | null>(null)

const extendPlanOptions = computed(() => [
  { value: 'all', label: t('system.batchExtendAllPlans') },
  ...plansData.value.data.map(plan => ({ value: plan.id, label: plan.name })),
])

const extendStatusOptions = computed(() => [
  { value: 'active', label: t('system.batchExtendStatusActive') },
  { value: 'inactive', label: t('system.batchExtendStatusInactive') },
  { value: 'all', label: t('system.batchExtendStatusAll') },
])

const resetQuotaStatusOptions = computed(() => [
  { value: 'active', label: t('system.resetQuotaStatusActive') },
  { value: 'inactive', label: t('system.resetQuotaStatusInactive') },
  { value: 'all', label: t('system.resetQuotaStatusAll') },
])

async function runExtend() {
  const days = Number(extendDays.value.trim())
  if (!Number.isInteger(days) || days === 0 || Math.abs(days) > 3650) {
    toast.error(t('system.extendDaysInvalid'))
    return
  }
  let affected = 0
  const ok = await run(async () => {
    const result = await endpoints.batchExtendSubscriptions(
      extendPlanId.value === 'all' ? '' : extendPlanId.value,
      days,
      extendStatus.value,
    )
    affected = result.affected
  })
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  if (affected > 0) toast.success(t('system.extendDone', { count: affected }))
  else toast.info(t('system.extendNoop'))
  await refresh()
}

async function resetQuotas() {
  resetQuotasOpen.value = false
  let affected = 0
  const ok = await run(async () => {
    const result = await endpoints.resetActiveSubscriptionQuotas(resetQuotaStatus.value)
    affected = result.affected
  })
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.resetQuotasDone', { count: affected }))
  await refresh()
}

async function resetSubscriptionQuota() {
  const target = resetQuotaTarget.value
  if (!target) return
  resetQuotaTarget.value = null
  const ok = await run(() => endpoints.resetAdminSubscriptionQuota(target.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.resetSubscriptionQuotaDone', { plan: target.plan_name }))
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="users.read">
    <div class="space-y-4">
      <div class="min-w-0 space-y-1">
        <h2 class="text-lg font-semibold text-ink">{{ t('system.subscriptionsTitle') }}</h2>
        <p class="text-[13px] text-muted">{{ t('system.subscriptionsLead') }}</p>
      </div>

      <UiCard v-if="canManage" :title="t('system.batchExtend')" :description="t('system.batchExtendLead')">
        <div class="grid items-end gap-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_auto]">
          <UiField :label="t('system.batchExtendPlan')" for="extend-plan">
            <UiSelect
              id="extend-plan"
              v-model="extendPlanId"
              :options="extendPlanOptions"
              :placeholder="t('common.selectPlaceholder')"
            />
          </UiField>

          <UiField :label="t('system.batchExtendStatus')" for="extend-status">
            <UiSelect
              id="extend-status"
              v-model="extendStatus"
              :options="extendStatusOptions"
            />
          </UiField>

          <UiField :label="t('system.extendDays')" :hint="t('system.extendDaysHint')" for="extend-days">
            <UiInput id="extend-days" v-model="extendDays" />
          </UiField>

          <UiButton :loading="busy" @click="runExtend">{{ t('system.runExtend') }}</UiButton>
        </div>
      </UiCard>

      <UiCard v-if="canManage" :title="t('system.resetActiveQuotas')" :description="t('system.resetActiveQuotasLead')">
        <div class="flex flex-wrap items-end gap-3">
          <UiField :label="t('system.resetQuotaStatus')" for="reset-quota-status">
            <UiSelect id="reset-quota-status" v-model="resetQuotaStatus" :options="resetQuotaStatusOptions" />
          </UiField>
          <UiButton variant="secondary" :loading="busy" @click="resetQuotasOpen = true">
            {{ t('system.resetActiveQuotas') }}
          </UiButton>
        </div>
      </UiCard>

      <div class="flex flex-wrap items-center gap-3">
        <div class="min-w-56 flex-1">
          <UiInput v-model="search" :placeholder="t('system.searchSubscriptions')" />
        </div>
        <div class="w-44">
          <UiSelect
            v-model="statusFilter"
            :options="statusOptions"
            :placeholder="t('common.status')"
          />
        </div>
        <UiButton variant="secondary" size="sm" :loading="pending" @click="refresh">
          {{ t('common.refresh') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="subscriptions.length === 0"
        :empty-icon="Repeat"
        :empty-title="t('system.subscriptionsEmptyTitle')"
        :empty-description="t('system.subscriptionsEmptyBody')"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('system.subscriber') }}</th>
              <th>{{ t('system.plan') }}</th>
              <th>{{ t('system.remainingQuota') }}</th>
              <th>{{ t('common.status') }}</th>
              <th class="num">{{ t('system.periodStart') }}</th>
              <th class="num">{{ t('system.periodEnd') }}</th>
              <th>{{ t('system.autoRenew') }}</th>
              <th class="num">{{ t('system.cancelledAt') }}</th>
              <th class="num">{{ t('common.createdAt') }}</th>
              <th v-if="canManage">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in subscriptions" :key="item.id">
              <td>
                <p class="font-medium text-ink">{{ item.email }}</p>
                <p v-if="item.user_name" class="mt-0.5 text-[13px] text-muted">{{ item.user_name }}</p>
              </td>
              <td>{{ item.plan_name }}</td>
              <td>
                <p v-for="line in remainingQuota(item)" :key="line" class="whitespace-nowrap text-[13px] text-muted">
                  {{ line }}
                </p>
              </td>
              <td>
                <UiBadge :tone="STATUS_TONES[item.status]" dot>{{ t(STATUS_KEYS[item.status]) }}</UiBadge>
              </td>
              <td class="num">{{ formatDateTime(item.current_period_start) }}</td>
              <td class="num">{{ formatDateTime(item.current_period_end) }}</td>
              <td>{{ item.auto_renew ? t('system.yes') : t('system.no') }}</td>
              <td class="num">{{ formatDateTime(item.cancelled_at) }}</td>
              <td class="num">{{ formatDateTime(item.created_at) }}</td>
              <td v-if="canManage">
                <UiButton variant="ghost" size="sm" @click="resetQuotaTarget = item">
                  {{ t('system.resetSubscriptionQuota') }}
                </UiButton>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>

      <UiDialog v-model:open="resetQuotasOpen" size="sm" :title="t('system.resetActiveQuotas')">
        <p class="text-sm text-muted">{{ t('system.confirmResetQuotaByStatus', { status: t(`system.resetQuotaStatus${resetQuotaStatus.charAt(0).toUpperCase() + resetQuotaStatus.slice(1)}`) }) }}</p>
        <template #footer>
          <UiButton variant="secondary" @click="resetQuotasOpen = false">{{ t('common.cancel') }}</UiButton>
          <UiButton :loading="busy" @click="resetQuotas">{{ t('common.confirm') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog :open="resetQuotaTarget !== null" size="sm" :title="t('system.resetSubscriptionQuota')">
        <p class="text-sm text-muted">
          {{ t('system.confirmResetSubscriptionQuota', { plan: resetQuotaTarget?.plan_name ?? '' }) }}
        </p>
        <template #footer>
          <UiButton variant="secondary" @click="resetQuotaTarget = null">{{ t('common.cancel') }}</UiButton>
          <UiButton :loading="busy" @click="resetSubscriptionQuota">{{ t('common.confirm') }}</UiButton>
        </template>
      </UiDialog>
    </div>
  </ConsoleSystemGate>
</template>
