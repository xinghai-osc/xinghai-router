<script setup lang="ts">
import { CalendarDays, ChevronDown, ScrollText, Search } from 'lucide-vue-next'
import {
  PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger,
} from 'reka-ui'
import { endpoints, type RequestLog, type UsageLog, type UsageStats } from '~/src/api'
import { formatCompact, formatDateTime, formatMoney, formatNumber, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()

const allowed = computed(() => can('logs.read'))

const tab = ref('usage')
const tabs = computed(() => [
  { value: 'usage', label: t('admin.tabUsage') },
  { value: 'requests', label: t('admin.tabRequests') },
])

const filters = reactive({
  user_id: '',
  model: '',
  channel_id: '',
  group_id: '',
  status: '',
  request_id: '',
})

const advancedOpen = ref(false)
const advancedActiveCount = computed(() => [filters.channel_id.trim(), filters.group_id.trim(), filters.request_id.trim()].filter(Boolean).length)

const DATE_PRESETS = [
  { labelKey: 'admin.datePresetToday', start: (now: Date) => { const s = new Date(now); s.setHours(0, 0, 0, 0); return s } },
  { labelKey: 'admin.datePreset24h', start: (now: Date) => new Date(now.getTime() - 24 * 3600 * 1000) },
  { labelKey: 'admin.datePreset7d', start: (now: Date) => new Date(now.getTime() - 7 * 24 * 3600 * 1000) },
  { labelKey: 'admin.datePreset14d', start: (now: Date) => new Date(now.getTime() - 14 * 24 * 3600 * 1000) },
  { labelKey: 'admin.datePreset30d', start: (now: Date) => new Date(now.getTime() - 30 * 24 * 3600 * 1000) },
]

function startOfToday(): Date {
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  return start
}

function defaultEnd(): Date {
  return new Date(Date.now() + 3600 * 1000)
}

const range = reactive<{ start: Date | null; end: Date | null }>({ start: startOfToday(), end: defaultEnd() })
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
  if (!range.start && !range.end) return t('admin.dateRange')
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
  const now = new Date()
  const end = defaultEnd()
  const start = preset.start(now)
  range.start = start
  range.end = end
  rangeDraft.start = toInputValue(start)
  rangeDraft.end = toInputValue(end)
  rangeOpen.value = false
}

const filtersActive = computed(() => Boolean(
  filters.user_id.trim() || filters.model.trim() || filters.channel_id.trim()
  || filters.group_id.trim() || filters.status || filters.request_id.trim()
  || range.start || range.end,
))

const page = ref(1)
const pageSize = ref('50')

const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'success', label: t('admin.statusSuccess') },
  { value: 'error', label: t('admin.statusError') },
])
const pageSizeOptions = ['20', '50', '100', '200'].map(value => ({ value, label: value }))

/** `datetime-local` yields a local wall-clock string; the API wants RFC 3339. */
function toRfc3339(value: Date | null): string {
  if (!value || Number.isNaN(value.getTime())) return ''
  return value.toISOString()
}

function filterParams(): URLSearchParams {
  const params = new URLSearchParams()
  if (filters.user_id.trim()) params.set('user_id', filters.user_id.trim())
  if (filters.model.trim()) params.set('model', filters.model.trim())
  if (filters.channel_id.trim()) params.set('channel_id', filters.channel_id.trim())
  if (filters.group_id.trim()) params.set('group_id', filters.group_id.trim())
  if (filters.status) params.set('status', filters.status)
  if (filters.request_id.trim()) params.set('request_id', filters.request_id.trim())
  const start = toRfc3339(range.start)
  if (start) params.set('start', start)
  const end = toRfc3339(range.end)
  if (end) params.set('end', end)
  return params
}

function logsQuery(): string {
  const params = filterParams()
  params.set('page', String(page.value))
  params.set('page_size', pageSize.value)
  return `?${params.toString()}`
}

function statsQuery(): string {
  const params = filterParams()
  // usage-stats has no channel/group/status filters; drop what it would ignore.
  params.delete('channel_id')
  params.delete('group_id')
  params.delete('status')
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
  total_cost: 0,
  avg_duration_ms: 0,
}

const usage = useResource(() => endpoints.getUsageLogs(logsQuery()), {
  data: [] as UsageLog[], total: 0, page: 1, page_size: 50,
})
const stats = useResource(() => endpoints.getUsageStats(statsQuery()), EMPTY_STATS)
const requests = useResource(() => endpoints.getRequestLogs(), { data: [] as RequestLog[] })

const totalPages = computed(() => Math.max(1, Math.ceil(usage.data.value.total / Math.max(1, usage.data.value.page_size))))

const statTiles = computed(() => [
  { key: 'statRequests', value: formatNumber(stats.data.value.total_requests) },
  { key: 'statSuccess', value: formatNumber(stats.data.value.success_count) },
  { key: 'statErrors', value: formatNumber(stats.data.value.error_count) },
  { key: 'statPromptTokens', value: formatCompact(stats.data.value.prompt_tokens) },
  { key: 'statCachedTokens', value: formatCompact(stats.data.value.cached_prompt_tokens) },
  { key: 'statCompletionTokens', value: formatCompact(stats.data.value.completion_tokens) },
  { key: 'statTotalTokens', value: formatCompact(stats.data.value.total_tokens) },
  { key: 'statCost', value: formatMoney(stats.data.value.total_cost, 4) },
  { key: 'statAvgDuration', value: t('admin.durationMs', { value: Math.round(stats.data.value.avg_duration_ms) }) },
])

async function applyFilters() {
  page.value = 1
  await Promise.all([usage.refresh(), stats.refresh()])
}

async function resetFilters() {
  filters.user_id = ''
  filters.model = ''
  filters.channel_id = ''
  filters.group_id = ''
  filters.status = ''
  filters.request_id = ''
  range.start = startOfToday()
  range.end = defaultEnd()
  await applyFilters()
}

async function goToPage(next: number) {
  if (next < 1 || next > totalPages.value) return
  page.value = next
  await usage.refresh()
}

watch(pageSize, applyFilters)

const statusTone = (code: number): 'danger' | 'warn' | 'success' =>
  (code >= 400 ? 'danger' : code >= 300 ? 'warn' : 'success')

const detailTarget = ref<UsageLog | RequestLog | null>(null)
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <UiTabs v-model="tab" :items="tabs">
      <div v-if="tab === 'usage'" class="space-y-4 pt-4">
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="tile in statTiles" :key="tile.key" class="rounded-card border border-line bg-surface px-4 py-3">
            <p class="text-2xs text-muted">{{ t(`admin.${tile.key}`) }}</p>
            <UiSkeleton v-if="stats.pending.value" class="mt-2 h-5 w-20" />
            <p v-else class="numeric mt-1 text-lg text-ink">{{ tile.value }}</p>
          </div>
        </div>

        <div class="rounded-control border border-line bg-sunken/40 p-3">
          <div class="flex flex-wrap items-end gap-2">
            <div class="w-full sm:w-64">
              <PopoverRoot v-model:open="rangeOpen">
                <PopoverTrigger as-child class="block w-full">
                  <button
                    type="button"
                    class="inline-flex h-10 w-full items-center gap-2 rounded-control border border-line-strong bg-surface px-3 text-sm transition-colors duration-150 hover:border-faint focus:border-clay focus:outline-none focus:ring-2 focus:ring-clay/20"
                    :aria-label="t('admin.dateRange')"
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
                          <div class="text-xs text-muted">{{ t('admin.filterStart') }}</div>
                          <UiInput v-model="rangeDraft.start" type="datetime-local" class="tabular-nums" />
                        </div>
                        <span class="hidden pb-2 text-xs text-muted sm:block">~</span>
                        <div class="space-y-1.5">
                          <div class="text-xs text-muted">{{ t('admin.filterEnd') }}</div>
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
              <UiInput v-model="filters.model" :placeholder="t('admin.filterModel')" />
            </div>
            <div class="w-full sm:w-48">
              <UiInput v-model="filters.user_id" mono :placeholder="t('admin.filterUserId')" />
            </div>
            <div class="w-full sm:w-40">
              <UiSelect v-model="filters.status" :options="statusOptions" :placeholder="t('common.all')" />
            </div>
            <UiButton variant="ghost" size="sm" class="sm:mb-1" @click="advancedOpen = !advancedOpen">
              {{ t('admin.moreFilters') }}
              <span v-if="advancedActiveCount" class="inline-flex size-5 items-center justify-center rounded-full bg-clay-soft text-[10px] text-clay">{{ advancedActiveCount }}</span>
              <ChevronDown
                class="size-3.5 transition-transform duration-200"
                :class="advancedOpen && 'rotate-180'"
              />
            </UiButton>
            <UiButton size="sm" class="sm:mb-1" @click="applyFilters">
              <Search class="size-4" />
              {{ t('common.search') }}
            </UiButton>
          </div>

          <div v-if="advancedOpen" class="mt-2 flex flex-wrap items-end gap-2 border-t border-line pt-3">
            <div class="w-full sm:w-48">
              <UiInput v-model="filters.channel_id" mono :placeholder="t('admin.filterChannelId')" />
            </div>
            <div class="w-full sm:w-48">
              <UiInput v-model="filters.group_id" mono :placeholder="t('admin.filterGroupId')" />
            </div>
            <div class="w-full sm:w-72">
              <UiInput v-model="filters.request_id" mono :placeholder="t('admin.requestId')" />
            </div>
          </div>

          <div class="mt-3 flex flex-wrap items-center gap-2 border-t border-line pt-3">
            <div class="ml-auto flex flex-wrap items-center gap-3">
              <div class="flex items-center gap-2">
                <span class="text-xs text-muted">{{ t('admin.pageSize') }}</span>
                <div class="w-24">
                  <UiSelect v-model="pageSize" :options="pageSizeOptions" size="sm" />
                </div>
              </div>
              <UiButton v-if="filtersActive" variant="ghost" size="sm" @click="resetFilters">
                {{ t('common.reset') }}
              </UiButton>
            </div>
          </div>
        </div>

        <ConsoleOpsListState
          :pending="usage.pending.value"
          :error="usage.error.value"
          :empty="!usage.data.value.data.length"
          :empty-icon="ScrollText"
          :empty-title="t('admin.usageEmptyTitle')"
          :empty-description="t('admin.usageEmptyBody')"
        >
          <UiTable dense>
            <thead>
              <tr>
                <th>{{ t('admin.time') }}</th>
                <th>{{ t('admin.requestId') }}</th>
                <th>{{ t('admin.user') }}</th>
                <th>{{ t('admin.keyName') }}</th>
                <th>{{ t('admin.model') }}</th>
                <th>{{ t('admin.channel') }}</th>
                <th>{{ t('admin.channelKey') }}</th>
                <th>{{ t('admin.groups') }}</th>
                <th>{{ t('admin.statusCode') }}</th>
                <th class="num">{{ t('admin.statPromptTokens') }}</th>
                <th class="num">{{ t('admin.statCachedTokens') }}</th>
                <th class="num">{{ t('admin.statCompletionTokens') }}</th>
                <th class="num">{{ t('admin.duration') }}</th>
                <th class="num">{{ t('admin.cost') }}</th>
                <th>{{ t('common.detail') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="log in usage.data.value.data" :key="log.request_id">
                <td class="text-muted whitespace-nowrap">{{ formatDateTime(log.created_at) }}</td>
                <td class="font-mono text-[13px] text-muted">{{ shortId(log.request_id) }}</td>
                <td class="text-muted">{{ log.user_name || shortId(log.user_id) }}</td>
                <td class="text-muted">{{ log.key_name || '—' }}</td>
                <td class="font-medium text-ink">{{ log.model }}</td>
                <td class="text-muted">{{ log.channel_name || '—' }}</td>
                <td class="text-muted">{{ log.channel_key_name || '—' }}</td>
                <td class="text-muted">{{ log.group_name || '—' }}</td>
                <td>
                  <UiBadge :tone="statusTone(log.status_code)">{{ log.status_code }}</UiBadge>
                </td>
                <td class="num">{{ formatNumber(log.prompt_tokens) }}</td>
                <td class="num">{{ formatNumber(log.cached_prompt_tokens) }}</td>
                <td class="num">{{ formatNumber(log.completion_tokens) }}</td>
                <td class="num">{{ t('admin.durationMs', { value: log.duration_ms }) }}</td>
                <td class="num">
                  <span class="inline-flex items-center justify-end gap-1.5">
                    {{ formatMoney(log.cost, 4) }}
                    <UiBadge v-if="log.subscription" tone="success" dot>{{ t('admin.subscriptionCovered') }}</UiBadge>
                  </span>
                </td>
                <td>
                  <UiButton variant="ghost" size="sm" @click="detailTarget = log">{{ t('common.detail') }}</UiButton>
                </td>
              </tr>
            </tbody>
          </UiTable>

          <div class="flex flex-wrap items-center justify-between gap-3 pt-3">
            <p class="text-[13px] text-muted">{{ t('common.totalItems', { total: usage.data.value.total }) }}</p>
            <div class="flex items-center gap-2">
              <UiButton
                variant="secondary"
                size="sm"
                :disabled="usage.data.value.page <= 1"
                @click="goToPage(usage.data.value.page - 1)"
              >
                {{ t('common.prev') }}
              </UiButton>
              <span class="numeric text-[13px] text-muted">
                {{ t('admin.pageOf', { page: usage.data.value.page, pages: totalPages }) }}
              </span>
              <UiButton
                variant="secondary"
                size="sm"
                :disabled="usage.data.value.page >= totalPages"
                @click="goToPage(usage.data.value.page + 1)"
              >
                {{ t('common.next') }}
              </UiButton>
            </div>
          </div>
        </ConsoleOpsListState>
      </div>

      <div v-else class="space-y-4 pt-4">
        <ConsoleOpsPageHeader>
          <template #actions>
            <UiButton variant="secondary" size="sm" @click="requests.refresh()">{{ t('common.refresh') }}</UiButton>
          </template>
        </ConsoleOpsPageHeader>

        <ConsoleOpsListState
          :pending="requests.pending.value"
          :error="requests.error.value"
          :empty="!requests.data.value.data.length"
          :empty-icon="ScrollText"
          :empty-title="t('admin.requestsEmptyTitle')"
          :empty-description="t('admin.requestsEmptyBody')"
        >
          <UiTable dense>
            <thead>
              <tr>
                <th>{{ t('admin.time') }}</th>
                <th>{{ t('admin.requestId') }}</th>
                <th>{{ t('admin.user') }}</th>
                <th>{{ t('admin.keyName') }}</th>
                <th>{{ t('admin.model') }}</th>
                <th>{{ t('admin.channel') }}</th>
                <th>{{ t('admin.channelKey') }}</th>
                <th>{{ t('admin.statusCode') }}</th>
                <th class="num">{{ t('admin.statPromptTokens') }}</th>
                <th class="num">{{ t('admin.statCompletionTokens') }}</th>
                <th class="num">{{ t('admin.statTotalTokens') }}</th>
                <th class="num">{{ t('admin.duration') }}</th>
                <th>{{ t('admin.errorCode') }}</th>
                <th>{{ t('common.detail') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="log in requests.data.value.data" :key="log.request_id">
                <td class="text-muted whitespace-nowrap">{{ formatDateTime(log.created_at) }}</td>
                <td class="font-mono text-[13px] text-muted">{{ shortId(log.request_id) }}</td>
                <td class="text-muted">{{ log.user_name || shortId(log.user_id) }}</td>
                <td class="text-muted">{{ log.key_name || '—' }}</td>
                <td class="font-medium text-ink">{{ log.model }}</td>
                <td class="text-muted">{{ log.channel_name || '—' }}</td>
                <td class="text-muted">{{ log.channel_key_name || '—' }}</td>
                <td><UiBadge :tone="statusTone(log.status_code)">{{ log.status_code }}</UiBadge></td>
                <td class="num">{{ formatNumber(log.prompt_tokens) }}</td>
                <td class="num">{{ formatNumber(log.completion_tokens) }}</td>
                <td class="num">{{ formatNumber(log.total_tokens) }}</td>
                <td class="num">{{ t('admin.durationMs', { value: log.duration_ms }) }}</td>
                <td class="text-muted">{{ log.error_code || '—' }}</td>
                <td>
                  <UiButton variant="ghost" size="sm" @click="detailTarget = log">{{ t('common.detail') }}</UiButton>
                </td>
              </tr>
            </tbody>
          </UiTable>
        </ConsoleOpsListState>
      </div>
    </UiTabs>

    <UiDialog v-model:open="detailTarget" :title="t('common.detail')">
      <div class="space-y-3 text-sm">
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('admin.requestId') }}</div>
          <div class="break-all font-mono text-ink">{{ detailTarget?.request_id || '—' }}</div>
        </div>
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('admin.clientIp') }}</div>
          <div class="font-mono text-ink">{{ detailTarget?.client_ip || '—' }}</div>
        </div>
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('admin.userAgent') }}</div>
          <div class="break-all font-mono text-ink">{{ detailTarget?.user_agent || '—' }}</div>
        </div>
        <div class="rounded-control border border-line bg-sunken/40 p-3">
          <div class="mb-2 text-xs font-medium text-muted">{{ t('admin.tokenBreakdown') }}</div>
          <div class="grid grid-cols-2 gap-x-4 gap-y-2">
            <div>
              <div class="text-2xs text-faint">{{ t('admin.statPromptTokens') }}</div>
              <div class="numeric text-[13px] text-ink">{{ detailTarget ? formatNumber(detailTarget.prompt_tokens ?? 0) : '—' }}</div>
            </div>
            <div>
              <div class="text-2xs text-faint">{{ t('admin.statCachedTokens') }}</div>
              <div class="numeric text-[13px] text-ink">{{ detailTarget ? formatNumber(detailTarget.cached_prompt_tokens ?? 0) : '—' }}</div>
            </div>
            <div>
              <div class="text-2xs text-faint">{{ t('admin.statCompletionTokens') }}</div>
              <div class="numeric text-[13px] text-ink">{{ detailTarget ? formatNumber(detailTarget.completion_tokens ?? 0) : '—' }}</div>
            </div>
          </div>
        </div>
        <div v-if="detailTarget && 'subscription' in detailTarget">
          <div class="mb-1 text-xs text-muted">{{ t('admin.cost') }}</div>
          <div class="text-[13px] text-ink">
            <UiBadge v-if="detailTarget.subscription" tone="success" dot>{{ t('admin.subscriptionCovered') }}</UiBadge>
            <template v-else>{{ formatMoney(Number(detailTarget.cost ?? 0), 4) }}</template>
          </div>
        </div>
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('admin.errorDetail') }}</div>
          <div class="break-all font-mono text-ink">{{ detailTarget?.error_detail || '—' }}</div>
        </div>
      </div>
    </UiDialog>
  </div>
</template>
