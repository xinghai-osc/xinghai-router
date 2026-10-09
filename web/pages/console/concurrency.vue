<script setup lang="ts">
import { Gauge, RefreshCw } from 'lucide-vue-next'
import { endpoints, type ConcurrencyScope, type ConcurrencyStatus, type ConcurrencyStatusEntry, type ConcurrencyStatusScope } from '~/src/api'
import { formatDateTime, formatNumber, formatPercent, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()

useHead({ title: () => `${t('nav.concurrency')} · ${site.value.name}` })

const AUTO_REFRESH_MS = 5000
const autoRefresh = ref(true)
const initial: ConcurrencyStatus = { deployment_mode: 'single', shared: false, generated_at: '', scopes: [] }
const { data, pending, error, refresh } = useResource(() => endpoints.getConcurrencyStatus(), initial)

const scopeOrder: ConcurrencyScope[] = ['channel', 'user', 'group']
const scopes = computed(() => scopeOrder
  .map(scope => data.value.scopes.find(item => item.scope === scope))
  .filter((item): item is ConcurrencyStatusScope => Boolean(item)))

let timer: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  timer = setInterval(() => {
    if (autoRefresh.value && document.visibilityState === 'visible') refresh()
  }, AUTO_REFRESH_MS)
})
onUnmounted(() => { if (timer) clearInterval(timer) })
watch(autoRefresh, enabled => { if (enabled) refresh() })

function scopeLabel(scope: ConcurrencyScope): string {
  if (scope === 'channel') return t('admin.concurrencyScopeChannel')
  if (scope === 'user') return t('admin.concurrencyScopeUser')
  return t('admin.concurrencyScopeGroup')
}

function entryLabel(entry: ConcurrencyStatusEntry): string {
  return entry.name || entry.email || shortId(entry.id)
}

function usagePercent(scope: { current: number; limit: number }): number {
  if (scope.limit <= 0) return 0
  return Math.min(100, Math.round((scope.current / scope.limit) * 1000) / 10)
}
</script>

<template>
  <ConsoleSystemGate permission="logs.read">
    <div class="space-y-4">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('admin.concurrencyTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('admin.concurrencyLead') }}</p>
        </div>
        <div class="flex items-center gap-3">
          <UiBadge :tone="data.shared ? 'success' : 'warn'" dot>
            {{ data.shared ? t('admin.concurrencyShared') : t('admin.concurrencyLocalOnly') }}
          </UiBadge>
          <UiSwitch v-model="autoRefresh" :label="t('admin.concurrencyAutoRefresh')" />
          <UiButton variant="secondary" size="sm" :loading="pending" @click="refresh">
            <RefreshCw class="size-4" />
            {{ t('common.refresh') }}
          </UiButton>
        </div>
      </div>

      <UiAlert v-if="error" tone="danger" :title="t('common.loadFailed')">{{ error }}</UiAlert>
      <UiAlert v-else-if="scopes.length && !data.shared" tone="warn">{{ t('admin.concurrencyLocalOnlyHint') }}</UiAlert>

      <UiSkeleton v-if="!scopes.length && !error" :rows="3" class="h-10" />

      <template v-else-if="scopes.length">
        <p class="text-2xs text-faint">{{ t('admin.concurrencyUpdatedAt', { time: formatDateTime(data.generated_at) }) }}</p>
        <UiCard
          v-for="scope in scopes"
          :key="scope.scope"
          :title="scopeLabel(scope.scope)"
          :description="t('admin.concurrencyScopeSummary', { current: formatNumber(scope.current), limit: formatNumber(scope.limit) })"
        >
          <UiEmptyState
            v-if="!scope.entries.length"
            :icon="Gauge"
            :title="t('admin.concurrencyScopeEmpty')"
            :description="t('admin.concurrencyScopeEmptyBody')"
          />
          <UiTable v-else dense>
            <thead>
              <tr>
                <th>{{ t('admin.concurrencyColumnName') }}</th>
                <th class="num">{{ t('admin.concurrencyColumnCurrent') }}</th>
                <th class="num">{{ t('admin.concurrencyColumnLimit') }}</th>
                <th>{{ t('admin.concurrencyColumnUsage') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in scope.entries" :key="entry.id">
                <td>
                  <div class="font-medium text-ink">{{ entryLabel(entry) }}</div>
                  <div v-if="entry.name && entry.email" class="text-2xs text-faint">{{ entry.email }}</div>
                </td>
                <td class="num">{{ formatNumber(entry.current) }}</td>
                <td class="num">{{ formatNumber(entry.limit) }}</td>
                <td>
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-24 rounded-full bg-sunken">
                      <div
                        class="h-full rounded-full"
                        :class="usagePercent(entry) >= 100 ? 'bg-danger' : 'bg-clay'"
                        :style="{ width: `${usagePercent(entry)}%` }"
                      />
                    </div>
                    <span class="numeric text-muted">{{ formatPercent(usagePercent(entry)) }}</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </UiTable>
        </UiCard>
      </template>
    </div>
  </ConsoleSystemGate>
</template>