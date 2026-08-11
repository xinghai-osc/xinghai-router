<script setup lang="ts">
import { Plus, Trash2 } from 'lucide-vue-next'
import type { Group, OveragePolicy, SubscriptionPlan, SubscriptionPlanForm, SubscriptionPlanModelQuota } from '~/src/api'

const open = defineModel<boolean>('open', { default: false })

const props = defineProps<{
  plan: SubscriptionPlan | null
  groups: Group[]
  busy?: boolean
}>()

const emit = defineEmits<{ submit: [form: SubscriptionPlanForm] }>()

const { t } = useI18n()

const name = ref('')
const description = ref('')
const price = ref('')
const currency = ref('CNY')
const billingPeriod = ref('month')
const creditAmount = ref('')
const groupId = ref('')
const whitelistText = ref('')
const maxRequests = ref('')
const maxCredit = ref('')
const overagePolicy = ref<OveragePolicy>('allow_wallet')
const modelQuotas = ref<SubscriptionPlanModelQuota[]>([])
const sortOrder = ref('0')
const enabled = ref(true)
const invalid = ref('')

const periodOptions = computed(() => [
  { value: 'hour', label: t('system.periodHour') },
  { value: 'day', label: t('system.periodDay') },
  { value: 'week', label: t('system.periodWeek') },
  { value: 'month', label: t('system.periodMonth') },
  { value: 'year', label: t('system.periodYear') },
])

const overageOptions = computed(() => [
  { value: 'allow_wallet', label: t('system.overageWallet') },
  { value: 'block', label: t('system.overageBlock') },
])

const groupOptions = computed(() => props.groups.map(group => ({ value: group.id, label: group.display_name || group.name })))

function reset(plan: SubscriptionPlan | null) {
  invalid.value = ''
  name.value = plan?.name ?? ''
  description.value = plan?.description ?? ''
  price.value = plan?.price ?? ''
  currency.value = plan?.currency || 'CNY'
  billingPeriod.value = plan?.billing_period ?? 'month'
  creditAmount.value = plan?.credit_amount ?? ''
  groupId.value = plan?.group_id ?? ''
  whitelistText.value = (plan?.model_whitelist ?? []).join('\n')
  maxRequests.value = plan?.max_requests_per_period == null ? '' : String(plan.max_requests_per_period)
  maxCredit.value = plan?.max_credit_per_period == null ? '' : String(plan.max_credit_per_period)
  overagePolicy.value = plan?.overage_policy ?? 'allow_wallet'
  modelQuotas.value = (plan?.model_quotas ?? []).map(quota => ({ ...quota }))
  sortOrder.value = String(plan?.sort_order ?? 0)
  enabled.value = plan?.enabled ?? true
}

watch(() => [open.value, props.plan] as const, ([isOpen]) => {
  if (isOpen) reset(props.plan)
}, { immediate: true })

function optionalNumber(value: string): number | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : null
}

function addQuotaRow() {
  modelQuotas.value.push({ model: '', max_requests_per_period: null, max_credit_per_period: null })
}

function removeQuotaRow(index: number) {
  modelQuotas.value.splice(index, 1)
}

function submit() {
  if (!name.value.trim()) {
    invalid.value = t('system.planNameRequired')
    return
  }
  if (modelQuotas.value.some(quota => !quota.model.trim())) {
    invalid.value = t('system.modelQuotaModelRequired')
    return
  }
  const seen = new Set<string>()
  for (const quota of modelQuotas.value) {
    const model = quota.model.trim()
    if (seen.has(model)) {
      invalid.value = t('system.modelQuotaDuplicate', { model })
      return
    }
    seen.add(model)
    if (optionalNumber(String(quota.max_requests_per_period ?? '')) == null
      && optionalNumber(String(quota.max_credit_per_period ?? '')) == null) {
      invalid.value = t('system.modelQuotaLimitRequired', { model })
      return
    }
  }
  invalid.value = ''
  emit('submit', {
    name: name.value.trim(),
    description: description.value.trim(),
    price: price.value.trim(),
    currency: currency.value.trim(),
    billing_period: billingPeriod.value,
    credit_amount: creditAmount.value.trim(),
    group_id: groupId.value,
    model_whitelist: whitelistText.value.split('\n').map(line => line.trim()).filter(Boolean),
    max_requests_per_period: optionalNumber(maxRequests.value),
    max_credit_per_period: optionalNumber(maxCredit.value),
    overage_policy: overagePolicy.value,
    model_quotas: modelQuotas.value.map(quota => ({
      model: quota.model.trim(),
      max_requests_per_period: optionalNumber(String(quota.max_requests_per_period ?? '')),
      max_credit_per_period: optionalNumber(String(quota.max_credit_per_period ?? '')),
    })),
    sort_order: Number(sortOrder.value) || 0,
    enabled: enabled.value,
  })
}
</script>

<template>
  <UiDialog
    v-model:open="open"
    size="lg"
    :title="plan ? t('system.editPlan') : t('system.newPlan')"
  >
    <form class="space-y-4" @submit.prevent="submit">
      <div class="grid gap-4 sm:grid-cols-2">
        <UiField :label="t('common.name')" :error="invalid" required for="plan-name">
          <UiInput id="plan-name" v-model="name" :placeholder="t('system.planNamePlaceholder')" />
        </UiField>

        <UiField :label="t('system.planGroup')" :hint="t('system.planGroupHint')" for="plan-group">
          <UiSelect
            id="plan-group"
            v-model="groupId"
            :options="groupOptions"
            :placeholder="t('common.selectPlaceholder')"
          />
        </UiField>
      </div>

      <UiField :label="t('system.planDescription')" :hint="t('system.planDescriptionHint')" for="plan-description">
        <UiInput id="plan-description" v-model="description" />
      </UiField>

      <div class="grid gap-4 sm:grid-cols-3">
        <UiField :label="t('system.price')" for="plan-price">
          <UiInput id="plan-price" v-model="price" />
        </UiField>

        <UiField :label="t('system.currency')" for="plan-currency">
          <UiInput id="plan-currency" v-model="currency" :placeholder="t('system.currencyPlaceholder')" />
        </UiField>

        <UiField :label="t('system.billingPeriod')" for="plan-period">
          <UiSelect
            id="plan-period"
            v-model="billingPeriod"
            :options="periodOptions"
            :placeholder="t('common.selectPlaceholder')"
          />
        </UiField>
      </div>

      <div class="grid gap-4 sm:grid-cols-3">
        <UiField :label="t('system.creditAmount')" :hint="t('system.creditAmountHint')" for="plan-credit">
          <UiInput id="plan-credit" v-model="creditAmount" />
        </UiField>

        <UiField :label="t('system.maxRequests')" :hint="t('system.limitHint')" for="plan-max-requests">
          <UiInput id="plan-max-requests" v-model="maxRequests" />
        </UiField>

        <UiField :label="t('system.maxCredit')" :hint="t('system.limitHint')" for="plan-max-credit">
          <UiInput id="plan-max-credit" v-model="maxCredit" inputmode="decimal" />
        </UiField>
      </div>

      <UiField :label="t('system.overagePolicy')" :hint="t('system.overagePolicyHint')" for="plan-overage">
        <UiSelect
          id="plan-overage"
          v-model="overagePolicy"
          :options="overageOptions"
          :placeholder="t('common.selectPlaceholder')"
        />
      </UiField>

      <UiField :label="t('system.modelQuotas')" :hint="t('system.modelQuotasHint')">
        <div class="overflow-hidden rounded-control border border-line">
          <div class="grid grid-cols-[1fr_1fr_1fr_auto] gap-2 bg-sunken px-3 py-2 text-[13px] font-medium text-muted">
            <span>{{ t('system.model') }}</span>
            <span>{{ t('system.maxRequests') }}</span>
            <span>{{ t('system.maxCredit') }}</span>
            <span />
          </div>
          <div v-for="(quota, index) in modelQuotas" :key="index" class="grid grid-cols-[1fr_1fr_1fr_auto] items-center gap-2 border-t border-line px-3 py-2">
            <UiInput v-model="quota.model" :placeholder="t('system.modelPlaceholder')" />
            <UiInput v-model="quota.max_requests_per_period" :placeholder="t('system.limitHint')" inputmode="numeric" />
            <UiInput v-model="quota.max_credit_per_period" :placeholder="t('system.limitHint')" inputmode="decimal" />
            <UiButton variant="ghost" size="icon" aria-label="Remove" @click="removeQuotaRow(index)">
              <Trash2 class="size-4 text-muted" />
            </UiButton>
          </div>
          <div v-if="modelQuotas.length === 0" class="border-t border-line px-3 py-4 text-[13px] text-faint">
            {{ t('system.modelQuotasEmpty') }}
          </div>
        </div>
        <div class="mt-2">
          <UiButton variant="secondary" size="sm" @click="addQuotaRow">
            <Plus class="size-4" />
            {{ t('system.modelQuotaAdd') }}
          </UiButton>
        </div>
      </UiField>

      <UiField :label="t('system.modelWhitelist')" :hint="t('system.modelWhitelistHint')" for="plan-whitelist">
        <UiTextarea id="plan-whitelist" v-model="whitelistText" mono :rows="6" />
      </UiField>

      <div class="grid gap-4 sm:grid-cols-2">
        <UiField :label="t('system.sortOrder')" :hint="t('system.sortOrderHint')" for="plan-sort">
          <UiInput id="plan-sort" v-model="sortOrder" />
        </UiField>

        <UiField :label="t('common.enable')">
          <div class="flex h-10 items-center">
            <UiSwitch v-model="enabled" :label="t('common.enable')" />
          </div>
        </UiField>
      </div>
    </form>

    <template #footer>
      <UiButton variant="secondary" @click="open = false">{{ t('common.cancel') }}</UiButton>
      <UiButton :loading="busy" @click="submit">{{ t('common.save') }}</UiButton>
    </template>
  </UiDialog>
</template>
