<script setup lang="ts">
import { CalendarDays, ChevronDown, KeyRound, Search } from 'lucide-vue-next'
import {
  PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger,
} from 'reka-ui'
import { endpoints, type UsageRecord } from '~/src/api'
import { formatCompact, formatDateTime, formatMoney, formatNumber, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.usage')} · ${settings.value.name}` })

const { data: usage, pending, error } = useResource(
  () => endpoints.getAccountUsage(),
  { data: [] as UsageRecord[] },
)

const statusOptions = computed(() => [
  { value: '', label: t('console.statusAll') },
  { value: 'success', label: t('console.statusSuccess') },
  { value: 'failed', label: t('console.statusFailed') },
  { value: 'settled', label: t('console.statusSettled') },
])

const model = ref('')
const group = ref('')
const key = ref('')
const requestId = ref('')
const status = ref('')
const advancedOpen = ref(false)

const DATE_PRESETS = [
  { days: 1, labelKey: 'console.datePreset24h' },
  { days: 7, labelKey: 'console.datePreset7d' },
  { days: 14, labelKey: 'console.datePreset14d' },
  { days: 30, labelKey: 'console.datePreset30d' },
]

const range = reactive<{ start: Date | null; end: Date | null }>({ start: null, end: null })
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

function applyRangePreset(days: number) {
  const end = new Date()
  const start = new Date(end.getTime() - days * 24 * 3600 * 1000)
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

const filtersActive = computed(() => Boolean(
  model.value.trim() || group.value.trim() || key.value.trim()
  || requestId.value.trim() || status.value || range.start || range.end,
))

function resetFilters() {
  model.value = ''
  group.value = ''
  key.value = ''
  requestId.value = ''
  status.value = ''
  range.start = null
  range.end = null
}

function isFailed(record: UsageRecord): boolean {
  return record.status === 'failed'
}

const clientTarget = ref<UsageRecord | null>(null)
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
                        @click="applyRangePreset(preset.days)"
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
            <UiInput v-model="group" :placeholder="t('console.group')" />
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
        </div>

        <div v-if="advancedOpen" class="mt-2 flex flex-wrap items-end gap-2 border-t border-line pt-3">
          <div class="w-full sm:w-52">
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
              <span class="numeric font-semibold text-ink">{{ formatMoney(stats.cost, 4) }}</span>
            </span>
            <span class="inline-flex h-7 items-center gap-2 rounded-md border border-line bg-surface px-2.5 text-xs">
              <span class="h-3.5 w-0.5 rounded-full bg-success" />
              <span class="text-muted">{{ t('console.requests') }}</span>
              <span class="numeric font-semibold text-ink">{{ formatNumber(stats.requests) }}</span>
            </span>
            <span class="inline-flex h-7 items-center gap-2 rounded-md border border-line bg-surface px-2.5 text-xs">
              <span class="h-3.5 w-0.5 rounded-full bg-warn" />
              <span class="text-muted">{{ t('console.tokens') }}</span>
              <span class="numeric font-semibold text-ink">{{ formatCompact(stats.tokens) }}</span>
            </span>
          </div>
          <div class="ml-auto">
            <UiButton v-if="filtersActive" variant="ghost" size="sm" @click="resetFilters">
              {{ t('common.reset') }}
            </UiButton>
          </div>
        </div>
      </div>

      <ConsoleUserDataState
        :pending="pending"
        :error="error"
        :empty="!filtered.length"
        :rows="6"
        :empty-icon="Search"
        :empty-title="filtersActive ? t('console.noMatchTitle') : t('console.usageEmptyTitle')"
        :empty-description="filtersActive ? t('console.noMatchBody') : t('console.usageEmptyBody')"
      >
        <UiTable dense>
          <thead>
            <tr>
              <th>{{ t('console.time') }}</th>
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
              v-for="record in filtered"
              :key="record.request_id"
              :class="isFailed(record) && 'bg-danger-soft/40'"
            >
              <td class="whitespace-nowrap">
                <div class="flex min-w-0 flex-col gap-1">
                  <span class="font-mono text-xs tabular-nums text-ink">{{ formatDateTime(record.created_at) }}</span>
                  <ConsoleUserStatusBadge :status="record.status" />
                </div>
              </td>
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
                  <span v-if="record.cached_prompt_tokens > 0" class="text-2xs text-faint">
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
      </ConsoleUserDataState>
    </div>

    <UiDialog v-model:open="clientTarget" :title="t('console.requestDetail')">
      <div class="space-y-4 text-sm">
        <div class="space-y-1.5">
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