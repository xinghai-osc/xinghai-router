<script setup lang="ts">
import { Check, Sparkles } from 'lucide-vue-next'
import {
  endpoints,
  type PaymentMethod,
  type PaymentOrder,
  type PublicSubscriptionPlan,
  type SubscriptionOrder,
  type UserSubscription,
} from '~/src/api'
import { formatDateTime, formatMoney, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const SUB_STATUS_KEYS: Record<string, string> = {
  pending: 'console.subStatusPending',
  active: 'console.subStatusActive',
  expired: 'console.subStatusExpired',
  cancelled: 'console.subStatusCancelled',
}

const { t } = useI18n()
const { toast } = useToast()
const { settings } = useSiteSettings()
const { account, loadAccount } = useAccount()

useHead({ title: () => `${t('nav.subscriptions')} · ${settings.value.name}` })

const {
  data: subscriptions, pending: subsPending, error: subsError, refresh: refreshSubs,
} = useResource(() => endpoints.getAccountSubscriptions(), { data: [] as UserSubscription[] })

const {
  data: plans, pending: plansPending, error: plansError,
} = useResource(() => endpoints.getPublicSubscriptionPlans(), { data: [] as PublicSubscriptionPlan[] })

const {
  data: orders, pending: ordersPending, error: ordersError, refresh: refreshOrders,
} = useResource(() => endpoints.getAccountSubscriptionOrders(), { data: [] as SubscriptionOrder[] })

const { data: payments } = useResource(
  () => endpoints.getAccountPayments(),
  { enabled: false, payment_methods: [] as PaymentMethod[], data: [] as PaymentOrder[] },
)

const { busy, run } = useAction()

const subscribeOpen = ref(false)
const cancelOpen = ref(false)
const selectedPlan = ref<PublicSubscriptionPlan | null>(null)
const cancelling = ref<UserSubscription | null>(null)
const method = ref('balance')
const autoRenew = ref(true)

const availableBalance = computed(() => Math.max(0, (account.value?.balance ?? 0) - (account.value?.reserved ?? 0)))
const methodOptions = computed(() => [
  { value: 'balance', label: t('console.payWithBalance') },
  ...payments.value.payment_methods.map(entry => ({ value: entry.code, label: entry.name })),
])
const payingWithBalance = computed(() => method.value === 'balance')
const selectedPrice = computed(() => Number(selectedPlan.value?.price ?? 0))
const balanceSufficient = computed(() => availableBalance.value >= selectedPrice.value)
const canSubscribe = computed(() => methodOptions.value.length > 0)

const sortedPlans = computed(() => [...plans.value.data].sort((a, b) => a.sort_order - b.sort_order))

watch(methodOptions, (options) => {
  if (!options.some(option => option.value === method.value)) method.value = options[0]?.value ?? 'balance'
})

const PERIOD_KEYS: Record<string, string> = {
  hour: 'console.perHour',
  day: 'console.perDay',
  week: 'console.perWeek',
  month: 'console.perMonth',
  year: 'console.perYear',
}

function periodLabel(period: string): string {
  return PERIOD_KEYS[period] ? t(PERIOD_KEYS[period]) : period
}

function statusLabel(status: string): string {
  return SUB_STATUS_KEYS[status] ? t(SUB_STATUS_KEYS[status]) : status
}

function planBenefits(plan: PublicSubscriptionPlan): string[] {
  const list: string[] = []
  if (plan.credit_amount) {
    list.push(t('console.planCredit', { amount: Number(plan.credit_amount).toFixed(2) }))
  } else if (plan.max_credit_per_period === null
    && !(plan.model_quotas ?? []).some(quota => quota.max_credit_per_period !== null)) {
    list.push(t('console.planUnlimitedCredit'))
  }
  if (plan.max_requests_per_period !== null) {
    list.push(t('console.planQuotaRequests', { count: formatNumber(plan.max_requests_per_period) }))
  }
  if (plan.max_credit_per_period !== null) {
    list.push(t('console.planQuotaCredit', { amount: formatMoney(plan.max_credit_per_period) }))
  }
  for (const quota of plan.model_quotas ?? []) {
    list.push(t('console.planModelQuota', { model: quota.model }))
  }
  if (plan.group_name) list.push(t('console.planGroup', { group: plan.group_name }))
  list.push(plan.model_whitelist.length
    ? t('console.planWhitelist', { count: plan.model_whitelist.length })
    : t('console.planAllModels'))
  return list
}

interface QuotaBar {
  key: string
  label: string
  used: string
  max: string
  percent: number
}

function barPercent(used: number, max: number): number {
  if (max <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((used / max) * 100)))
}

function quotaBars(subscription: UserSubscription): QuotaBar[] {
  const bars: QuotaBar[] = []
  if (subscription.max_requests_per_period !== null) {
    bars.push({
      key: 'requests',
      label: t('console.quotaRequests'),
      used: formatNumber(subscription.usage_requests),
      max: formatNumber(subscription.max_requests_per_period),
      percent: barPercent(subscription.usage_requests, subscription.max_requests_per_period),
    })
  }
  if (subscription.max_credit_per_period !== null) {
    bars.push({
      key: 'credit',
      label: t('console.quotaCredit'),
      used: formatMoney(subscription.usage_credit),
      max: formatMoney(subscription.max_credit_per_period),
      percent: barPercent(subscription.usage_credit, subscription.max_credit_per_period),
    })
  }
  for (const quota of subscription.model_quotas ?? []) {
    const usage = subscription.model_usage.find(u => u.model === quota.model)
    if (quota.max_requests_per_period !== null) {
      bars.push({
        key: `model-${quota.model}-requests`,
        label: t('console.quotaModelRequests', { model: quota.model }),
        used: formatNumber(usage?.requests ?? 0),
        max: formatNumber(quota.max_requests_per_period),
        percent: barPercent(usage?.requests ?? 0, quota.max_requests_per_period),
      })
    }
    if (quota.max_credit_per_period !== null) {
      bars.push({
        key: `model-${quota.model}-credit`,
        label: t('console.quotaModelCredit', { model: quota.model }),
        used: formatMoney(usage?.credit ?? 0),
        max: formatMoney(quota.max_credit_per_period),
        percent: barPercent(usage?.credit ?? 0, quota.max_credit_per_period),
      })
    }
  }
  return bars
}

function openSubscribe(plan: PublicSubscriptionPlan) {
  selectedPlan.value = plan
  method.value = 'balance'
  autoRenew.value = false
  subscribeOpen.value = true
}

function openCancel(subscription: UserSubscription) {
  cancelling.value = subscription
  cancelOpen.value = true
}

async function confirmSubscribe() {
  const plan = selectedPlan.value
  if (!plan || !method.value) return

  if (payingWithBalance.value) {
    if (!balanceSufficient.value) return
    const ok = await run(() => endpoints.createBalanceSubscription(plan.id))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    subscribeOpen.value = false
    toast.success(t('console.balancePurchaseSuccess'))
    await Promise.all([refreshSubs(), refreshOrders(), loadAccount(true)])
    return
  }

  let payUrl = ''
  const ok = await run(async () => {
    payUrl = (await endpoints.createAccountSubscription(plan.id, method.value, autoRenew.value)).pay_url
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }

  subscribeOpen.value = false
  const opened = window.open(payUrl, '_blank', 'noopener')
  if (opened) toast.success(t('console.subscribeRedirect'))
  else toast.warn(t('console.popupBlocked'))

  await Promise.all([refreshSubs(), refreshOrders()])
}

async function confirmCancel() {
  const target = cancelling.value
  if (!target) return
  const ok = await run(() => endpoints.cancelAccountSubscription(target.id))
  cancelOpen.value = false
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('console.subscriptionCancelled'))
  await refreshSubs()
}
</script>

<template>
  <div class="space-y-4">
    <UiCard :title="t('console.currentSubscriptions')" :description="t('console.subscriptionsDescription')" flush>
      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="subsPending"
          :error="subsError"
          :empty="!subscriptions.data.length"
          :rows="4"
          :empty-icon="Sparkles"
          :empty-title="t('console.subsEmptyTitle')"
          :empty-description="t('console.subsEmptyBody')"
        >
          <div class="grid gap-3 md:grid-cols-2">
            <article
              v-for="subscription in subscriptions.data"
              :key="subscription.id"
              class="space-y-3 rounded-card border border-line bg-surface p-4"
            >
              <header class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-ink">{{ subscription.plan_name }}</p>
                  <p class="numeric mt-0.5 text-[13px] text-muted">
                    {{ formatMoney(subscription.price) }} / {{ periodLabel(subscription.billing_period) }}
                  </p>
                </div>
                <ConsoleUserStatusBadge :status="subscription.status" />
              </header>

              <dl class="space-y-1 text-[13px]">
                <div class="flex justify-between gap-3">
                  <dt class="text-muted">{{ t('console.periodEnd') }}</dt>
                  <dd class="numeric text-ink">{{ formatDateTime(subscription.current_period_end) }}</dd>
                </div>
                <div class="flex justify-between gap-3">
                  <dt class="text-muted">{{ t('console.autoRenew') }}</dt>
                  <dd class="text-ink">
                    {{ subscription.auto_renew ? t('console.autoRenewOn') : t('console.autoRenewOff') }}
                  </dd>
                </div>
                <div v-if="subscription.group_name" class="flex justify-between gap-3">
                  <dt class="text-muted">{{ t('console.group') }}</dt>
                  <dd class="text-ink">{{ subscription.group_name }}</dd>
                </div>
              </dl>

              <div
                v-if="quotaBars(subscription).length"
                class="space-y-2 border-t border-line pt-3"
              >
                <p class="text-[13px] font-medium text-ink">{{ t('console.periodQuota') }}</p>
                <div v-for="bar in quotaBars(subscription)" :key="bar.key" class="space-y-1">
                  <div class="flex items-center justify-between gap-3 text-[13px]">
                    <span class="min-w-0 truncate text-muted">{{ bar.label }}</span>
                    <span class="numeric shrink-0 text-ink">{{ bar.used }} / {{ bar.max }}</span>
                  </div>
                  <div
                    class="h-1.5 w-full overflow-hidden rounded-full bg-sunken"
                    role="progressbar"
                    :aria-label="bar.label"
                    :aria-valuenow="bar.percent"
                    aria-valuemin="0"
                    aria-valuemax="100"
                  >
                    <div
                      class="h-full rounded-full bg-clay transition-[width] duration-150 ease-out"
                      :style="{ width: `${bar.percent}%` }"
                    />
                  </div>
                </div>
                <p v-if="subscription.overage_policy === 'block'" class="text-[13px] text-faint">
                  {{ t('console.planQuotaBlocked') }}
                </p>
              </div>

              <UiButton
                v-if="subscription.status === 'active' || subscription.status === 'pending'"
                variant="secondary"
                size="sm"
                @click="openCancel(subscription)"
              >{{ t('console.cancelSubscription') }}</UiButton>
              <p v-else class="text-[13px] text-faint">{{ statusLabel(subscription.status) }}</p>
            </article>
          </div>
        </ConsoleUserDataState>
      </div>
    </UiCard>

    <UiCard :title="t('console.availablePlans')" flush>
      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="plansPending"
          :error="plansError"
          :empty="!sortedPlans.length"
          :rows="4"
          :empty-icon="Sparkles"
          :empty-title="t('console.plansEmptyTitle')"
          :empty-description="t('console.plansEmptyBody')"
        >
          <div class="grid gap-3 md:grid-cols-3">
            <article
              v-for="plan in sortedPlans"
              :key="plan.id"
              class="flex flex-col gap-4 rounded-card border border-line bg-surface p-5 transition-colors duration-150 hover:border-line-strong"
            >
              <header class="space-y-1">
                <h3 class="text-sm font-semibold text-ink">{{ plan.name }}</h3>
                <p v-if="plan.description" class="text-[13px] text-muted">{{ plan.description }}</p>
              </header>

              <p class="flex items-baseline gap-1">
                <span class="display text-3xl text-ink">{{ formatMoney(plan.price, 0) }}</span>
                <span class="text-[13px] text-muted">/ {{ periodLabel(plan.billing_period) }}</span>
              </p>

              <ul class="flex-1 space-y-1.5">
                <li
                  v-for="benefit in planBenefits(plan)"
                  :key="benefit"
                  class="flex items-start gap-2 text-[13px] text-muted"
                >
                  <Check class="mt-0.5 size-3.5 shrink-0 text-clay" />
                  {{ benefit }}
                </li>
              </ul>

              <UiButton block :disabled="!canSubscribe" @click="openSubscribe(plan)">
                {{ t('console.subscribe') }}
              </UiButton>
            </article>
          </div>
        </ConsoleUserDataState>
      </div>
    </UiCard>

    <UiCard :title="t('console.subscriptionOrders')" flush>
      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="ordersPending"
          :error="ordersError"
          :empty="!orders.data.length"
          :rows="5"
          :empty-icon="Sparkles"
          :empty-title="t('console.subOrdersEmptyTitle')"
          :empty-description="t('console.subOrdersEmptyBody')"
        >
          <UiTable>
            <thead>
              <tr>
                <th>{{ t('console.orderNo') }}</th>
                <th>{{ t('console.plan') }}</th>
                <th class="num">{{ t('console.amount') }}</th>
                <th>{{ t('console.orderMethod') }}</th>
                <th>{{ t('console.orderKind') }}</th>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('console.paidAt') }}</th>
                <th>{{ t('common.createdAt') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in orders.data" :key="order.id">
                <td><code class="font-mono text-[13px] text-muted">{{ order.order_no }}</code></td>
                <td class="font-medium">{{ order.plan_name }}</td>
                <td class="num">{{ formatMoney(order.amount) }}</td>
                <td class="text-muted">{{ order.payment_type }}</td>
                <td class="text-muted">
                  {{ order.period_kind === 'renewal' ? t('console.orderKindRenewal') : t('console.orderKindNew') }}
                </td>
                <td><ConsoleUserStatusBadge :status="order.status" /></td>
                <td class="text-muted">{{ formatDateTime(order.paid_at) }}</td>
                <td class="text-muted">{{ formatDateTime(order.created_at) }}</td>
              </tr>
            </tbody>
          </UiTable>
        </ConsoleUserDataState>
      </div>
    </UiCard>

    <UiSlidePanel
      v-model:open="subscribeOpen"
      size="sm"
      :title="selectedPlan ? t('console.subscribeTitle', { name: selectedPlan.name }) : t('console.subscribe')"
    >
      <div class="space-y-4">
        <UiField :label="t('console.paymentMethod')" required>
          <UiSelect
            v-model="method"
            :options="methodOptions"
            :placeholder="t('common.selectPlaceholder')"
          />
        </UiField>

        <div v-if="payingWithBalance" class="space-y-3 rounded-card border border-line bg-sunken p-4 text-[13px]">
          <div class="flex justify-between gap-3">
            <span class="text-muted">{{ t('console.availableBalance') }}</span>
            <span class="numeric text-ink">{{ formatMoney(availableBalance) }}</span>
          </div>
          <div class="flex justify-between gap-3">
            <span class="text-muted">{{ t('console.purchaseAmount') }}</span>
            <span class="numeric text-ink">{{ formatMoney(selectedPrice) }}</span>
          </div>
          <div v-if="balanceSufficient" class="flex justify-between gap-3 border-t border-line pt-3">
            <span class="text-muted">{{ t('console.balanceAfterPurchase') }}</span>
            <span class="numeric text-ink">{{ formatMoney(availableBalance - selectedPrice) }}</span>
          </div>
          <UiAlert v-else tone="warn">
            {{ t('console.insufficientBalance', { amount: formatMoney(selectedPrice - availableBalance) }) }}
            <UiButton to="/console/wallet" variant="link" size="sm" class="ml-1 h-auto p-0">
              {{ t('console.topUpToContinue') }}
            </UiButton>
          </UiAlert>
        </div>

        <UiField v-else :label="t('console.autoRenew')">
          <UiSwitch v-model="autoRenew" :label="t('console.autoRenew')" />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="subscribeOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" :disabled="!method || (payingWithBalance && !balanceSufficient)" @click="confirmSubscribe">
          {{ payingWithBalance ? t('console.payBalanceAndActivate') : t('console.subscribe') }}
        </UiButton>
      </template>
    </UiSlidePanel>

    <UiDialog
      v-model:open="cancelOpen"
      size="sm"
      :title="t('console.cancelSubscriptionTitle')"
      :description="cancelling?.plan_name"
    >
      <p class="text-[13px] text-muted">{{ t('console.cancelSubscriptionBody') }}</p>

      <template #footer>
        <UiButton variant="secondary" @click="cancelOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="confirmCancel">
          {{ t('console.cancelSubscription') }}
        </UiButton>
      </template>
    </UiDialog>
  </div>
</template>
