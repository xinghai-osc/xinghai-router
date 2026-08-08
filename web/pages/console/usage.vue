<script setup lang="ts">
import { CalendarDays, ChevronDown, KeyRound, Search } from 'lucide-vue-next'
import {
  PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger,
} from 'reka-ui'
import { endpoints, type UsageLog, type UsageRecord, type UsageStats } from '~/src/api'
import { formatCompact, formatDateTime, formatMoney, formatNumber, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { isAdmin } = useAccount()

useHead({ title: () => `${t('nav.usage')} · ${settings.value.name}` })

const adminView = ref(false)

const { data: usage, pending, error } = useResource(
  () => endpoints.getAccountUsage(),
  { data: [] as UsageRecord[] },
)

const statusOptions = computed(() => {
  if (adminView.value) {
    return [
      { value: '', label: t('console.statusAll') },
      { value: 'success', label: t('console.statusSuccess') },
      { value: 'error', label: t('admin.statusError') },
    ]
  }
  return [
    { value: '', label: t('console.statusAll') },
    { value: 'success', label: t('console.statusSuccess') },
    { value: 'failed', label: t('console.statusFailed') },
    { value: 'settled', label: t('console.statusSettled') },
  ]
})

const model = ref('')
const group = ref('')
const key = ref('')
const requestId = ref('')
const status = ref('')
const advancedOpen = ref(false)

const DATE_PRESETS = [
  { labelKey: 'console.datePresetToday', start: (now: Date) => { const s = new Date(now); s.setHours(0, 0, 0, 0); return s } },
  { labelKey: 'console.datePreset24h', start: (now: Date) => new Date(now.getTime() - 24 * 3600 * 1000) },
  { labelKey: 'console.datePreset7d', start: (now: Date) => new Date(now.getTime() - 7 * 24 * 3600 * 1000) },
  { labelKey: 'console.datePreset14d', start: (now: Date) => new Date(now.getTime() - 14 * 24 * 3600 * 1000) },
  { labelKey: 'console.datePreset30d', start: (now: Date) => new Date(now.getTime() - 30 * 24 * 3600 * 1000) },
]

function startOfToday(): Date {
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  return start
}

const range = reactive<{ start: Date | null; end: Date | null }>({ start: startOfToday(), end: new Date() })
const rangeDraft = reactive({ start: '', end: '' })
const rangeOpen = ref(false)

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

function toInputValue(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function compactDate(date: Date): string {
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const rangeLabel = computed(() => {
  if (!range.start && !range.end) return t('console.dateRange')
  if (range.start && range.end) return `${compactDate(range.start)} ~ ${compactDate(range.end)}`
  return `${range.start ? compactDate(range.start) : '-'} ~ ${range.end ? compactDate(range.end) : '-'}`
})

watch(rangeOpen, (open) => {
  if (open) {
    rangeDraft.start = range.start ? toInputValue(range.start) : ''
    rangeDraft.end = range.end ? toInputValue(range.end) : ''
  }
})

function applyRangeDraft() {
  range.start = rangeDraft.start ? new Date(rangeDraft.start) : null
  range.end = rangeDraft.end ? new Date(rangeDraft.end) : null
  rangeOpen.value = false
}

function applyRangePreset(preset: (typeof DATE_PRESETS)[number]) {
  const end = new Date()
  const start = preset.start(end)
  range.start = start
  range.end = end
  rangeDraft.start = toInputValue(start)
  rangeDraft.end = toInputValue(end)
  rangeOpen.value = false
}

const advancedActiveCount = computed(() => [key.value.trim(), requestId.value.trim()].filter(Boolean).length)

const filtered = computed(() => {
  const keyword = (value: string) => value.trim().toLowerCase()
  const needleModel = keyword(model.value)
  const needleGroup = keyword(group.value)
  const needleKey = keyword(key.value)
  const needleRequestId = keyword(requestId.value)
  return usage.value.data.filter((record) => {
    if (status.value && record.status !== status.value) return false
    if (needleModel && !record.model.toLowerCase().includes(needleModel)) return false
    if (needleGroup && !(record.group_name || '').toLowerCase().includes(needleGroup)) return false
    if (needleKey && !(record.key_name || '').toLowerCase().includes(needleKey)) return false
    if (needleRequestId && !record.request_id.toLowerCase().includes(needleRequestId)) return false
    const created = new Date(record.created_at)
    if (range.start && created < range.start) return false
    if (range.end && created > range.end) return false
    return true
  })
})

const stats = computed(() => filtered.value.reduce(
  (sum, record) => ({
    cost: sum.cost + Number(record.cost ?? 0),
    requests: sum.requests + 1,
    tokens: sum.tokens + record.prompt_tokens + record.completion_tokens,
  }),
  { cost: 0, requests: 0, tokens: 0 },
))

const userId = ref('')
const page = ref(1)
const pageSize = ref('50')
const pageSizeOptions = ['20', '50', '100', '200']

function toRfc3339(value: Date | null): string {
  if (!value || Number.isNaN(value.getTime())) return ''
  return value.toISOString()
}

function adminLogsQuery(): string {
  const params = new URLSearchParams()
  if (userId.value.trim()) params.set('user_id', userId.value.trim())
  if (model.value.trim()) params.set('model', model.value.trim())
  if (status.value) params.set('status', status.value)
  if (requestId.value.trim()) params.set('request_id', requestId.value.trim())
  const start = toRfc3339(range.start)
  if (start) params.set('start', start)
  const end = toRfc3339(range.end)
  if (end) params.set('end', end)
  params.set('page', String(page.value))
  params.set('page_size', pageSize.value)
  return `?${params.toString()}`
}

function adminStatsQuery(): string {
  const params = new URLSearchParams()
  if (userId.value.trim()) params.set('user_id', userId.value.trim())
  if (model.value.trim()) params.set('model', model.value.trim())
  const start = toRfc3339(range.start)
  if (start) params.set('start', start)
  const end = toRfc3339(range.end)
  if (end) params.set('end', end)
  const query = params.toString()
  return query ? `?${query}` : ''
}

const EMPTY_STATS: UsageStats = {
  total_requests: 0,
  success_count: 0,
  error_count: 0,
  prompt_tokens: 0,
  cached_prompt_tokens: 0,
  completion_tokens: 0,
  total_tokens: 0,
  total_cost: '0',
  avg_duration_ms: 0,
}

const adminUsage = useResource(
  () => endpoints.getUsageLogs(adminLogsQuery()),
  { data: [] as UsageLog[], total: 0, page: 1, page_size: 50 },
)
const adminStats = useResource(() => endpoints.getUsageStats(adminStatsQuery()), EMPTY_STATS)

const totalPages = computed(() => Math.max(1, Math.ceil(adminUsage.data.value.total / Math.max(1, adminUsage.data.value.page_size))))

type UsageRow = UsageRecord & { user_id?: string | null; user_name?: string }

function normalizeRow(log: UsageLog): UsageRow {
  return {
    request_id: log.request_id,
    model: log.model,
    prompt_tokens: log.prompt_tokens ?? 0,
    cached_prompt_tokens: log.cached_prompt_tokens,
    completion_tokens: log.completion_tokens ?? 0,
    cost: log.cost,
    status: log.status_code >= 400 ? 'failed' : 'success',
    created_at: log.created_at,
    client_ip: log.client_ip,
    user_agent: log.user_agent,
    error: log.error_detail,
    key_name: log.key_name,
    subscription: false,
    duration_ms: log.duration_ms,
    group_name: log.group_name,
    user_name: log.user_name,
    user_id: log.user_id,
  }
}

const rows = computed<UsageRow[]>(() => (adminView.value ? adminUsage.data.value.data.map(normalizeRow) : filtered.value))

const displayStats = computed(() => {
  if (adminView.value) {
    const admin = adminStats.data.value
    return {
      cost: Number(admin.total_cost ?? 0),
      requests: admin.total_requests,
      tokens: admin.total_tokens,
    }
  }
  return stats.value
})

const filtersActive = computed(() => Boolean(
  model.value.trim() || group.value.trim() || key.value.trim()
  || requestId.value.trim() || status.value || range.start || range.end || userId.value.trim(),
))

async function applyAdminFilters() {
  page.value = 1
  await Promise.all([adminUsage.refresh(), adminStats.refresh()])
}

async function resetFilters() {
  model.value = ''
  group.value = ''
  key.value = ''
  requestId.value = ''
  status.value = ''
  userId.value = ''
  range.start = startOfToday()
  range.end = new Date()
  if (adminView.value) await applyAdminFilters()
}

async function setAdminView(on: boolean) {
  if (adminView.value === on) return
  adminView.value = on
  status.value = ''
  page.value = 1
  if (on) await applyAdminFilters()
}

async function goToPage(next: number) {
  if (next < 1 || next > totalPages.value) return
  page.value = next
  await adminUsage.refresh()
}

watch(pageSize, () => {
  if (adminView.value) void applyAdminFilters()
})

function isFailed(record: UsageRow): boolean {
  return record.status === 'failed'
}

const clientTarget = ref<UsageRow | null>(null)
</script>

<template>
  <UiCard :title="t('nav.usage')" :description="t('console.usageDescription')" flush>
    <div class="space-y-4 px-5 py-4">
      <div class="rounded-control border border-line bg-sunken/40 p-3">
        <div class="flex flex-wrap items-end gap-2">
          <div class="w-full sm:w-64">
            <PopoverRoot v-model:open="rangeOpen">
              <PopoverTrigger as-child class="block w-full">
                <button
                  type="button"
                  class="inline-flex h-10 w-full items-center gap-2 rounded-control border border-line-strong bg-surface px-3 text-sm transition-colors duration-150 hover:border-faint focus:border-clay focus:outline-none focus:ring-2 focus:ring-clay/20"
                  :aria-label="t('console.dateRange')"
                >
                  <CalendarDays class="size-4 shrink-0 text-faint" />
                  <span class="truncate tabular-nums" :class="range.start || range.end ? 'text-ink' : 'text-faint'">{{ rangeLabel }}</span>
                </button>
              </PopoverTrigger>
              <PopoverPortal>
                <PopoverContent
                  :side-offset="6"
                  class="animate-pop z-50 w-[min(520px,calc(100vw-2rem))] rounded-control border border-line bg-surface p-3 shadow-pop"
                >
                  <div class="space-y-3">
                    <div class="grid gap-2 sm:grid-cols-[1fr_auto_1fr] sm:items-end">
                      <div class="space-y-1.5">
                        <div class="text-xs text-muted">{{ t('console.filterStart') }}</div>
                        <UiInput v-model="rangeDraft.start" type="datetime-local" class="tabular-nums" />
                      </div>
                      <span class="hidden pb-2 text-xs text-muted sm:block">~</span>
                      <div class="space-y-1.5">
                        <div class="text-xs text-muted">{{ t('console.filterEnd') }}</div>
                        <UiInput v-model="rangeDraft.end" type="datetime-local" class="tabular-nums" />
                      </div>
                    </div>
                    <div class="flex flex-wrap gap-1.5">
                      <UiButton
                        v-for="preset in DATE_PRESETS"
                        :key="preset.labelKey"
                        variant="secondary"
                        size="sm"
                        class="h-7 flex-1 px-2 text-xs"
                        @click="applyRangePreset(preset)"
                      >
                        {{ t(preset.labelKey) }}
                      </UiButton>
                    </div>
                    <div class="flex justify-end">
                      <UiButton size="sm" @click="applyRangeDraft">{{ t('common.confirm') }}</UiButton>
                    </div>
                  </div>
                </PopoverContent>
              </PopoverPortal>
            </PopoverRoot>
          </div>

          <div class="w-full sm:w-48">
            <UiInput v-model="model" :placeholder="t('console.model')" />
          </div>
          <div class="w-full sm:w-44">
            <UiInput v-if="!adminView" v-model="group" :placeholder="t('console.group')" />
            <UiInput v-else v-model="userId" mono :placeholder="t('admin.filterUserId')" />
          </div>
          <div class="w-full sm:w-40">
            <UiSelect v-model="status" :options="statusOptions" :placeholder="t('console.statusAll')" />
          </div>
          <UiButton variant="ghost" size="sm" class="sm:mb-1" @click="advancedOpen = !advancedOpen">
            {{ t('console.moreFilters') }}
            <span v-if="advancedActiveCount" class="inline-flex size-5 items-center justify-center rounded-full bg-clay-soft text-[10px] text-clay">{{ advancedActiveCount }}</span>
            <ChevronDown
              class="size-3.5 transition-transform duration-200"
              :class="advancedOpen && 'rotate-180'"
            />
          </UiButton>

          <div v-if="isAdmin" class="ml-auto flex items-center gap-0.5 self-center rounded-control border border-line bg-surface p-0.5 sm:mb-1" role="group" :aria-label="t('console.viewMode')">
            <button
              type="button"
              class="rounded-[7px] px-3 py-1.5 text-[13px] transition-colors duration-150"
              :class="adminView ? 'text-muted hover:text-ink' : 'bg-clay text-clay-ink'"
              @click="setAdminView(false)"
            >
              {{ t('console.viewMine') }}
            </button>
            <button
              type="button"
              class="rounded-[7px] px-3 py-1.5 text-[13px] transition-colors duration-150"
              :class="adminView ? 'bg-clay text-clay-ink' : 'text-muted hover:text-ink'"
              @click="setAdminView(true)"
            >
              {{ t('console.viewAdmin') }}
            </button>
          </div>
        </div>

        <div v-if="advancedOpen" class="mt-2 flex flex-wrap items-end gap-2 border-t border-line pt-3">
          <div v-if="!adminView" class="w-full sm:w-52">
            <UiInput v-model="key" :placeholder="t('console.keyUsed')" />
          </div>
          <div class="w-full sm:w-72">
            <UiInput v-model="requestId" mono :placeholder="t('console.requestId')" />
          </div>
        </div>

        <div class="mt-3 flex flex-wrap items-center gap-2 border-t border-line pt-3">
          <div class="flex flex-wrap items-center gap-2">
            <span class="inline-flex h-7 items-center gap-2 rounded-md border border-line bg-surface px-2.5 text-xs">
              <span class="h-3.5 w-0.5 rounded-full bg-clay" />
              <span class="text-muted">{{ t('console.cost') }}</span>
              <span class="numeric font-semibold text-ink">{{ formatMoney(displayStats.cost, 4) }}</span>
            </span>
            <span class="inline-flex h-7 items-center gap-2 rounded-md border border-line bg-surface px-2.5 text-xs">
              <span class="h-3.5 w-0.5 rounded-full bg-success" />
              <span class="text-muted">{{ t('console.requests') }}</span>
              <span class="numeric font-semibold text-ink">{{ formatNumber(displayStats.requests) }}</span>
            </span>
            <span class="inline-flex h-7 items-center gap-2 rounded-md border border-line bg-surface px-2.5 text-xs">
              <span class="h-3.5 w-0.5 rounded-full bg-warn" />
              <span class="text-muted">{{ t('console.tokens') }}</span>
              <span class="numeric font-semibold text-ink">{{ formatCompact(displayStats.tokens) }}</span>
            </span>
            <span v-if="adminView" class="inline-flex h-7 items-center gap-2 rounded-md border border-line bg-surface px-2.5 text-xs">
              <span class="h-3.5 w-0.5 rounded-full bg-success" />
              <span class="text-muted">{{ t('admin.statSuccess') }}</span>
              <span class="numeric font-semibold text-ink">{{ formatNumber(adminStats.data.value.success_count) }}</span>
            </span>
            <span v-if="adminView" class="inline-flex h-7 items-center gap-2 rounded-md border border-line bg-surface px-2.5 text-xs">
              <span class="h-3.5 w-0.5 rounded-full bg-danger" />
              <span class="text-muted">{{ t('admin.statErrors') }}</span>
              <span class="numeric font-semibold text-ink">{{ formatNumber(adminStats.data.value.error_count) }}</span>
            </span>
          </div>
          <div class="ml-auto">
            <UiButton v-if="filtersActive" variant="ghost" size="sm" @click="resetFilters">
              {{ t('common.reset') }}
            </UiButton>
            <UiButton v-if="adminView" size="sm" class="ml-2" @click="applyAdminFilters">
              {{ t('common.filter') }}
            </UiButton>
          </div>
        </div>
      </div>

      <ConsoleUserDataState
        :pending="adminView ? adminUsage.pending.value : pending"
        :error="adminView ? adminUsage.error.value : error"
        :empty="!rows.length"
        :rows="6"
        :empty-icon="Search"
        :empty-title="adminView ? t('admin.usageEmptyTitle') : (filtersActive ? t('console.noMatchTitle') : t('console.usageEmptyTitle'))"
        :empty-description="adminView ? t('admin.usageEmptyBody') : (filtersActive ? t('console.noMatchBody') : t('console.usageEmptyBody'))"
      >
        <UiTable dense>
          <thead>
            <tr>
              <th>{{ t('console.time') }}</th>
              <th v-if="adminView">{{ t('admin.user') }}</th>
              <th>{{ t('console.keyUsed') }}</th>
              <th>{{ t('console.model') }}</th>
              <th class="num">{{ t('console.promptTokens') }} / {{ t('console.completionTokens') }}</th>
              <th class="num">{{ t('console.cost') }}</th>
              <th class="num">{{ t('console.duration') }}</th>
              <th>{{ t('common.detail') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="record in rows"
              :key="record.request_id"
              :class="isFailed(record) && 'bg-danger-soft/40'"
            >
              <td class="whitespace-nowrap">
                <div class="flex min-w-0 flex-col gap-1">
                  <span class="font-mono text-xs tabular-nums text-ink">{{ formatDateTime(record.created_at) }}</span>
                  <ConsoleUserStatusBadge :status="record.status" />
                </div>
              </td>
              <td v-if="adminView" class="text-[13px] text-muted">{{ record.user_name || (record.user_id ? shortId(record.user_id) : '-') }}</td>
              <td>
                <div class="flex max-w-56 flex-col gap-1">
                  <span class="inline-flex w-fit max-w-full items-center gap-1.5 overflow-hidden rounded-md border border-line bg-sunken px-2 py-0.5 text-[13px] text-ink">
                    <KeyRound class="size-3 shrink-0 text-faint" />
                    <span class="truncate">{{ record.key_name || '-' }}</span>
                  </span>
                  <span v-if="record.group_name" class="text-2xs leading-none text-faint">{{ record.group_name }}</span>
                </div>
              </td>
              <td class="font-medium">{{ record.model }}</td>
              <td class="num">
                <div class="flex flex-col items-end gap-0.5">
                  <span class="font-mono text-xs font-medium tabular-nums">
                    {{ formatNumber(record.prompt_tokens) }} / {{ formatNumber(record.completion_tokens) }}
                  </span>
                  <span class="text-2xs text-faint">
                    {{ t('console.cacheReadTokens') }} ↓ {{ formatNumber(record.cached_prompt_tokens) }}
                  </span>
                </div>
              </td>
              <td class="num">
                <UiBadge v-if="record.subscription" tone="success" dot>{{ t('console.subscriptionCovered') }}</UiBadge>
                <span v-else class="inline-flex h-6 items-center rounded-md border border-line-strong bg-sunken px-2 font-mono text-xs font-semibold tabular-nums">
                  {{ formatMoney(record.cost, 4) }}
                </span>
              </td>
              <td class="num text-muted">{{ t('console.durationMs', { value: record.duration_ms }) }}</td>
              <td>
                <button
                  type="button"
                  class="group flex max-w-48 items-center gap-1 text-left text-[13px] text-ink"
                  :title="t('common.detail')"
                  @click="clientTarget = record"
                >
                  <template v-if="isFailed(record) && record.error">
                    <span class="truncate text-danger group-hover:underline">{{ record.error }}</span>
                  </template>
                  <span v-else class="truncate font-mono text-muted group-hover:underline">{{ shortId(record.request_id) }}</span>
                </button>
              </td>
            </tr>
          </tbody>
        </UiTable>

        <ConsoleOpsPagination
          v-if="adminView"
          :page="adminUsage.data.value.page"
          :page-size="pageSize"
          :total="adminUsage.data.value.total"
          :page-size-options="pageSizeOptions"
          @update:page="goToPage"
          @update:page-size="pageSize = $event"
        />
      </ConsoleUserDataState>
    </div>

    <UiDialog v-model:open="clientTarget" :title="t('console.requestDetail')">
      <div class="space-y-4 text-sm">
        <div class="space-y-1.5">
          <div v-if="adminView">
            <div class="mb-1 text-xs text-muted">{{ t('admin.user') }}</div>
            <div class="break-all text-[13px] text-ink">{{ clientTarget?.user_name || (clientTarget?.user_id ? shortId(clientTarget.user_id) : '-') }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.requestId') }}</div>
            <div class="break-all font-mono text-[13px] text-ink">{{ clientTarget?.request_id || '-' }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.keyUsed') }}</div>
            <div class="break-all text-[13px] text-ink">{{ clientTarget?.key_name || '-' }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.group') }}</div>
            <div class="text-[13px] text-ink">{{ clientTarget?.group_name || '-' }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.time') }}</div>
            <div class="font-mono text-[13px] text-ink">{{ clientTarget ? formatDateTime(clientTarget.created_at) : '-' }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.duration') }}</div>
            <div class="numeric text-[13px] text-ink">{{ clientTarget ? t('console.durationMs', { value: clientTarget.duration_ms }) : '-' }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.cost') }}</div>
            <div class="text-[13px] text-ink">
              <UiBadge v-if="clientTarget?.subscription" tone="success" dot>{{ t('console.subscriptionCovered') }}</UiBadge>
              <template v-else>{{ clientTarget ? formatMoney(clientTarget.cost, 4) : '-' }}</template>
            </div>
          </div>
        </div>

        <div class="rounded-control border border-line bg-sunken/40 p-3">
          <div class="mb-2 text-xs font-medium text-muted">{{ t('console.tokenBreakdown') }}</div>
          <div class="grid grid-cols-2 gap-x-4 gap-y-2">
            <div>
              <div class="text-2xs text-faint">{{ t('console.promptTokens') }}</div>
              <div class="numeric text-[13px] text-ink">{{ clientTarget ? formatNumber(clientTarget.prompt_tokens) : '-' }}</div>
            </div>
            <div>
              <div class="text-2xs text-faint">{{ t('console.cachedTokens') }}</div>
              <div class="numeric text-[13px] text-ink">{{ clientTarget ? formatNumber(clientTarget.cached_prompt_tokens) : '-' }}</div>
            </div>
            <div>
              <div class="text-2xs text-faint">{{ t('console.completionTokens') }}</div>
              <div class="numeric text-[13px] text-ink">{{ clientTarget ? formatNumber(clientTarget.completion_tokens) : '-' }}</div>
            </div>
            <div>
              <div class="text-2xs text-faint">{{ t('console.totalTokens') }}</div>
              <div class="numeric text-[13px] text-ink">{{ clientTarget ? formatNumber(clientTarget.prompt_tokens + clientTarget.completion_tokens) : '-' }}</div>
            </div>
          </div>
        </div>

        <div v-if="clientTarget?.status === 'failed' && clientTarget.error" class="rounded-control border border-danger/25 bg-danger-soft p-3">
          <div class="mb-1 text-xs font-medium text-danger">{{ t('console.errorDetail') }}</div>
          <div class="whitespace-pre-wrap break-all font-mono text-[13px] text-danger">{{ clientTarget.error }}</div>
        </div>

        <div class="space-y-1.5">
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.clientIp') }}</div>
            <div class="break-all font-mono text-[13px] text-ink">{{ clientTarget?.client_ip || '-' }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('console.userAgent') }}</div>
            <div class="break-all font-mono text-[13px] text-ink">{{ clientTarget?.user_agent || '-' }}</div>
          </div>
        </div>
      </div>
    </UiDialog>
  </UiCard>
</template>