<script setup lang="ts">
import { ScrollText } from 'lucide-vue-next'
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
  start: '',
  end: '',
})

const page = ref(1)
const pageSize = ref('50')

const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'success', label: t('admin.statusSuccess') },
  { value: 'error', label: t('admin.statusError') },
])
const pageSizeOptions = ['20', '50', '100', '200'].map(value => ({ value, label: value }))

/** `datetime-local` yields a local wall-clock string; the API wants RFC 3339. */
function toRfc3339(value: string): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toISOString()
}

function filterParams(): URLSearchParams {
  const params = new URLSearchParams()
  if (filters.user_id.trim()) params.set('user_id', filters.user_id.trim())
  if (filters.model.trim()) params.set('model', filters.model.trim())
  if (filters.channel_id.trim()) params.set('channel_id', filters.channel_id.trim())
  if (filters.group_id.trim()) params.set('group_id', filters.group_id.trim())
  if (filters.status) params.set('status', filters.status)
  const start = toRfc3339(filters.start)
  if (start) params.set('start', start)
  const end = toRfc3339(filters.end)
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
  filters.start = ''
  filters.end = ''
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

        <UiCard :title="t('common.filter')">
          <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <UiField :label="t('admin.filterUserId')">
              <UiInput v-model="filters.user_id" mono />
            </UiField>
            <UiField :label="t('admin.model')">
              <UiInput v-model="filters.model" mono />
            </UiField>
            <UiField :label="t('admin.filterChannelId')">
              <UiInput v-model="filters.channel_id" mono />
            </UiField>
            <UiField :label="t('admin.filterGroupId')">
              <UiInput v-model="filters.group_id" mono />
            </UiField>
            <UiField :label="t('common.status')">
              <UiSelect v-model="filters.status" :options="statusOptions" :placeholder="t('common.all')" />
            </UiField>
            <UiField :label="t('admin.filterStart')">
              <UiInput v-model="filters.start" type="datetime-local" />
            </UiField>
            <UiField :label="t('admin.filterEnd')">
              <UiInput v-model="filters.end" type="datetime-local" />
            </UiField>
            <UiField :label="t('admin.pageSize')">
              <UiSelect v-model="pageSize" :options="pageSizeOptions" :placeholder="t('common.selectPlaceholder')" />
            </UiField>
          </div>

          <template #footer>
            <div class="flex justify-end gap-2">
              <UiButton variant="secondary" size="sm" @click="resetFilters">{{ t('common.reset') }}</UiButton>
              <UiButton size="sm" @click="applyFilters">{{ t('common.filter') }}</UiButton>
            </div>
          </template>
        </UiCard>

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
                <td class="num">{{ formatNumber(log.completion_tokens) }}</td>
                <td class="num">{{ t('admin.durationMs', { value: log.duration_ms }) }}</td>
                <td class="num">{{ formatMoney(log.cost, 4) }}</td>
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
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('admin.errorDetail') }}</div>
          <div class="break-all font-mono text-ink">{{ detailTarget?.error_detail || '—' }}</div>
        </div>
      </div>
    </UiDialog>
  </div>
</template>
