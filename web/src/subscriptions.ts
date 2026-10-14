import type { BillingPeriod, SubscriptionProduct, SubscriptionTier, SubscriptionTierPrice } from './api'

export const BILLING_PERIODS: BillingPeriod[] = ['hour', 'day', 'week', 'month', 'year']

const PERIOD_SUFFIX: Record<BillingPeriod, string> = {
  hour: 'Hour',
  day: 'Day',
  week: 'Week',
  month: 'Month',
  year: 'Year',
}

/** Which locale namespace owns the billing-period labels for a surface. */
export type PeriodDict = 'site' | 'console' | 'system'

const PERIOD_PREFIX: Record<PeriodDict, string> = {
  site: 'planPer',
  console: 'per',
  system: 'period',
}

export function periodLabel(
  t: (key: string) => string,
  period: BillingPeriod,
  dict: PeriodDict = 'console',
): string {
  const key = `${dict}.${PERIOD_PREFIX[dict]}${PERIOD_SUFFIX[period]}`
  const label = t(key)
  return label === key ? period : label
}

export function sortProducts(products: SubscriptionProduct[]): SubscriptionProduct[] {
  return [...products].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name))
}

export function sortTiers(tiers: SubscriptionTier[]): SubscriptionTier[] {
  return [...tiers].sort((a, b) => a.level - b.level || a.sort_order - b.sort_order)
}

/** Enabled prices of a tier, in display order. */
export function tierPrices(tier: SubscriptionTier, includeDisabled = false): SubscriptionTierPrice[] {
  return (tier.prices ?? [])
    .filter(price => includeDisabled || price.enabled)
    .sort((a, b) => a.sort_order - b.sort_order || BILLING_PERIODS.indexOf(a.billing_period) - BILLING_PERIODS.indexOf(b.billing_period))
}

export function priceFor(tier: SubscriptionTier, period: BillingPeriod): SubscriptionTierPrice | undefined {
  return (tier.prices ?? []).find(price => price.billing_period === period)
}

/** Amounts a purchase of this period grants: the period override, else the tier default. */
export function effectiveAmounts(tier: SubscriptionTier, price?: SubscriptionTierPrice | null) {
  return {
    creditAmount: price?.credit_amount || tier.credit_amount,
    maxRequests: price?.max_requests_per_period ?? tier.max_requests_per_period,
    maxCredit: price?.max_credit_per_period ?? tier.max_credit_per_period,
  }
}

/** Product name alone for a single-tier subscription, otherwise "Product · Tier". */
export function tierDisplayName(productName: string, tierName: string): string {
  const trimmed = (tierName ?? '').trim()
  if (!trimmed || trimmed.toLowerCase() === productName.toLowerCase()) return productName
  return `${productName} · ${trimmed}`
}

export interface TierOption {
  value: string
  label: string
  productId: string
  productName: string
  tierName: string
  level: number
  periods: BillingPeriod[]
}

/** Flat tier list for the admin pickers (redemption codes, reset cards, grants). */
export function tierOptions(products: SubscriptionProduct[], includeDisabled = false): TierOption[] {
  const options: TierOption[] = []
  for (const product of sortProducts(products)) {
    if (!includeDisabled && !product.enabled) continue
    for (const tier of sortTiers(product.tiers ?? [])) {
      if (!includeDisabled && !tier.enabled) continue
      options.push({
        value: tier.id,
        label: tierDisplayName(product.name, tier.name),
        productId: product.id,
        productName: product.name,
        tierName: tier.name,
        level: tier.level,
        periods: tierPrices(tier, includeDisabled).map(price => price.billing_period),
      })
    }
  }
  return options
}

/** Flattened tier + billing period pairs, for pickers that must name a period too. */
export interface TierPriceOption {
  value: string
  tierId: string
  label: string
  period: BillingPeriod
  price: SubscriptionTierPrice
}

export function tierPriceOptions(products: SubscriptionProduct[], includeDisabled = false): TierPriceOption[] {
  const options: TierPriceOption[] = []
  for (const product of sortProducts(products)) {
    if (!includeDisabled && !product.enabled) continue
    for (const tier of sortTiers(product.tiers ?? [])) {
      if (!includeDisabled && !tier.enabled) continue
      for (const price of tierPrices(tier, includeDisabled)) {
        options.push({
          value: `${tier.id}:${price.billing_period}`,
          tierId: tier.id,
          label: `${tierDisplayName(product.name, tier.name)} · ${price.price} ${price.currency}`,
          period: price.billing_period,
          price,
        })
      }
    }
  }
  return options
}

/** Cheapest enabled price of a tier, used for the "from" figure on public cards. */
export function cheapestPrice(tier: SubscriptionTier): SubscriptionTierPrice | null {
  const prices = tierPrices(tier)
  if (!prices.length) return null
  return prices.reduce((lowest, price) => (Number(price.price) < Number(lowest.price) ? price : lowest))
}
