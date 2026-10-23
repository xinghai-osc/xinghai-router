<script setup lang="ts">
import { Activity, ArrowRight, ChartColumn, Coins, KeyRound, Lock, ReceiptText, RefreshCw, Wallet } from 'lucide-vue-next'
import { endpoints, type DailyUsageRecord } from '~/src/api'
import { formatCompact, formatMoney, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

interface TokenPoint { key: string; label: string; value: number }

const CHART_DAYS = 14
const HEATMAP_WEEKS = 53
const HEATMAP_DAYS = HEATMAP_WEEKS * 7

const { t, locale } = useI18n()
const { account, error: accountError, loadAccount } = useAccount()
const { settings } = useSiteSettings()
const mounted = ref(false)
onMounted(() => { mounted.value = true })

const accountName = computed(() => account.value?.name?.trim() || account.value?.email || '')

useHead({ title: () => `${t('console.overviewTitle')} · ${settings.value.name}` })

const { data: dailyUsage, pending, error, refresh: refreshDaily } = useResource(
  () => endpoints.getAccountUsageDaily(CHART_DAYS, -new Date().getTimezoneOffset()),
  { data: [] as DailyUsageRecord[] },
)

const { data: heatmapUsage, pending: heatmapPending, error: heatmapError, refresh: refreshHeatmap } = useResource(
  () => endpoints.getAccountUsageDaily(400, -new Date().getTimezoneOffset()),
  { data: [] as DailyUsageRecord[] },
  { immediate: false },
)
const { target: heatmapTarget, started: heatmapStarted } = useDeferredLoad(() => { void refreshHeatmap() }, { rootMargin: '240px 0px' })

const { data: summary, pending: summaryPending, error: summaryError, refresh: refreshSummary } = useResource(
  () => endpoints.getAccountUsageSummary(),
  { requests: 0, tokens: 0, cost: '0' },
)

const dashboardRefreshing = ref(false)

async function refreshDashboard() {
  if (dashboardRefreshing.value) return
  dashboardRefreshing.value = true
  try {
    await Promise.all([loadAccount(true), refreshDaily(), refreshSummary(), refreshHeatmap()])
  } finally {
    dashboardRefreshing.value = false
  }
}

const endpointUrls = computed(() => {
  const configured = settings.value.api_base_urls ?? []
  if (configured.length) return configured.map(base => `${base.replace(/\/+$/, '')}/api/v1`)
  return [`${useRequestURL().origin}/api/v1`]
})
const endpointUrl = computed(() => endpointUrls.value.join('\n'))

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

const dailyTokensTotal = computed(() => daily.value.reduce((sum, point) => sum + point.value, 0))
const hasUsage = computed(() => dailyTokensTotal.value > 0)

function heatmapDayKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const heatmapDateFormatter = computed(() => new Intl.DateTimeFormat(
  locale.value === 'en' ? 'en-US' : locale.value === 'zh-Hant' ? 'zh-TW' : 'zh-CN',
  { year: 'numeric', month: 'short', day: 'numeric' },
))

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
      label: heatmapDateFormatter.value.format(date),
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
  { to: '/console/keys?create=1', icon: KeyRound, tone: 'bg-clay-soft text-clay', title: t('console.quickCreateKey'), hint: t('console.quickCreateKeyHint') },
  { to: '/console/wallet', icon: Wallet, tone: 'bg-success-soft text-success', title: t('console.quickTopUp'), hint: t('console.quickTopUpHint') },
  { to: '/console/usage', icon: Activity, tone: 'bg-warn-soft text-warn', title: t('console.quickUsage'), hint: t('console.quickUsageHint') },
  { to: '/console/ledger', icon: ReceiptText, tone: 'bg-clay-soft text-clay', title: t('console.quickLedger'), hint: t('console.quickLedgerHint') },
])
</script>

<template>
  <div class="space-y-6 lg:space-y-8">
    <section class="border-b border-line pb-6 lg:pb-8">
      <div class="flex flex-col gap-5 2xl:flex-row 2xl:items-center 2xl:justify-between">
        <div class="min-w-0 max-w-2xl">
          <p class="mb-2 text-xs font-semibold text-clay">{{ t('console.overviewTitle') }}</p>
          <h2 class="text-2xl leading-snug font-semibold tracking-tight text-ink [overflow-wrap:anywhere] sm:text-3xl">
            {{ accountName ? t('console.overviewWelcome', { name: accountName }) : t('console.overviewWelcomeFallback') }}
          </h2>
          <p class="mt-2 text-sm leading-6 text-muted">{{ t('console.overviewDescription') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2 2xl:shrink-0">
          <UiButton variant="ghost" :loading="dashboardRefreshing" :aria-label="t('console.overviewRefresh')" @click="refreshDashboard">
            <RefreshCw class="size-4" aria-hidden="true" />
            <span>{{ t('console.overviewRefresh') }}</span>
          </UiButton>
          <UiButton to="/console/keys?create=1">
            <KeyRound class="size-4" aria-hidden="true" />
            {{ t('console.createKey') }}
          </UiButton>
          <UiButton to="/console/wallet" variant="secondary">
            <Wallet class="size-4" aria-hidden="true" />
            {{ t('console.overviewWallet') }}
          </UiButton>
        </div>
      </div>
    </section>

    <section :aria-label="t('console.overviewStats')" class="space-y-3">
      <UiAlert v-if="accountError" tone="danger" :title="t('console.overviewAccountError')">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="min-w-0 [overflow-wrap:anywhere]">{{ accountError }}</p>
          <UiButton variant="secondary" size="sm" @click="loadAccount(true)">{{ t('console.overviewRetry') }}</UiButton>
        </div>
      </UiAlert>
      <UiAlert v-if="summaryError" tone="danger" :title="t('console.overviewSummaryError')">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="min-w-0 [overflow-wrap:anywhere]">{{ summaryError }}</p>
          <UiButton variant="secondary" size="sm" @click="refreshSummary()">{{ t('console.overviewRetry') }}</UiButton>
        </div>
      </UiAlert>
      <div class="grid gap-6 2xl:grid-cols-2">
        <div class="min-w-0 space-y-3">
          <h2 class="text-sm font-semibold text-ink">{{ t('console.overviewFundsTitle') }}</h2>
          <div class="grid grid-cols-2 gap-3 [&>*:first-child]:col-span-2 sm:grid-cols-3 sm:[&>*:first-child]:col-span-1">
            <ConsoleUserStatCard
              :label="t('console.balance')"
              :value="accountError ? t('console.overviewUnavailable') : formatMoney(account?.balance ?? 0)"
              :hint="t('console.balanceHint')"
              :icon="Wallet"
              :loading="!account && !accountError"
              tone="success"
            />
            <ConsoleUserStatCard
              :label="t('console.reserved')"
              :value="accountError ? t('console.overviewUnavailable') : formatMoney(account?.reserved ?? 0)"
              :hint="t('console.reservedHint')"
              :icon="Lock"
              :loading="!account && !accountError"
              tone="warn"
            />
            <ConsoleUserStatCard
              :label="t('console.pendingSettlement')"
              :value="accountError ? t('console.overviewUnavailable') : formatMoney(account?.pending_settlement ?? 0)"
              :hint="t('console.pendingSettlementHint')"
              :icon="ReceiptText"
              :loading="!account && !accountError"
              tone="warn"
            />
          </div>
        </div>
        <div class="min-w-0 space-y-3">
          <h2 class="text-sm font-semibold text-ink">{{ t('console.overviewUsageTitle') }}</h2>
          <div class="grid grid-cols-2 gap-3 [&>*:first-child]:col-span-2 sm:grid-cols-3 sm:[&>*:first-child]:col-span-1">
            <ConsoleUserStatCard
              :label="t('console.periodRequests')"
              :value="summaryError ? t('console.overviewUnavailable') : formatNumber(summary.requests)"
              :hint="t('console.overviewRequestsHint')"
              :icon="Activity"
              :loading="!mounted || summaryPending"
            />
            <ConsoleUserStatCard
              :label="t('console.periodTokens')"
              :value="summaryError ? t('console.overviewUnavailable') : formatCompact(summary.tokens)"
              :hint="t('console.overviewTokensHint')"
              :icon="ChartColumn"
              :loading="!mounted || summaryPending"
            />
            <ConsoleUserStatCard
              :label="t('console.periodSpend')"
              :value="summaryError ? t('console.overviewUnavailable') : formatMoney(summary.cost)"
              :hint="t('console.overviewSpendHint')"
              :icon="Coins"
              :loading="!mounted || summaryPending"
              tone="success"
            />
          </div>
        </div>
      </div>
    </section>

    <div class="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(18rem,22rem)]">
      <div class="min-w-0 space-y-6">
        <UiCard :title="t('console.dailyTokens')" :description="t('console.dailyTokensHint')">
          <template #actions>
            <span class="rounded-control border border-line bg-sunken px-2.5 py-1 text-xs font-medium text-muted">{{ t('console.overviewChartRange') }}</span>
          </template>
          <div v-if="!mounted || pending" class="min-h-56" :aria-busy="true">
            <UiSkeleton :rows="6" />
          </div>
          <UiAlert v-else-if="error" tone="danger" :title="t('console.overviewDailyError')">
            <p class="[overflow-wrap:anywhere]">{{ error }}</p>
            <UiButton class="mt-3" variant="secondary" size="sm" @click="refreshDaily()">{{ t('console.overviewRetry') }}</UiButton>
          </UiAlert>
          <UiEmptyState v-else-if="!hasUsage" :icon="Activity" :title="t('console.chartEmptyTitle')" :description="t('console.chartEmptyBody')">
            <UiButton to="/console/keys?create=1" variant="secondary" size="sm">{{ t('console.createKey') }}</UiButton>
          </UiEmptyState>
          <div v-else class="space-y-5">
            <div>
              <p class="text-xs text-muted">{{ t('console.overviewChartTotal') }}</p>
              <p class="numeric mt-1 text-2xl font-semibold tracking-tight text-ink [overflow-wrap:anywhere]">{{ formatNumber(dailyTokensTotal) }}</p>
            </div>
            <ConsoleUserTokenChart :points="daily" />
          </div>
          <template #footer>
            <UiButton to="/console/usage" variant="link" size="sm" class="h-auto px-0">
              {{ t('console.quickUsage') }}
              <ArrowRight class="size-3.5" aria-hidden="true" />
            </UiButton>
          </template>
        </UiCard>

        <div ref="heatmapTarget" class="min-w-0">
          <UiCard :title="t('console.callHeatmap')" :description="t('console.callHeatmapHint')">
            <template #actions>
              <span v-if="heatmapStarted && !heatmapPending && !heatmapError" class="numeric max-w-full rounded-control bg-clay-soft px-2.5 py-1 text-xs font-medium text-clay [overflow-wrap:anywhere]">
                {{ t('console.heatmapTotal', { count: formatNumber(heatmapTotal) }) }}
              </span>
            </template>
            <div v-if="!heatmapStarted || heatmapPending" class="min-h-40" :aria-busy="true">
              <UiSkeleton :rows="5" />
            </div>
            <UiAlert v-else-if="heatmapError" tone="danger" :title="t('console.heatmapError')">
              <p class="[overflow-wrap:anywhere]">{{ heatmapError }}</p>
              <UiButton class="mt-3" variant="secondary" size="sm" @click="refreshHeatmap()">{{ t('console.overviewRetry') }}</UiButton>
            </UiAlert>
            <div v-else class="space-y-3">
              <ConsoleUserCallHeatmap :points="heatmap" />
              <p v-if="heatmapTotal === 0" class="rounded-control bg-sunken px-3 py-2 text-xs leading-5 text-muted">{{ t('console.overviewHeatmapEmpty') }}</p>
              <p v-else class="text-xs text-muted">{{ t('console.heatmapActiveDays', { count: heatmapActiveDays }) }}</p>
            </div>
          </UiCard>
        </div>
      </div>
      <div class="min-w-0 space-y-6">
        <UiCard :title="t('console.quickActions')" :description="t('console.overviewQuickActionsHint')" flush>
          <ul class="space-y-1 p-2">
            <li v-for="link in quickLinks" :key="link.to">
              <NuxtLink
                :to="link.to"
                class="group flex items-center gap-3 rounded-control px-3 py-3 transition-colors duration-150 ease-out hover:bg-sunken focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-clay sm:px-4"
              >
                <span class="flex size-9 shrink-0 items-center justify-center rounded-control" :class="link.tone">
                  <component :is="link.icon" class="size-4" aria-hidden="true" />
                </span>
                <span class="min-w-0 flex-1">
                  <span class="block text-sm font-medium text-ink">{{ link.title }}</span>
                  <span class="mt-0.5 block text-xs leading-5 text-muted">{{ link.hint }}</span>
                </span>
                <ArrowRight class="size-4 shrink-0 text-faint transition-transform duration-150 ease-out motion-safe:group-hover:translate-x-1 motion-safe:group-focus-visible:translate-x-1" aria-hidden="true" />
              </NuxtLink>
            </li>
          </ul>
        </UiCard>

        <UiCard :title="t('console.apiEndpoint')" :description="t('console.apiEndpointHint')">
          <div class="space-y-3">
            <p class="text-xs font-medium text-muted">{{ t('console.overviewBaseUrl') }}</p>
            <code class="block min-w-0 whitespace-pre-wrap rounded-control border border-line bg-sunken px-3 py-3 font-mono text-[13px] leading-6 text-ink [overflow-wrap:anywhere]">{{ endpointUrl }}</code>
            <ConsoleUserCopyButton :value="endpointUrl" :label="t('console.overviewCopyEndpoint')" :success-message="t('console.endpointCopied')" />
            <ConsoleMiniMaxApiHelp :base-url="endpointUrls[0]!" />
            <ConsoleMediaApiHelp :base-url="endpointUrls[0]!" />
          </div>
          <template #footer>
            <p class="text-xs leading-5 text-muted">{{ t('console.overviewEndpointNote') }}</p>
          </template>
        </UiCard>
      </div>
    </div>
  </div>
</template>
