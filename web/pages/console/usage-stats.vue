<script setup lang="ts">
import { CalendarDays, RefreshCw, RotateCcw, Sigma } from 'lucide-vue-next'
import {
  PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger,
} from 'reka-ui'
import { endpoints, type UsageStatsPage } from '~/src/api'
import { formatMoney, formatNumber, formatPercent } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { can } = useAccount()
const allowed = computed(() => can('logs.read'))

useHead({ title: () => `${t('nav.usageStats')} · ${settings.value.name}` })

type Dimension = 'model' | 'channel'
type Preset = '24h' | 'today' | '7d' | '30d'
type WindowValue = Preset | 'custom'

const dimension = ref<Dimension>('model')
const page = ref(1)
const pageSize = ref('50')
const pageSizeOptions = ['20', '50', '100']
const activeWindow = ref<WindowValue>('24h')

function defaultStart(): Date {
  return new Date(Date.now() - 24 * 3600 * 1000)
}

function defaultEnd(): Date {
  return new Date()
}

const range = reactive<{ start: Date | null; end: Date | null }>({ start: defaultStart(), end: defaultEnd() })
const rangeDraft = reactive({ start: '', end: '' })
const rangeOpen = ref(false)

const dimensionTabs = computed(() => [
  { value: 'model', label: t('admin.usageStatsDimensionModel') },
  { value: 'channel', label: t('admin.usageStatsDimensionChannel') },
])

const windowPresets = computed(() => [
  { value: '24h' as const, label: t('admin.usageStatsWindow24h') },
  { value: 'today' as const, label: t('admin.usageStatsWindowToday') },
  { value: '7d' as const, label: t('admin.usageStatsWindow7d') },
  { value: '30d' as const, label: t('admin.usageStatsWindow30d') },
])

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

function toInputValue(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function compactDate(date: Date): string {
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function toRfc3339(value: Date | null): string {
  if (!value || Number.isNaN(value.getTime())) return ''
  return value.toISOString()
}

const rangeLabel = computed(() => {
  if (!range.start && !range.end) return t('admin.dateRange')
  if (range.start && range.end) return `${compactDate(range.start)} ~ ${compactDate(range.end)}`
  return `${range.start ? compactDate(range.start) : t('common.none')} ~ ${range.end ? compactDate(range.end) : t('common.none')}`
})

watch(rangeOpen, (open) => {
  if (open) {
    rangeDraft.start = range.start ? toInputValue(range.start) : ''
    rangeDraft.end = range.end ? toInputValue(range.end) : ''
  }
})

function presetStart(preset: Preset, now: Date): Date {
  if (preset === 'today') {
    const start = new Date(now)
    start.setHours(0, 0, 0, 0)
    return start
  }
  const hours = preset === '24h' ? 24 : preset === '7d' ? 7 * 24 : 30 * 24
  return new Date(now.getTime() - hours * 3600 * 1000)
}

async function applyPreset(preset: Preset) {
  const now = new Date()
  activeWindow.value = preset
  range.start = presetStart(preset, now)
  range.end = now
  page.value = 1
  await stats.refresh()
}

async function applyRangeDraft() {
  const nextStart = rangeDraft.start ? new Date(rangeDraft.start) : null
  const nextEnd = rangeDraft.end ? new Date(rangeDraft.end) : null
  if (nextStart && Number.isNaN(nextStart.getTime())) return
  if (nextEnd && Number.isNaN(nextEnd.getTime())) return
  range.start = nextStart
  range.end = nextEnd
  activeWindow.value = 'custom'
  rangeOpen.value = false
  page.value = 1
  await stats.refresh()
}

function openCustomRange() {
  activeWindow.value = 'custom'
  rangeOpen.value = true
}

function query(): string {
  const params = new URLSearchParams({
    group_by: dimension.value,
    page: String(page.value),
    page_size: pageSize.value,
  })
  const start = toRfc3339(range.start)
  const end = toRfc3339(range.end)
  if (start) params.set('start', start)
  if (end) params.set('end', end)
  return `?${params.toString()}`
}

const emptyPage: UsageStatsPage = { data: [], total: 0, page: 1, page_size: 50 }
const stats = useResource(() => endpoints.getUsageStatsByDimension(query()), emptyPage)
const totalPages = computed(() => Math.max(1, Math.ceil(stats.data.value.total / Math.max(1, stats.data.value.page_size))))
const dimensionLabel = computed(() => dimension.value === 'model' ? t('admin.usageStatsModel') : t('admin.usageStatsChannel'))

watch(dimension, () => {
  page.value = 1
  void stats.refresh()
})

watch(pageSize, () => {
  page.value = 1
  void stats.refresh()
})

async function resetFilters() {
  dimension.value = 'model'
  activeWindow.value = '24h'
  range.start = defaultStart()
  range.end = defaultEnd()
  page.value = 1
  await stats.refresh()
}

async function goToPage(next: number) {
  if (next < 1 || next > totalPages.value) return
  page.value = next
  await stats.refresh()
}

function refresh() {
  return stats.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.usageStatsLead')">
      <template #actions>
        <UiButton variant="secondary" size="sm" :loading="stats.pending.value" @click="refresh">
          <RefreshCw class="size-4" />
          {{ t('common.refresh') }}
        </UiButton>
        <UiButton variant="ghost" size="sm" @click="resetFilters">
          <RotateCcw class="size-4" />
          {{ t('common.reset') }}
        </UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <UiCard :title="t('nav.usageStats')" flush>
      <div class="space-y-4 px-5 py-4">
        <UiTabs v-model="dimension" :items="dimensionTabs" />

        <div class="rounded-control border border-line bg-sunken/40 p-3">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-xs text-muted">{{ t('admin.usageStatsWindow') }}</span>
            <div class="flex flex-wrap items-center gap-1 rounded-control border border-line bg-surface p-1" role="group" :aria-label="t('admin.usageStatsWindow')">
              <button
                v-for="preset in windowPresets"
                :key="preset.value"
                type="button"
                class="rounded-[7px] px-3 py-1.5 text-[13px] transition-colors duration-150"
                :class="activeWindow === preset.value ? 'bg-clay text-clay-ink' : 'text-muted hover:text-ink'"
                @click="applyPreset(preset.value)"
              >
                {{ preset.label }}
              </button>
              <button
                type="button"
                class="rounded-[7px] px-3 py-1.5 text-[13px] transition-colors duration-150"
                :class="activeWindow === 'custom' ? 'bg-clay text-clay-ink' : 'text-muted hover:text-ink'"
                @click="openCustomRange"
              >
                {{ t('admin.usageStatsWindowCustom') }}
              </button>
            </div>

            <PopoverRoot v-model:open="rangeOpen">
              <PopoverTrigger as-child>
                <button
                  type="button"
                  class="inline-flex h-9 min-w-56 items-center gap-2 rounded-control border border-line-strong bg-surface px-3 text-sm transition-colors duration-150 hover:border-faint focus:border-clay focus:outline-none focus:ring-2 focus:ring-clay/20"
                  :aria-label="t('admin.dateRange')"
                >
                  <CalendarDays class="size-4 shrink-0 text-faint" />
                  <span class="truncate tabular-nums text-ink">{{ rangeLabel }}</span>
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
                      <span class="hidden pb-2 text-xs text-muted sm:block" aria-hidden="true">~</span>
                      <div class="space-y-1.5">
                        <div class="text-xs text-muted">{{ t('admin.filterEnd') }}</div>
                        <UiInput v-model="rangeDraft.end" type="datetime-local" class="tabular-nums" />
                      </div>
                    </div>
                    <div class="flex justify-end">
                      <UiButton size="sm" @click="applyRangeDraft">{{ t('common.confirm') }}</UiButton>
                    </div>
                  </div>
                </PopoverContent>
              </PopoverPortal>
            </PopoverRoot>
          </div>
        </div>

        <ConsoleOpsListState
          :pending="stats.pending.value"
          :error="stats.error.value"
          :empty="!stats.data.value.data.length"
          :empty-icon="Sigma"
          :empty-title="t('admin.usageStatsEmptyTitle')"
          :empty-description="t('admin.usageStatsEmptyBody')"
        >
          <UiTable dense>
            <thead>
              <tr>
                <th>{{ dimensionLabel }}</th>
                <th class="num">{{ t('admin.statTotalTokens') }}</th>
                <th class="num">{{ t('admin.statPromptTokens') }}</th>
                <th class="num">{{ t('admin.statCompletionTokens') }}</th>
                <th class="num">{{ t('admin.usageStatsCacheHit') }} / {{ t('admin.usageStatsCacheHitRate') }}</th>
                <th class="num">{{ t('admin.usageStatsUncachedInput') }}</th>
                <th class="num">{{ t('admin.usageStatsCacheWrite') }}</th>
                <th class="num">{{ t('admin.statRequests') }}</th>
                <th class="num">{{ t('admin.statCost') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in stats.data.value.data" :key="row.key || row.name">
                <td class="font-medium text-ink">{{ row.name || t('common.none') }}</td>
                <td class="num">{{ formatNumber(row.total_tokens) }}</td>
                <td class="num">{{ formatNumber(row.prompt_tokens) }}</td>
                <td class="num">{{ formatNumber(row.completion_tokens) }}</td>
                <td class="num">
                  <div class="flex flex-col items-end gap-0.5">
                    <span>{{ formatNumber(row.cache_hit_tokens) }}</span>
                    <span class="text-2xs text-muted">{{ formatPercent(row.cache_hit_rate) }}</span>
                  </div>
                </td>
                <td class="num">{{ formatNumber(row.uncached_input_tokens) }}</td>
                <td class="num">{{ formatNumber(row.cache_write_tokens) }}</td>
                <td class="num">{{ formatNumber(row.requests) }}</td>
                <td class="num">{{ formatMoney(row.cost, 4) }}</td>
              </tr>
            </tbody>
          </UiTable>
          <ConsoleOpsPagination
            :page="stats.data.value.page"
            :page-size="pageSize"
            :total="stats.data.value.total"
            :page-size-options="pageSizeOptions"
            @update:page="goToPage"
            @update:page-size="pageSize = $event"
          />
        </ConsoleOpsListState>
      </div>
    </UiCard>
  </div>
</template>
