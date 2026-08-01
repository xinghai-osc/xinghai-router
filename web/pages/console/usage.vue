<script setup lang="ts">
import { Activity, Search } from 'lucide-vue-next'
import { endpoints, type UsageRecord } from '~/src/api'
import { formatDateTime, formatMoney, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.usage')} · ${settings.value.name}` })

const { data: usage, pending, error } = useResource(
  () => endpoints.getAccountUsage(),
  { data: [] as UsageRecord[] },
)

const query = ref('')
const model = ref('')
const key = ref('')
const group = ref('')
const status = ref('')
const start = ref('')
const end = ref('')

function nameOptions(records: UsageRecord[], pick: (record: UsageRecord) => string, allLabel: string) {
  return [
    { value: '', label: allLabel },
    ...[...new Set(records.map(pick).filter(Boolean))]
      .sort()
      .map(name => ({ value: name, label: name })),
  ]
}

const modelOptions = computed(() => nameOptions(usage.value.data, record => record.model, t('console.allModels')))
const keyOptions = computed(() => nameOptions(usage.value.data, record => record.key_name, t('console.allKeys')))
const groupOptions = computed(() => nameOptions(usage.value.data, record => record.group_name, t('console.allGroups')))

const statusOptions = computed(() => [
  { value: '', label: t('console.statusAll') },
  { value: 'settled', label: t('console.statusSettled') },
  { value: 'success', label: t('console.statusSuccess') },
  { value: 'failed', label: t('console.statusFailed') },
])

const filtered = computed(() => {
  const needle = query.value.trim().toLowerCase()
  return usage.value.data.filter((record) => {
    if (model.value && record.model !== model.value) return false
    if (key.value && record.key_name !== key.value) return false
    if (group.value && record.group_name !== group.value) return false
    if (status.value && record.status !== status.value) return false
    if (start.value && new Date(record.created_at) < new Date(start.value)) return false
    if (end.value) {
      const endDate = new Date(end.value)
      endDate.setDate(endDate.getDate() + 1)
      if (new Date(record.created_at) >= endDate) return false
    }
    if (!needle) return true
    return record.model.toLowerCase().includes(needle)
      || record.request_id.toLowerCase().includes(needle)
  })
})

const totals = computed(() => filtered.value.reduce(
  (sum, record) => ({
    prompt: sum.prompt + record.prompt_tokens,
    cached: sum.cached + record.cached_prompt_tokens,
    completion: sum.completion + record.completion_tokens,
    total: sum.total + record.prompt_tokens + record.completion_tokens,
    cost: sum.cost + Number(record.cost ?? 0),
  }),
  { prompt: 0, cached: 0, completion: 0, total: 0, cost: 0 },
))

const filtersActive = computed(() => Boolean(query.value.trim() || model.value || key.value || group.value || status.value || start.value || end.value))

function resetFilters() {
  query.value = ''
  model.value = ''
  key.value = ''
  group.value = ''
  status.value = ''
  start.value = ''
  end.value = ''
}

const clientTarget = ref<UsageRecord | null>(null)
</script>

<template>
  <UiCard :title="t('nav.usage')" :description="t('console.usageDescription')" flush>
    <div class="space-y-4 px-5 py-4">
      <div class="flex flex-wrap items-center gap-2">
        <div class="min-w-52 flex-1">
          <UiInput v-model="query" :placeholder="t('console.usageSearchPlaceholder')">
            <template #leading>
              <Search class="size-4" />
            </template>
          </UiInput>
        </div>
        <div class="w-full sm:w-40">
          <UiSelect v-model="model" :options="modelOptions" :placeholder="t('console.allModels')" />
        </div>
        <div class="w-full sm:w-40">
          <UiSelect v-model="key" :options="keyOptions" :placeholder="t('console.allKeys')" />
        </div>
        <div class="w-full sm:w-36">
          <UiSelect v-model="group" :options="groupOptions" :placeholder="t('console.allGroups')" />
        </div>
        <div class="w-full sm:w-36">
          <UiSelect v-model="status" :options="statusOptions" :placeholder="t('console.statusAll')" />
        </div>
        <div class="w-full sm:w-44">
          <UiInput v-model="start" type="datetime-local" :aria-label="t('console.filterStart')" />
        </div>
        <div class="w-full sm:w-44">
          <UiInput v-model="end" type="datetime-local" :aria-label="t('console.filterEnd')" />
        </div>
        <UiButton v-if="filtersActive" variant="ghost" @click="resetFilters">
          {{ t('common.reset') }}
        </UiButton>
      </div>

      <ConsoleUserDataState
        :pending="pending"
        :error="error"
        :empty="!filtered.length"
        :rows="6"
        :empty-icon="Activity"
        :empty-title="filtersActive ? t('console.noMatchTitle') : t('console.usageEmptyTitle')"
        :empty-description="filtersActive ? t('console.noMatchBody') : t('console.usageEmptyBody')"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('console.time') }}</th>
              <th>{{ t('console.keyUsed') }}</th>
              <th>{{ t('console.model') }}</th>
              <th>{{ t('console.group') }}</th>
              <th class="num">{{ t('console.duration') }}</th>
              <th class="num">{{ t('console.promptTokens') }}</th>
              <th class="num">{{ t('console.cachedTokens') }}</th>
              <th class="num">{{ t('console.completionTokens') }}</th>
              <th class="num">{{ t('console.totalTokens') }}</th>
              <th class="num">{{ t('console.cost') }}</th>
              <th>{{ t('common.status') }}</th>
              <th>{{ t('common.detail') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="record in filtered" :key="record.request_id">
              <td class="text-muted whitespace-nowrap">{{ formatDateTime(record.created_at) }}</td>
              <td class="text-muted">{{ record.key_name || '-' }}</td>
              <td class="font-medium">{{ record.model }}</td>
              <td class="text-muted">{{ record.group_name || '-' }}</td>
              <td class="num text-muted">{{ t('console.durationMs', { value: record.duration_ms }) }}</td>
              <td class="num">{{ formatNumber(record.prompt_tokens) }}</td>
              <td class="num text-muted">{{ formatNumber(record.cached_prompt_tokens) }}</td>
              <td class="num">{{ formatNumber(record.completion_tokens) }}</td>
              <td class="num">{{ formatNumber(record.prompt_tokens + record.completion_tokens) }}</td>
              <td class="num">
                <UiBadge v-if="record.subscription" tone="clay">{{ t('console.subscriptionCovered') }}</UiBadge>
                <template v-else>{{ formatMoney(record.cost, 4) }}</template>
              </td>
              <td><ConsoleUserStatusBadge :status="record.status" /></td>
              <td>
                <UiButton variant="ghost" size="sm" @click="clientTarget = record">
                  {{ t('common.detail') }}
                </UiButton>
              </td>
            </tr>
          </tbody>
          <tfoot>
            <tr class="border-t border-line-strong bg-sunken font-medium">
              <td colspan="5">{{ t('console.totalsRow', { count: filtered.length }) }}</td>
              <td class="num">{{ formatNumber(totals.prompt) }}</td>
              <td class="num">{{ formatNumber(totals.cached) }}</td>
              <td class="num">{{ formatNumber(totals.completion) }}</td>
              <td class="num">{{ formatNumber(totals.total) }}</td>
              <td class="num">{{ formatMoney(totals.cost, 4) }}</td>
              <td colspan="2" />
            </tr>
          </tfoot>
        </UiTable>
      </ConsoleUserDataState>
    </div>

    <UiDialog v-model:open="clientTarget" :title="t('common.detail')">
      <div class="space-y-3 text-sm">
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('console.requestId') }}</div>
          <div class="break-all font-mono text-ink">{{ clientTarget?.request_id || '-' }}</div>
        </div>
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('console.clientIp') }}</div>
          <div class="font-mono text-ink">{{ clientTarget?.client_ip || '-' }}</div>
        </div>
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('console.userAgent') }}</div>
          <div class="break-all font-mono text-ink">{{ clientTarget?.user_agent || '-' }}</div>
        </div>
        <div>
          <div class="mb-1 text-xs text-muted">{{ t('console.errorDetail') }}</div>
          <div class="break-all font-mono text-ink">{{ clientTarget?.error || '-' }}</div>
        </div>
      </div>
    </UiDialog>
  </UiCard>
</template>
