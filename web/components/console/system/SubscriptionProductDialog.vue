<script setup lang="ts">
import { Plus, Trash2 } from 'lucide-vue-next'
import type {
  BillingPeriod, Group, OveragePolicy, SubscriptionProduct, SubscriptionProductForm, SubscriptionTierForm,
} from '~/src/api'
import { BILLING_PERIODS, periodLabel, sortTiers, tierPrices } from '~/src/subscriptions'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  product: SubscriptionProduct | null
  groups: Group[]
  busy?: boolean
}>()

const emit = defineEmits<{ submit: [form: SubscriptionProductForm] }>()

const { t } = useI18n()

interface PriceDraft {
  key: string
  id: string
  billing_period: BillingPeriod
  price: string
  currency: string
  credit_amount: string
  max_requests_per_period: string
  max_credit_per_period: string
  enabled: boolean
  sort_order: number
}

interface ModelQuotaDraft {
  key: string
  model: string
  max_requests_per_period: string
  max_credit_per_period: string
}

interface TierDraft {
  key: string
  id: string
  level: number
  name: string
  description: string
  credit_amount: string
  group_id: string
  whitelistText: string
  max_requests_per_period: string
  max_credit_per_period: string
  overage_policy: OveragePolicy
  model_quotas: ModelQuotaDraft[]
  prices: PriceDraft[]
  sort_order: number
  enabled: boolean
}

let draftSequence = 0
function nextKey(prefix: string): string {
  draftSequence += 1
  return `${prefix}-${draftSequence}`
}

const name = ref('')
const description = ref('')
const sortOrder = ref('0')
const enabled = ref(true)
const tiers = ref<TierDraft[]>([])
const invalid = ref('')
const nameError = ref('')

const periodOptions = computed(() => BILLING_PERIODS.map(period => ({
  value: period,
  label: periodLabel(t, period, 'system'),
})))

const overageOptions = computed(() => [
  { value: 'allow_wallet', label: t('system.overageWallet') },
  { value: 'block', label: t('system.overageBlock') },
])

const groupOptions = computed(() => [
  { value: '', label: t('common.none') },
  ...props.groups.map(group => ({
    value: group.id,
    label: group.display_name || group.name,
  })),
])

function optionalText(value: number | null | undefined): string {
  return value == null ? '' : String(value)
}

function reset(product: SubscriptionProduct | null) {
  invalid.value = ''
  nameError.value = ''
  name.value = product?.name ?? ''
  description.value = product?.description ?? ''
  sortOrder.value = String(product?.sort_order ?? 0)
  enabled.value = product?.enabled ?? true
  tiers.value = sortTiers(product?.tiers ?? []).map(tier => ({
    key: nextKey('tier'),
    id: tier.id,
    level: tier.level,
    name: tier.name,
    description: tier.description,
    credit_amount: tier.credit_amount,
    group_id: tier.group_id,
    whitelistText: (tier.model_whitelist ?? []).join('\n'),
    max_requests_per_period: optionalText(tier.max_requests_per_period),
    max_credit_per_period: optionalText(tier.max_credit_per_period),
    overage_policy: tier.overage_policy,
    model_quotas: (tier.model_quotas ?? []).map(quota => ({
      key: nextKey('quota'),
      model: quota.model,
      max_requests_per_period: optionalText(quota.max_requests_per_period),
      max_credit_per_period: optionalText(quota.max_credit_per_period),
    })),
    prices: tierPrices(tier, true).map(price => ({
      key: nextKey('price'),
      id: price.id,
      billing_period: price.billing_period,
      price: price.price,
      currency: price.currency,
      credit_amount: price.credit_amount,
      max_requests_per_period: optionalText(price.max_requests_per_period),
      max_credit_per_period: optionalText(price.max_credit_per_period),
      enabled: price.enabled,
      sort_order: price.sort_order,
    })),
    sort_order: tier.sort_order,
    enabled: tier.enabled,
  }))
}

watch(() => [open.value, props.product] as const, ([isOpen]) => {
  if (isOpen) reset(props.product)
}, { immediate: true })

function tierLabel(tier: TierDraft, index: number): string {
  return tier.name.trim() || `${t('system.tierLevel')} ${index + 1}`
}

function nextLevel(): number {
  return tiers.value.reduce((highest, tier) => Math.max(highest, normalizeLevel(tier.level)), 0) + 1
}

function addTier() {
  tiers.value.push({
    key: nextKey('tier'),
    id: '',
    level: nextLevel(),
    name: '',
    description: '',
    credit_amount: '',
    group_id: '',
    whitelistText: '',
    max_requests_per_period: '',
    max_credit_per_period: '',
    overage_policy: 'allow_wallet',
    model_quotas: [],
    prices: [],
    sort_order: tiers.value.length,
    enabled: true,
  })
}

function removeTier(index: number) {
  tiers.value.splice(index, 1)
}

function addPrice(tier: TierDraft) {
  const used = new Set(tier.prices.map(price => price.billing_period))
  tier.prices.push({
    key: nextKey('price'),
    id: '',
    billing_period: BILLING_PERIODS.find(period => !used.has(period)) ?? 'month',
    price: '',
    currency: 'CNY',
    credit_amount: '',
    max_requests_per_period: '',
    max_credit_per_period: '',
    enabled: true,
    sort_order: tier.prices.length,
  })
}

function removePrice(tier: TierDraft, index: number) {
  tier.prices.splice(index, 1)
}

function addQuota(tier: TierDraft) {
  tier.model_quotas.push({
    key: nextKey('quota'),
    model: '',
    max_requests_per_period: '',
    max_credit_per_period: '',
  })
}

function removeQuota(tier: TierDraft, index: number) {
  tier.model_quotas.splice(index, 1)
}

function normalizeLevel(value: number | string): number {
  const parsed = Math.trunc(Number(value))
  return Number.isFinite(parsed) && parsed >= 1 ? parsed : 1
}

function optionalNumber(value: string): number | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : null
}

function sortOrderValue(value: number | string): number {
  const parsed = Math.trunc(Number(value))
  return Number.isFinite(parsed) ? parsed : 0
}

function buildTier(tier: TierDraft): SubscriptionTierForm {
  return {
    id: tier.id,
    level: normalizeLevel(tier.level),
    name: tier.name.trim(),
    description: tier.description.trim(),
    credit_amount: tier.credit_amount.trim(),
    group_id: tier.group_id,
    model_whitelist: tier.whitelistText.split(/[,\n]/).map(entry => entry.trim()).filter(Boolean),
    max_requests_per_period: optionalNumber(tier.max_requests_per_period),
    max_credit_per_period: optionalNumber(tier.max_credit_per_period),
    overage_policy: tier.overage_policy,
    model_quotas: tier.model_quotas.map(quota => ({
      model: quota.model.trim(),
      max_requests_per_period: optionalNumber(quota.max_requests_per_period),
      max_credit_per_period: optionalNumber(quota.max_credit_per_period),
    })),
    prices: tier.prices.map(price => ({
      id: price.id,
      billing_period: price.billing_period,
      price: price.price.trim(),
      currency: price.currency.trim(),
      credit_amount: price.credit_amount.trim(),
      max_requests_per_period: optionalNumber(price.max_requests_per_period),
      max_credit_per_period: optionalNumber(price.max_credit_per_period),
      enabled: price.enabled,
      sort_order: sortOrderValue(price.sort_order),
    })),
    sort_order: sortOrderValue(tier.sort_order),
    enabled: tier.enabled,
  }
}

function submit() {
  invalid.value = ''
  nameError.value = ''
  if (!name.value.trim()) {
    nameError.value = t('system.planNameRequired')
    return
  }
  if (tiers.value.length > 1 && tiers.value.some(tier => !tier.name.trim())) {
    invalid.value = t('system.tierNameRequired')
    return
  }
  const levels = new Set<number>()
  for (const tier of tiers.value) {
    const level = normalizeLevel(tier.level)
    if (levels.has(level)) {
      invalid.value = t('system.tierLevelDuplicate')
      return
    }
    levels.add(level)
  }
  const names = new Set<string>()
  for (const tier of tiers.value) {
    const key = tier.name.trim().toLowerCase()
    if (!key) continue
    if (names.has(key)) {
      invalid.value = t('system.tierNameDuplicate')
      return
    }
    names.add(key)
  }
  for (const tier of tiers.value) {
    const periods = new Set<BillingPeriod>()
    for (const price of tier.prices) {
      if (periods.has(price.billing_period)) {
        invalid.value = t('system.pricePeriodDuplicate')
        return
      }
      periods.add(price.billing_period)
    }
  }
  for (const tier of tiers.value) {
    const models = new Set<string>()
    for (const quota of tier.model_quotas) {
      const model = quota.model.trim()
      if (!model) {
        invalid.value = t('system.modelQuotaModelRequired')
        return
      }
      if (models.has(model)) {
        invalid.value = t('system.modelQuotaDuplicate', { model })
        return
      }
      models.add(model)
      if (optionalNumber(quota.max_requests_per_period) == null && optionalNumber(quota.max_credit_per_period) == null) {
        invalid.value = t('system.modelQuotaLimitRequired', { model })
        return
      }
    }
  }
  emit('submit', {
    name: name.value.trim(),
    description: description.value.trim(),
    sort_order: sortOrderValue(sortOrder.value),
    enabled: enabled.value,
    tiers: tiers.value.map(buildTier),
  })
}
</script>

<template>
  <UiDialog
    v-model:open="open"
    size="lg"
    :title="product ? t('system.editProduct') : t('system.newProduct')"
  >
    <form class="space-y-5" @submit.prevent="submit">
      <UiAlert v-if="invalid" tone="danger">{{ invalid }}</UiAlert>

      <div class="grid gap-4 sm:grid-cols-2">
        <UiField :label="t('system.productName')" :error="nameError" required>
          <UiInput v-model="name" :placeholder="t('system.productNamePlaceholder')" />
        </UiField>

        <UiField :label="t('system.productDescription')" :hint="t('system.productDescriptionHint')">
          <UiInput v-model="description" />
        </UiField>

        <UiField :label="t('system.productSortOrder')" :hint="t('system.sortOrderHint')">
          <UiInput v-model="sortOrder" type="number" />
        </UiField>

        <UiField :label="t('system.productEnabled')">
          <div class="flex h-10 items-center">
            <UiSwitch v-model="enabled" :label="t('system.productEnabled')" />
          </div>
        </UiField>
      </div>

      <section class="space-y-3 border-t border-line pt-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0 space-y-1">
            <h3 class="text-sm font-semibold text-ink">{{ t('system.tiersTitle') }}</h3>
            <p class="text-[13px] text-muted">{{ t('system.tiersLead') }}</p>
          </div>
          <UiButton variant="secondary" size="sm" @click="addTier">
            <Plus class="size-4" />
            {{ t('system.addTier') }}
          </UiButton>
        </div>

        <div
          v-for="(tier, tierIndex) in tiers"
          :key="tier.key"
          class="space-y-4 rounded-card border border-line bg-surface p-3.5"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="text-sm font-semibold text-ink">{{ tierLabel(tier, tierIndex) }}</p>
            <div class="flex items-center gap-3">
              <div class="flex items-center gap-2">
                <UiSwitch v-model="tier.enabled" :label="t('system.tierEnabled')" />
                <span class="text-[13px] text-muted">{{ t('system.tierEnabled') }}</span>
              </div>
              <UiButton variant="ghost" size="icon" :aria-label="t('system.removeTier')" @click="removeTier(tierIndex)">
                <Trash2 class="size-4" />
              </UiButton>
            </div>
          </div>

          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('system.tierLevel')" :hint="t('system.tierLevelHint')">
              <UiInput v-model="tier.level" type="number" />
            </UiField>

            <UiField :label="t('system.tierName')" :hint="t('system.tierNameHint')">
              <UiInput v-model="tier.name" :placeholder="t('system.tierNamePlaceholder')" />
            </UiField>

            <UiField :label="t('system.tierDescription')">
              <UiInput v-model="tier.description" />
            </UiField>

            <UiField :label="t('system.tierCredit')" :hint="t('system.tierCreditHint')">
              <UiInput v-model="tier.credit_amount" />
            </UiField>

            <UiField :label="t('system.tierGroup')" :hint="t('system.planGroupHint')">
              <UiSelect v-model="tier.group_id" :options="groupOptions" />
            </UiField>

            <UiField :label="t('system.tierOverage')" :hint="t('system.overagePolicyHint')">
              <UiSelect v-model="tier.overage_policy" :options="overageOptions" />
            </UiField>

            <UiField :label="t('system.maxRequests')" :hint="t('system.limitHint')">
              <UiInput v-model="tier.max_requests_per_period" inputmode="numeric" />
            </UiField>

            <UiField :label="t('system.maxCredit')" :hint="t('system.limitHint')">
              <UiInput v-model="tier.max_credit_per_period" inputmode="decimal" />
            </UiField>

            <UiField :label="t('system.sortOrder')" :hint="t('system.sortOrderHint')">
              <UiInput v-model="tier.sort_order" type="number" />
            </UiField>
          </div>

          <UiField :label="t('system.tierWhitelist')" :hint="t('system.modelWhitelistHint')">
            <UiTextarea v-model="tier.whitelistText" mono :rows="4" />
          </UiField>

          <UiField :label="t('system.tierModelQuotas')" :hint="t('system.modelQuotasHint')">
            <div class="overflow-hidden rounded-control border border-line">
              <div class="grid grid-cols-[1fr_1fr_1fr_auto] gap-2 bg-sunken px-3 py-2 text-[13px] font-medium text-muted">
                <span>{{ t('system.model') }}</span>
                <span>{{ t('system.maxRequests') }}</span>
                <span>{{ t('system.maxCredit') }}</span>
                <span />
              </div>
              <div
                v-for="(quota, quotaIndex) in tier.model_quotas"
                :key="quota.key"
                class="grid grid-cols-[1fr_1fr_1fr_auto] items-center gap-2 border-t border-line px-3 py-2"
              >
                <UiInput v-model="quota.model" :placeholder="t('system.modelPlaceholder')" />
                <UiInput v-model="quota.max_requests_per_period" :placeholder="t('system.limitHint')" inputmode="numeric" />
                <UiInput v-model="quota.max_credit_per_period" :placeholder="t('system.limitHint')" inputmode="decimal" />
                <UiButton variant="ghost" size="icon" :aria-label="t('common.remove')" @click="removeQuota(tier, quotaIndex)">
                  <Trash2 class="size-4" />
                </UiButton>
              </div>
              <div v-if="!tier.model_quotas.length" class="border-t border-line px-3 py-4 text-[13px] text-faint">
                {{ t('system.modelQuotasEmpty') }}
              </div>
            </div>
            <div class="mt-2">
              <UiButton variant="secondary" size="sm" @click="addQuota(tier)">
                <Plus class="size-4" />
                {{ t('system.modelQuotaAdd') }}
              </UiButton>
            </div>
          </UiField>

          <div class="space-y-3 border-t border-line pt-3">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div class="min-w-0 space-y-1">
                <h4 class="text-[13px] font-semibold text-ink">{{ t('system.pricesTitle') }}</h4>
                <p class="text-[13px] text-muted">{{ t('system.pricesLead') }}</p>
              </div>
              <UiButton variant="secondary" size="sm" @click="addPrice(tier)">
                <Plus class="size-4" />
                {{ t('system.addPrice') }}
              </UiButton>
            </div>

            <div v-for="(price, priceIndex) in tier.prices" :key="price.key" class="rounded-control border border-line">
              <div class="flex items-center justify-between gap-2 border-b border-line bg-sunken px-3 py-1.5">
                <span class="text-[13px] font-medium text-ink">{{ periodLabel(t, price.billing_period, 'system') }}</span>
                <div class="flex items-center gap-2">
                  <UiSwitch v-model="price.enabled" :label="t('common.enable')" />
                  <UiButton variant="ghost" size="icon" :aria-label="t('system.removePrice')" @click="removePrice(tier, priceIndex)">
                    <Trash2 class="size-4" />
                  </UiButton>
                </div>
              </div>

              <div class="grid gap-4 p-3 sm:grid-cols-3">
                <UiField :label="t('system.pricePeriod')">
                  <UiSelect v-model="price.billing_period" :options="periodOptions" />
                </UiField>

                <UiField :label="t('system.priceAmount')">
                  <UiInput v-model="price.price" inputmode="decimal" />
                </UiField>

                <UiField :label="t('system.priceCurrency')">
                  <UiInput v-model="price.currency" :placeholder="t('system.currencyPlaceholder')" />
                </UiField>

                <UiField :label="t('system.priceCredit')">
                  <UiInput v-model="price.credit_amount" :placeholder="t('system.priceInherit')" />
                </UiField>

                <UiField :label="t('system.priceRequests')">
                  <UiInput v-model="price.max_requests_per_period" :placeholder="t('system.priceInherit')" inputmode="numeric" />
                </UiField>

                <UiField :label="t('system.priceCreditQuota')">
                  <UiInput v-model="price.max_credit_per_period" :placeholder="t('system.priceInherit')" inputmode="decimal" />
                </UiField>

                <UiField :label="t('system.sortOrder')">
                  <UiInput v-model="price.sort_order" type="number" />
                </UiField>
              </div>
            </div>
          </div>
        </div>
      </section>
    </form>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.cancel') }}</UiButton>
      <UiButton :loading="busy" @click="submit">{{ t('common.save') }}</UiButton>
    </template>
  </UiDialog>
</template>
