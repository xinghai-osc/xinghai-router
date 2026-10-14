<script setup lang="ts">
import { Check } from 'lucide-vue-next'
import type { BillingPeriod, SubscriptionProduct, SubscriptionTier, SubscriptionTierPrice } from '~/src/api'
import { formatMoney, formatNumber } from '~/src/format'
import { effectiveAmounts, periodLabel, sortProducts, sortTiers, tierPrices } from '~/src/subscriptions'

const props = withDefaults(defineProps<{
  products: SubscriptionProduct[]
  loading?: boolean
  error?: string
  /** Index of the product rendered as the recommended one. */
  featured?: number
}>(), { featured: 1 })

defineEmits<{ retry: [] }>()

const { t } = useI18n()

interface CardSelection { tierId: string; period: BillingPeriod }

const sorted = computed(() => sortProducts(props.products))
const selection = ref<Record<string, CardSelection>>({})

function purchasableTiers(product: SubscriptionProduct): SubscriptionTier[] {
  return sortTiers(product.tiers ?? []).filter(tier => tier.enabled && tierPrices(tier).length > 0)
}

watch(sorted, (products) => {
  const next: Record<string, CardSelection> = {}
  for (const product of products) {
    const tiers = purchasableTiers(product)
    const current = selection.value[product.id]
    const tier = tiers.find(item => item.id === current?.tierId) ?? tiers[0]
    if (!tier) continue
    const prices = tierPrices(tier)
    const period = prices.find(price => price.billing_period === current?.period)?.billing_period ?? prices[0]?.billing_period
    if (period) next[product.id] = { tierId: tier.id, period }
  }
  selection.value = next
}, { immediate: true })

function currentTier(product: SubscriptionProduct): SubscriptionTier | undefined {
  const tiers = purchasableTiers(product)
  return tiers.find(tier => tier.id === selection.value[product.id]?.tierId) ?? tiers[0]
}

function currentPrices(product: SubscriptionProduct): SubscriptionTierPrice[] {
  const tier = currentTier(product)
  return tier ? tierPrices(tier) : []
}

function currentPrice(product: SubscriptionProduct): SubscriptionTierPrice | undefined {
  const prices = currentPrices(product)
  const period = selection.value[product.id]?.period
  return prices.find(price => price.billing_period === period) ?? prices[0]
}

function tierSelectOptions(product: SubscriptionProduct) {
  return purchasableTiers(product).map(tier => ({ value: tier.id, label: tier.name }))
}

function onTierChange(product: SubscriptionProduct) {
  return (tierId: string) => selectTier(product, tierId)
}

function selectTier(product: SubscriptionProduct, tierId: string) {
  const tier = purchasableTiers(product).find(item => item.id === tierId)
  const period = tier ? tierPrices(tier)[0]?.billing_period : undefined
  if (period) selection.value[product.id] = { tierId, period }
}

function selectPeriod(product: SubscriptionProduct, period: BillingPeriod) {
  const tier = currentTier(product)
  if (tier) selection.value[product.id] = { tierId: tier.id, period }
}

function priceText(price: SubscriptionTierPrice | undefined): string {
  if (!price) return '—'
  const currency = (price.currency || 'CNY').toUpperCase()
  return currency === 'CNY' ? formatMoney(price.price) : `${price.price} ${currency}`
}

function periodText(product: SubscriptionProduct): string {
  const price = currentPrice(product)
  return price ? periodLabel(t, price.billing_period, 'site') : ''
}

function productBenefits(product: SubscriptionProduct): string[] {
  const tier = currentTier(product)
  const price = currentPrice(product)
  if (!tier || !price) return []
  const list: string[] = []
  if (purchasableTiers(product).length > 1) list.push(t('site.planTierLevel', { level: tier.level }))
  const amounts = effectiveAmounts(tier, price)
  if (amounts.creditAmount) {
    list.push(t('site.planCredit', { amount: Number(amounts.creditAmount).toFixed(2) }))
  } else if (amounts.maxCredit === null
    && !(tier.model_quotas ?? []).some(quota => quota.max_credit_per_period !== null)) {
    list.push(t('site.planUnlimitedCredit'))
  }
  if (amounts.maxRequests !== null) {
    list.push(t('site.planQuotaRequests', { count: formatNumber(amounts.maxRequests) }))
  }
  if (amounts.maxCredit !== null) {
    list.push(t('site.planQuotaCredit', { amount: formatMoney(amounts.maxCredit) }))
  }
  for (const quota of tier.model_quotas ?? []) {
    list.push(t('site.planModelQuota', { model: quota.model }))
  }
  if (tier.group_name) list.push(t('site.planGroup', { group: tier.group_name }))
  list.push(tier.model_whitelist.length
    ? t('site.planWhitelist', { count: tier.model_whitelist.length })
    : t('site.planAllModels'))
  return list
}

function signupTo(product: SubscriptionProduct): string {
  const tier = currentTier(product)
  const price = currentPrice(product)
  if (!tier || !price) return '/auth?mode=register'
  return `/auth?mode=register&tier=${encodeURIComponent(tier.id)}&period=${price.billing_period}`
}
</script>

<template>
  <SiteDataLoader v-if="loading" />

  <UiAlert v-else-if="error" tone="danger" :title="t('common.loadFailed')">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <p>{{ error }}</p>
      <UiButton variant="secondary" size="sm" @click="$emit('retry')">{{ t('common.retry') }}</UiButton>
    </div>
  </UiAlert>

  <UiEmptyState
    v-else-if="!sorted.length"
    :title="t('site.planEmptyTitle')"
    :description="t('site.planEmptyBody')"
  >
    <UiButton to="/auth?mode=register" size="sm">{{ t('site.planEmptyCta') }}</UiButton>
  </UiEmptyState>

  <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
    <article
      v-for="(product, index) in sorted"
      :key="product.id"
      :class="[
        'relative flex min-w-0 flex-col gap-6 rounded-card border p-6 transition-colors duration-150 ease-out lg:p-8',
        index === featured ? 'border-clay/60 bg-clay-soft/30' : 'border-line bg-surface hover:border-line-strong',
      ]"
    >
      <UiBadge v-if="index === featured" tone="clay" class="absolute -top-2.5 left-6">
        {{ t('site.planRecommended') }}
      </UiBadge>

      <header class="space-y-1">
        <h3 class="text-[15px] font-semibold text-ink">{{ product.name }}</h3>
        <p v-if="product.description" class="text-[13px] text-muted">{{ product.description }}</p>
      </header>

      <div v-if="purchasableTiers(product).length > 1" class="space-y-1.5">
        <label :for="`plan-tier-${product.id}`" class="block text-2xs font-medium tracking-wide text-faint uppercase">
          {{ t('site.planTier') }}
        </label>
        <UiSelect
          :id="`plan-tier-${product.id}`"
          size="sm"
          :model-value="currentTier(product)?.id ?? ''"
          :options="tierSelectOptions(product)"
          @update:model-value="onTierChange(product)"
        />
      </div>

      <div v-if="currentPrices(product).length > 1" class="space-y-1.5">
        <p :id="`plan-period-${product.id}`" class="text-2xs font-medium tracking-wide text-faint uppercase">
          {{ t('site.planChoosePeriod') }}
        </p>
        <div
          class="inline-flex flex-wrap items-center gap-0.5 rounded-control border border-line-strong bg-surface p-0.5"
          role="group"
          :aria-labelledby="`plan-period-${product.id}`"
        >
          <button
            v-for="price in currentPrices(product)"
            :key="price.id"
            type="button"
            :class="[
              'rounded-[7px] px-2.5 py-1.5 text-[13px] transition-colors duration-150 ease-out',
              currentPrice(product)?.billing_period === price.billing_period ? 'bg-sunken text-ink' : 'text-muted hover:text-ink',
            ]"
            :aria-pressed="currentPrice(product)?.billing_period === price.billing_period"
            @click="selectPeriod(product, price.billing_period)"
          >{{ periodLabel(t, price.billing_period, 'site') }}</button>
        </div>
      </div>

      <p class="flex items-baseline gap-1">
        <span class="display text-4xl text-ink">{{ priceText(currentPrice(product)) }}</span>
        <span v-if="currentPrice(product)" class="text-[13px] text-muted">
          {{ t('site.planPerPeriod', { period: periodText(product) }) }}
        </span>
      </p>

      <ul class="flex-1 space-y-3 border-t border-line pt-5">
        <li v-for="item in productBenefits(product)" :key="item" class="flex items-start gap-2 text-[13px] text-muted">
          <Check class="mt-0.5 size-3.5 shrink-0 text-clay" />
          {{ item }}
        </li>
      </ul>

      <footer class="space-y-3">
        <UiButton
          :to="signupTo(product)"
          :variant="index === featured ? 'primary' : 'secondary'"
          :disabled="!currentTier(product)"
          block
        >{{ t('site.planChoose', { name: product.name }) }}</UiButton>
        <p class="text-2xs leading-relaxed text-faint">{{ t('site.planRenewHint') }}</p>
      </footer>
    </article>
  </div>
</template>
