<script setup lang="ts">
import { Plus, Sparkles } from 'lucide-vue-next'
import { endpoints, type Group, type SubscriptionPlan, type SubscriptionPlanForm } from '~/src/api'
import { formatMoney, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('system.plansTitle')} · ${settings.value.name}` })

const { data: plansData, pending, error, refresh } = useResource(
  () => endpoints.getAdminSubscriptionPlans(),
  { data: [] as SubscriptionPlan[] },
)
const { data: groupsData } = useResource(() => endpoints.getAdminGroups('?page_size=100'), { data: [] as Group[], total: 0, page: 1, page_size: 100 })

const plans = computed(() => [...plansData.value.data].sort((a, b) => a.sort_order - b.sort_order))
const groups = computed(() => groupsData.value.data)

const dialogOpen = ref(false)
const editing = ref<SubscriptionPlan | null>(null)
const removing = ref<SubscriptionPlan | null>(null)

const PERIOD_KEYS: Record<string, string> = {
  hour: 'system.periodHour',
  day: 'system.periodDay',
  week: 'system.periodWeek',
  month: 'system.periodMonth',
  year: 'system.periodYear',
}

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(plan: SubscriptionPlan) {
  editing.value = plan
  dialogOpen.value = true
}

async function submitPlan(form: SubscriptionPlanForm) {
  const target = editing.value
  const ok = await run(() => (target
    ? endpoints.updateSubscriptionPlan(target.id, form)
    : endpoints.createSubscriptionPlan(form)))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(target ? t('system.planUpdated') : t('system.planCreated'))
  dialogOpen.value = false
  await refresh()
}

async function confirmDelete() {
  const target = removing.value
  if (!target) return
  const ok = await run(() => endpoints.deleteSubscriptionPlan(target.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('system.planDeleted'))
  removing.value = null
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('system.plansTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('system.plansLead') }}</p>
        </div>
        <UiButton size="sm" @click="openCreate">
          <Plus class="size-4" />
          {{ t('system.newPlan') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="plans.length === 0"
        :empty-icon="Sparkles"
        :empty-title="t('system.plansEmptyTitle')"
        :empty-description="t('system.plansEmptyBody')"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('common.name') }}</th>
              <th class="num">{{ t('system.price') }}</th>
              <th class="num">{{ t('system.creditAmount') }}</th>
              <th>{{ t('system.planGroup') }}</th>
              <th class="num">{{ t('system.modelWhitelist') }}</th>
              <th class="num">{{ t('system.quotaLimits') }}</th>
              <th>{{ t('system.overagePolicy') }}</th>
              <th class="num">{{ t('system.sortOrder') }}</th>
              <th>{{ t('common.status') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="plan in plans" :key="plan.id">
              <td>
                <p class="font-medium text-ink">{{ plan.name }}</p>
                <p v-if="plan.description" class="mt-0.5 text-[13px] text-muted">{{ plan.description }}</p>
              </td>
              <td class="num">
                {{ plan.price }} {{ plan.currency }}
                <span class="text-muted"> · {{ t(PERIOD_KEYS[plan.billing_period]) }}</span>
              </td>
              <td class="num">{{ plan.credit_amount || t('system.unlimited') }}</td>
              <td>{{ plan.group_name || '—' }}</td>
              <td class="num">
                {{ plan.model_whitelist.length
                  ? t('system.modelWhitelistCount', { count: formatNumber(plan.model_whitelist.length) })
                  : t('system.unlimited') }}
              </td>
              <td class="num">
                <div class="text-[13px]">
                  <p v-if="plan.max_requests_per_period !== null">
                    {{ t('system.quotaRequests', { count: formatNumber(plan.max_requests_per_period) }) }}
                  </p>
                  <p v-if="plan.max_credit_per_period !== null">
                    {{ t('system.quotaCredit', { amount: formatMoney(plan.max_credit_per_period) }) }}
                  </p>
                  <p v-if="plan.model_quotas.length" class="text-muted">
                    {{ t('system.modelQuotaCount', { count: formatNumber(plan.model_quotas.length) }) }}
                  </p>
                  <p v-if="plan.max_requests_per_period === null && plan.max_credit_per_period === null && plan.model_quotas.length === 0" class="text-muted">
                    {{ t('system.unlimited') }}
                  </p>
                </div>
              </td>
              <td>
                <UiBadge :tone="plan.overage_policy === 'block' ? 'warn' : 'neutral'">
                  {{ plan.overage_policy === 'block' ? t('system.overageBlock') : t('system.overageWallet') }}
                </UiBadge>
              </td>
              <td class="num">{{ plan.sort_order }}</td>
              <td>
                <UiBadge :tone="plan.enabled ? 'success' : 'neutral'" dot>
                  {{ plan.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </td>
              <td>
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEdit(plan)">{{ t('common.edit') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="removing = plan">{{ t('common.delete') }}</UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>
    </div>

    <ConsoleSystemPlanDialog
      v-model:open="dialogOpen"
      :plan="editing"
      :groups="groups"
      :busy="busy"
      @submit="submitPlan"
    />

    <ConsoleSystemConfirmDialog
      :open="removing !== null"
      :body="t('system.deletePlanBody', { name: removing?.name ?? '' })"
      :busy="busy"
      @update:open="value => { if (!value) removing = null }"
      @confirm="confirmDelete"
    />
  </ConsoleSystemGate>
</template>
