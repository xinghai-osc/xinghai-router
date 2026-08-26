<script setup lang="ts">
import { Activity, Coins, KeyRound, Lock, Wallet } from 'lucide-vue-next'
import { endpoints, type DailyUsageRecord } from '~/src/api'
import { formatCompact, formatMoney, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

interface TokenPoint { key: string; label: string; value: number }

const CHART_DAYS = 14
const HEATMAP_WEEKS = 53
const HEATMAP_DAYS = HEATMAP_WEEKS * 7

const { t, locale } = useI18n()
const { account } = useAccount()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.overview')} · ${settings.value.name}` })

const { data: dailyUsage, pending, error } = useResource(
  () => endpoints.getAccountUsageDaily(CHART_DAYS, -new Date().getTimezoneOffset()),
  { data: [] as DailyUsageRecord[] },
)

const { data: heatmapUsage, pending: heatmapPending, error: heatmapError } = useResource(
  () => endpoints.getAccountUsageDaily(400, -new Date().getTimezoneOffset()),
  { data: [] as DailyUsageRecord[] },
)

const { data: summary, pending: summaryPending } = useResource(
  () => endpoints.getAccountUsageSummary(),
  { requests: 0, tokens: 0, cost: '0' },
)

const endpointUrl = `${useRequestURL().origin}/api/v1`

function dayKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const daily = computed<TokenPoint[]>(() => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)

  const buckets = new Map<string, number>()
  const points: TokenPoint[] = []
  for (let offset = CHART_DAYS - 1; offset >= 0; offset -= 1) {
    const day = new Date(today)
    day.setDate(today.getDate() - offset)
    const key = dayKey(day)
    buckets.set(key, 0)
    points.push({ key, label: `${day.getMonth() + 1}/${day.getDate()}`, value: 0 })
  }

  for (const record of dailyUsage.value.data) {
    const current = buckets.get(record.day)
    if (current !== undefined) {
      buckets.set(record.day, current + record.prompt_tokens + record.completion_tokens)
    }
  }

  return points.map(point => ({ ...point, value: buckets.get(point.key) ?? 0 }))
})

const hasUsage = computed(() => daily.value.some(point => point.value > 0))

function heatmapDayKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const heatmap = computed(() => {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const end = new Date(today)
  end.setDate(end.getDate() + (6 - end.getDay()))
  const start = new Date(end)
  start.setDate(start.getDate() - (HEATMAP_DAYS - 1))

  const buckets = new Map<string, number>()
  const points: { key: string; date: string; label: string; requests: number }[] = []
  for (let offset = 0; offset < HEATMAP_DAYS; offset += 1) {
    const date = new Date(start)
    date.setDate(start.getDate() + offset)
    const key = heatmapDayKey(date)
    buckets.set(key, 0)
    points.push({
      key,
      date: key,
      label: new Intl.DateTimeFormat(locale.value === 'en' ? 'en-US' : locale.value === 'zh-Hant' ? 'zh-TW' : 'zh-CN', { year: 'numeric', month: 'short', day: 'numeric' }).format(date),
      requests: 0,
    })
  }

  for (const record of heatmapUsage.value.data) {
    if (buckets.has(record.day)) buckets.set(record.day, record.requests)
  }
  return points.map(point => ({ ...point, requests: buckets.get(point.key) ?? 0 }))
})

const heatmapTotal = computed(() => heatmap.value.reduce((sum, point) => sum + point.requests, 0))
const heatmapActiveDays = computed(() => heatmap.value.filter(point => point.requests > 0).length)

const quickLinks = computed(() => [
  { to: '/console/keys?create=1', icon: KeyRound, title: t('console.quickCreateKey'), hint: t('console.quickCreateKeyHint') },
  { to: '/console/wallet', icon: Wallet, title: t('console.quickTopUp'), hint: t('console.quickTopUpHint') },
  { to: '/console/usage', icon: Activity, title: t('console.quickUsage'), hint: t('console.quickUsageHint') },
])
</script>

<template>
  <div class="space-y-4">
    <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-5">
      <ConsoleUserStatCard
        :label="t('console.balance')"
        :value="formatMoney(account?.balance ?? 0)"
        :hint="t('console.balanceHint')"
        :icon="Wallet"
        :loading="!account"
      />
      <ConsoleUserStatCard
        :label="t('console.reserved')"
        :value="formatMoney(account?.reserved ?? 0)"
        :hint="t('console.reservedHint')"
        :icon="Lock"
        :loading="!account"
      />
      <ConsoleUserStatCard
        :label="t('console.pendingSettlement')"
        :value="formatMoney(account?.pending_settlement ?? 0)"
        :hint="t('console.pendingSettlementHint')"
        :icon="Coins"
        :loading="!account"
      />
      <ConsoleUserStatCard
        :label="t('console.periodRequests')"
        :value="formatNumber(summary.requests)"
        :icon="Activity"
        :loading="summaryPending"
      />
      <ConsoleUserStatCard
        :label="t('console.periodTokens')"
        :value="formatCompact(summary.tokens)"
        :icon="Coins"
        :loading="summaryPending"
      />
      <ConsoleUserStatCard
        :label="t('console.periodSpend')"
        :value="formatMoney(summary.cost)"
        :icon="Coins"
        :loading="summaryPending"
      />
    </div>

    <UiCard :title="t('console.dailyTokens')" :description="t('console.dailyTokensHint')">
      <ConsoleUserDataState
        :pending="pending"
        :error="error"
        :empty="!hasUsage"
        :rows="5"
        :empty-icon="Activity"
        :empty-title="t('console.chartEmptyTitle')"
        :empty-description="t('console.chartEmptyBody')"
      >
        <ConsoleUserTokenChart :points="daily" />
      </ConsoleUserDataState>
    </UiCard>

    <UiCard :title="t('console.callHeatmap')" :description="t('console.callHeatmapHint')">
      <template #actions>
        <div class="flex items-center gap-2 text-xs text-muted">
          <span class="numeric font-semibold text-ink">{{ formatNumber(heatmapTotal) }}</span>
          <span>{{ t('console.heatmapRequestsUnit') }}</span>
        </div>
      </template>
      <div v-if="heatmapPending" class="space-y-2">
        <UiSkeleton :rows="5" />
      </div>
      <UiAlert v-else-if="heatmapError" tone="danger" :title="t('console.heatmapError')" />
      <div v-else class="space-y-3">
        <ConsoleUserCallHeatmap :points="heatmap" />
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
          <span>{{ t('console.heatmapActiveDays', { count: heatmapActiveDays }) }}</span>
          <span>{{ t('console.heatmapTotal', { count: formatNumber(heatmapTotal) }) }}</span>
        </div>
      </div>
    </UiCard>

    <div class="grid gap-4 lg:grid-cols-2">
      <UiCard :title="t('console.apiEndpoint')" :description="t('console.apiEndpointHint')">
        <div class="flex flex-wrap items-center gap-2">
          <code class="min-w-0 flex-1 truncate rounded-control bg-sunken px-3 py-2 font-mono text-[13px] text-ink">
            {{ endpointUrl }}
          </code>
          <ConsoleUserCopyButton :value="endpointUrl" :success-message="t('console.endpointCopied')" />
        </div>
      </UiCard>

      <UiCard :title="t('console.quickActions')" flush>
        <ul class="divide-y divide-line">
          <li v-for="link in quickLinks" :key="link.to">
            <NuxtLink
              :to="link.to"
              class="flex items-center gap-3 px-5 py-3.5 transition-colors duration-150 hover:bg-sunken"
            >
              <span class="flex size-9 shrink-0 items-center justify-center rounded-control bg-clay-soft text-clay">
                <component :is="link.icon" class="size-4" />
              </span>
              <span class="min-w-0">
                <span class="block truncate text-sm font-medium text-ink">{{ link.title }}</span>
                <span class="block truncate text-[13px] text-muted">{{ link.hint }}</span>
              </span>
            </NuxtLink>
          </li>
        </ul>
      </UiCard>
    </div>
  </div>
</template>
