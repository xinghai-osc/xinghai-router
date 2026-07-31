<script setup lang="ts">
import type { Group, SubscriptionPlan, SubscriptionPlanForm } from '~/src/api'

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
const maxTokens = ref('')
const sortOrder = ref('0')
const enabled = ref(true)
const invalid = ref('')

const periodOptions = computed(() => [
  { value: 'month', label: t('system.periodMonth') },
  { value: 'year', label: t('system.periodYear') },
])

const groupOptions = computed(() => props.groups.map(group => ({ value: group.id, label: group.name })))

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
  maxTokens.value = plan?.max_tokens_per_period == null ? '' : String(plan.max_tokens_per_period)
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

function submit() {
  if (!name.value.trim()) {
    invalid.value = t('system.planNameRequired')
    return
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
    max_tokens_per_period: optionalNumber(maxTokens.value),
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

        <UiField :label="t('system.maxTokens')" :hint="t('system.limitHint')" for="plan-max-tokens">
          <UiInput id="plan-max-tokens" v-model="maxTokens" />
        </UiField>
      </div>

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
