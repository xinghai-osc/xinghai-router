<script setup lang="ts">
import { Check } from 'lucide-vue-next'
import type { PublicSubscriptionPlan } from '~/src/api'
import { formatMoney, formatNumber } from '~/src/format'

const props = withDefaults(defineProps<{
  plans: PublicSubscriptionPlan[]
  loading?: boolean
  error?: string
  /** Index of the plan rendered as the recommended one. */
  featured?: number
}>(), { featured: 1 })

defineEmits<{ retry: [] }>()

const { t } = useI18n()

const sorted = computed(() => [...props.plans].sort((a, b) => a.sort_order - b.sort_order))

const PERIOD_KEYS: Record<string, string> = {
  hour: 'site.planPerHour',
  day: 'site.planPerDay',
  week: 'site.planPerWeek',
  month: 'site.planPerMonth',
  year: 'site.planPerYear',
}

function periodLabel(plan: PublicSubscriptionPlan): string {
  return PERIOD_KEYS[plan.billing_period] ? t(PERIOD_KEYS[plan.billing_period]) : plan.billing_period
}

function benefits(plan: PublicSubscriptionPlan): string[] {
  const list: string[] = []
  if (plan.credit_amount) {
    list.push(t('site.planCredit', { amount: Number(plan.credit_amount).toFixed(2) }))
  } else if (plan.max_credit_per_period === null
    && !(plan.model_quotas ?? []).some(quota => quota.max_credit_per_period !== null)) {
    list.push(t('site.planUnlimitedCredit'))
  }
  if (plan.max_requests_per_period !== null) {
    list.push(t('site.planQuotaRequests', { count: formatNumber(plan.max_requests_per_period) }))
  }
  if (plan.max_credit_per_period !== null) {
    list.push(t('site.planQuotaCredit', { amount: formatMoney(plan.max_credit_per_period) }))
  }
  for (const quota of plan.model_quotas ?? []) {
    list.push(t('site.planModelQuota', { model: quota.model }))
  }
  if (plan.group_name) list.push(t('site.planGroup', { group: plan.group_name }))
  list.push(plan.model_whitelist.length
    ? t('site.planWhitelist', { count: plan.model_whitelist.length })
    : t('site.planAllModels'))
  return list
}
</script>

<template>
  <div v-if="loading" class="grid gap-4 md:grid-cols-3">
    <div v-for="index in 3" :key="index" class="rounded-card border border-line bg-surface p-6">
      <UiSkeleton :rows="5" />
    </div>
  </div>

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

  <div v-else class="grid gap-4 md:grid-cols-3">
    <article
      v-for="(plan, index) in sorted"
      :key="plan.id"
      :class="[
        'relative flex min-w-0 flex-col gap-6 rounded-card border p-6 transition-colors duration-150 ease-out lg:p-8',
        index === featured ? 'border-clay/60 bg-clay-soft/30' : 'border-line bg-surface hover:border-line-strong',
      ]"
    >
      <UiBadge v-if="index === featured" tone="clay" class="absolute -top-2.5 left-6">
        {{ t('site.planRecommended') }}
      </UiBadge>

      <header class="space-y-1">
        <h3 class="text-[15px] font-semibold text-ink">{{ plan.name }}</h3>
        <p v-if="plan.description" class="text-[13px] text-muted">{{ plan.description }}</p>
      </header>

      <p class="flex items-baseline gap-1">
        <span class="display text-4xl text-ink">{{ formatMoney(plan.price) }}</span>
        <span class="text-[13px] text-muted">/ {{ periodLabel(plan) }}</span>
      </p>

      <ul class="flex-1 space-y-3 border-t border-line pt-5">
        <li v-for="item in benefits(plan)" :key="item" class="flex items-start gap-2 text-[13px] text-muted">
          <Check class="mt-0.5 size-3.5 shrink-0 text-clay" />
          {{ item }}
        </li>
      </ul>

      <UiButton
        :to="`/auth?mode=register&plan=${plan.id}`"
        :variant="index === featured ? 'primary' : 'secondary'"
        block
      >{{ t('site.planChoose', { name: plan.name }) }}</UiButton>
    </article>
  </div>
</template>
