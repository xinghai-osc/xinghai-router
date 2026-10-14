<script setup lang="ts">
import { Check, RotateCcw, Sparkles } from 'lucide-vue-next'
import {
  endpoints,
  type BillingPeriod,
  type PaymentMethod,
  type PaymentOrder,
  type SubscriptionOrder,
  type SubscriptionProduct,
  type SubscriptionTier,
  type SubscriptionTierPrice,
  type UserSubscription,
} from '~/src/api'
import { formatDateTime, formatMoney, formatNumber } from '~/src/format'
import {
  effectiveAmounts,
  periodLabel,
  priceFor,
  sortProducts,
  sortTiers,
  tierDisplayName,
  tierOptions,
  tierPrices,
} from '~/src/subscriptions'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const SUB_STATUS_KEYS: Record<string, string> = {
  pending: 'console.subStatusPending',
  active: 'console.subStatusActive',
  expired: 'console.subStatusExpired',
  cancelled: 'console.subStatusCancelled',
}

const { t } = useI18n()
const { toast } = useToast()
const route = useRoute()
const { settings } = useSiteSettings()
const { account, loadAccount } = useAccount()

useHead({ title: () => `${t('nav.subscriptions')} · ${settings.value.name}` })

const {
  data: subscriptions, pending: subsPending, error: subsError, refresh: refreshSubs,
} = useResource(() => endpoints.getAccountSubscriptions(), { data: [] as UserSubscription[] })

const {
  data: products, pending: productsPending, error: productsError,
} = useResource(() => endpoints.getPublicSubscriptionPlans(), { data: [] as SubscriptionProduct[] })

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
const resetOpen = ref(false)
const selectedTierId = ref('')
const selectedPeriod = ref<BillingPeriod | ''>('')
const cancelling = ref<UserSubscription | null>(null)
const resetting = ref<UserSubscription | null>(null)
const method = ref('balance')
const autoRenew = ref(true)

function money(value: number | string | null | undefined, currency?: string): string {
  const formatted = formatMoney(value)
  const code = (currency ?? '').trim().toUpperCase()
  return code && code !== 'CNY' && code !== 'RMB' ? `${formatted} ${code}` : formatted
}

const sortedProducts = computed(() => sortProducts(products.value.data))

const subscribeTierOptions = computed(() => tierOptions(sortedProducts.value))

function findTier(id: string): SubscriptionTier | null {
  for (const product of sortedProducts.value) {
    const tier = (product.tiers ?? []).find(item => item.id === id)
    if (tier) return tier
  }
  return null
}

const selectedTier = computed(() => findTier(selectedTierId.value))
const selectedTierLabel = computed(
  () => subscribeTierOptions.value.find(option => option.value === selectedTierId.value)?.label ?? '',
)
const selectedPrice = computed<SubscriptionTierPrice | null>(() => {
  const tier = selectedTier.value
  const period = selectedPeriod.value
  if (!tier || !period) return null
  return priceFor(tier, period) ?? null
})

const periodOptions = computed(() => {
  const tier = selectedTier.value
  if (!tier) return []
  return tierPrices(tier).map(price => ({
    value: price.billing_period,
    label: `${money(price.price, price.currency)} / ${periodLabel(t, price.billing_period, 'console')}`,
  }))
})

interface AvailableTier {
  product: SubscriptionProduct
  tier: SubscriptionTier
  prices: SubscriptionTierPrice[]
}

const availableTiers = computed<AvailableTier[]>(() => {
  const list: AvailableTier[] = []
  for (const product of sortedProducts.value) {
    for (const tier of sortTiers(product.tiers ?? [])) {
      if (!tier.enabled) continue
      const prices = tierPrices(tier)
      if (prices.length) list.push({ product, tier, prices })
    }
  }
  return list
})

const availableBalance = computed(() => Math.max(0, (account.value?.balance ?? 0) - (account.value?.reserved ?? 0)))
const methodOptions = computed(() => [
  { value: 'balance', label: t('console.payWithBalance') },
  ...payments.value.payment_methods.map(entry => ({ value: entry.code, label: entry.name })),
])
const payingWithBalance = computed(() => method.value === 'balance')
const purchaseAmount = computed(() => Number(selectedPrice.value?.price ?? 0))
const balanceSufficient = computed(() => availableBalance.value >= purchaseAmount.value)
const canSubscribe = computed(() => methodOptions.value.length > 0)
const canConfirm = computed(() => Boolean(selectedPrice.value) && Boolean(method.value))

watch(methodOptions, (options) => {
  if (!options.some(option => option.value === method.value)) method.value = options[0]?.value ?? 'balance'
})

watch(selectedTierId, () => {
  const tier = selectedTier.value
  const prices = tier ? tierPrices(tier) : []
  if (!prices.some(price => price.billing_period === selectedPeriod.value)) {
    selectedPeriod.value = prices[0]?.billing_period ?? ''
  }
})

function queryParam(value: unknown): string {
  if (Array.isArray(value)) return typeof value[0] === 'string' ? value[0] : ''
  return typeof value === 'string' ? value : ''
}

const queryApplied = ref(false)

function applyQuerySelection() {
  if (queryApplied.value) return
  const productList = sortedProducts.value
  if (!productList.length) return
  queryApplied.value = true
  const option = subscribeTierOptions.value.find(item => item.value === queryParam(route.query.tier))
  if (!option) return
  const tier = findTier(option.value)
  if (!tier) return
  const prices = tierPrices(tier)
  if (!prices.length) return
  const match = prices.find(price => price.billing_period === queryParam(route.query.period))
  openSubscribe(tier.id, match?.billing_period)
}

watch(sortedProducts, applyQuerySelection, { immediate: true })

function statusLabel(status: string): string {
  return SUB_STATUS_KEYS[status] ? t(SUB_STATUS_KEYS[status]) : status
}

function tierBenefits(tier: SubscriptionTier, price?: SubscriptionTierPrice): string[] {
  const list: string[] = []
  const amounts = effectiveAmounts(tier, price)
  if (amounts.creditAmount) {
    list.push(t('console.planCredit', { amount: Number(amounts.creditAmount).toFixed(2) }))
  } else if (amounts.maxCredit === null
    && !(tier.model_quotas ?? []).some(quota => quota.max_credit_per_period !== null)) {
    list.push(t('console.planUnlimitedCredit'))
  }
  if (amounts.maxRequests !== null) {
    list.push(t('console.planQuotaRequests', { count: formatNumber(amounts.maxRequests) }))
  }
  if (amounts.maxCredit !== null) {
    list.push(t('console.planQuotaCredit', { amount: formatMoney(amounts.maxCredit) }))
  }
  for (const quota of tier.model_quotas ?? []) {
    list.push(t('console.planModelQuota', { model: quota.model }))
  }
  if (tier.group_name) list.push(t('console.planGroup', { group: tier.group_name }))
  list.push((tier.model_whitelist ?? []).length
    ? t('console.planWhitelist', { count: (tier.model_whitelist ?? []).length })
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

function openSubscribe(tierId?: string, period?: BillingPeriod) {
  if (tierId) selectedTierId.value = tierId
  if (!selectedTierId.value) selectedTierId.value = subscribeTierOptions.value[0]?.value ?? ''
  const prices = selectedTier.value ? tierPrices(selectedTier.value) : []
  selectedPeriod.value = period && prices.some(item => item.billing_period === period)
    ? period
    : prices[0]?.billing_period ?? ''
  method.value = 'balance'
  autoRenew.value = false
  subscribeOpen.value = true
}

function openCancel(subscription: UserSubscription) {
  cancelling.value = subscription
  cancelOpen.value = true
}

async function confirmSubscribe() {
  const tier = selectedTier.value
  const price = selectedPrice.value
  if (!tier || !price || !method.value) return
  const period = price.billing_period

  if (payingWithBalance.value) {
    if (!balanceSufficient.value) return
    const ok = await run(() => endpoints.createBalanceSubscription(tier.id, period))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    subscribeOpen.value = false
    toast.success(t('console.balancePurchaseSuccess'))
    await Promise.all([refreshSubs(), refreshOrders(), loadAccount(true)])
    return
  }

  let payUrl = ''
  const ok = await run(async () => {
    payUrl = (await endpoints.createAccountSubscription(tier.id, period, method.value, autoRenew.value)).pay_url
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

function openReset(subscription: UserSubscription) {
  resetting.value = subscription
  resetOpen.value = true
}

async function confirmReset() {
  const target = resetting.value
  if (!target) return
  const ok = await run(() => endpoints.useResetCard(target.id))
  resetOpen.value = false
  resetting.value = null
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('console.resetCardSuccess'))
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
                  <p class="truncate text-sm font-medium text-ink">{{ subscription.display_name }}</p>
                  <p class="numeric mt-0.5 text-[13px] text-muted">
                    {{ money(subscription.price, subscription.currency) }} / {{ periodLabel(t, subscription.billing_period, 'console') }}
                  </p>
                  <p class="mt-0.5 text-[13px] text-faint">
                    {{ t('console.tierLevel', { level: subscription.tier_level }) }}
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

              <div class="flex items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <UiButton
                    v-if="subscription.status === 'active' || subscription.status === 'pending'"
                    variant="secondary"
                    size="sm"
                    @click="openCancel(subscription)"
                  >{{ t('console.cancelSubscription') }}</UiButton>
                  <p v-else class="text-[13px] text-faint">{{ statusLabel(subscription.status) }}</p>
                </div>
                <div v-if="subscription.reset_card_count > 0" class="flex items-center gap-2">
                  <UiBadge tone="success">
                    {{ t('console.resetCardCount', { count: subscription.reset_card_count }) }}
                  </UiBadge>
                  <UiButton
                    v-if="subscription.status === 'active'"
                    variant="secondary"
                    size="sm"
                    @click="openReset(subscription)"
                  >
                    <RotateCcw class="size-3.5" />
                    {{ t('console.resetCardUse') }}
                  </UiButton>
                </div>
              </div>
            </article>
          </div>
        </ConsoleUserDataState>
      </div>
    </UiCard>

    <UiCard :title="t('console.availablePlans')" flush>
      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="productsPending"
          :error="productsError"
          :empty="!availableTiers.length"
          :rows="4"
          :empty-icon="Sparkles"
          :empty-title="t('console.plansEmptyTitle')"
          :empty-description="t('console.plansEmptyBody')"
        >
          <div class="grid gap-3 md:grid-cols-3">
            <article
              v-for="entry in availableTiers"
              :key="entry.tier.id"
              class="flex flex-col gap-4 rounded-card border border-line bg-surface p-5 transition-colors duration-150 hover:border-line-strong"
            >
              <header class="space-y-1">
                <div class="flex items-start justify-between gap-2">
                  <h3 class="text-sm font-semibold text-ink">
                    {{ tierDisplayName(entry.product.name, entry.tier.name) }}
                  </h3>
                  <UiBadge tone="outline">
                    {{ t('console.tierLevel', { level: entry.tier.level }) }}
                  </UiBadge>
                </div>
                <p v-if="entry.tier.description || entry.product.description" class="text-[13px] text-muted">
                  {{ entry.tier.description || entry.product.description }}
                </p>
              </header>

              <ul class="space-y-1 border-y border-line py-3">
                <li
                  v-for="price in entry.prices"
                  :key="price.id"
                  class="flex items-baseline justify-between gap-3 text-[13px]"
                >
                  <span class="text-muted">{{ periodLabel(t, price.billing_period, 'console') }}</span>
                  <span class="numeric text-ink">{{ money(price.price, price.currency) }}</span>
                </li>
              </ul>

              <ul class="flex-1 space-y-1.5">
                <li
                  v-for="benefit in tierBenefits(entry.tier, entry.prices[0])"
                  :key="benefit"
                  class="flex items-start gap-2 text-[13px] text-muted"
                >
                  <Check class="mt-0.5 size-3.5 shrink-0 text-clay" />
                  {{ benefit }}
                </li>
              </ul>

              <UiButton
                block
                :disabled="!canSubscribe"
                @click="openSubscribe(entry.tier.id, entry.prices[0]?.billing_period)"
              >
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
      :title="selectedTierLabel ? t('console.subscribeTitle', { name: selectedTierLabel }) : t('console.subscribe')"
    >
      <div class="space-y-4">
        <UiField :label="t('console.tier')" required>
          <UiSelect
            v-model="selectedTierId"
            :options="subscribeTierOptions"
            :placeholder="t('common.selectPlaceholder')"
          />
        </UiField>

        <UiField :label="t('console.choosePeriod')" required>
          <UiSelect
            v-model="selectedPeriod"
            :options="periodOptions"
            :placeholder="t('common.selectPlaceholder')"
          />
        </UiField>

        <div class="space-y-1.5 rounded-card border border-line bg-sunken p-4 text-[13px]">
          <p class="text-muted">{{ t('console.renewalHint') }}</p>
          <p class="text-faint">{{ t('console.subscriptionMergedHint') }}</p>
        </div>

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
            <span class="numeric text-ink">{{ money(selectedPrice?.price, selectedPrice?.currency) }}</span>
          </div>
          <div v-if="balanceSufficient" class="flex justify-between gap-3 border-t border-line pt-3">
            <span class="text-muted">{{ t('console.balanceAfterPurchase') }}</span>
            <span class="numeric text-ink">{{ formatMoney(availableBalance - purchaseAmount) }}</span>
          </div>
          <UiAlert v-else tone="warn">
            {{ t('console.insufficientBalance', { amount: formatMoney(purchaseAmount - availableBalance) }) }}
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
        <UiButton :loading="busy" :disabled="!canConfirm || (payingWithBalance && !balanceSufficient)" @click="confirmSubscribe">
          {{ payingWithBalance ? t('console.payBalanceAndActivate') : t('console.subscribe') }}
        </UiButton>
      </template>
    </UiSlidePanel>

    <UiDialog
      v-model:open="cancelOpen"
      size="sm"
      :title="t('console.cancelSubscriptionTitle')"
      :description="cancelling?.display_name"
    >
      <p class="text-[13px] text-muted">{{ t('console.cancelSubscriptionBody') }}</p>

      <template #footer>
        <UiButton variant="secondary" @click="cancelOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="confirmCancel">
          {{ t('console.cancelSubscription') }}
        </UiButton>
      </template>
    </UiDialog>

    <UiDialog
      :open="resetOpen"
      size="sm"
      :title="t('console.resetCardUse')"
      :description="resetting?.display_name"
    >
      <p class="text-[13px] text-muted">{{ t('console.resetCardConfirmBody') }}</p>

      <template #footer>
        <UiButton variant="secondary" @click="resetOpen = false; resetting = null">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="confirmReset">
          {{ t('console.resetCardUse') }}
        </UiButton>
      </template>
    </UiDialog>
  </div>
</template>
