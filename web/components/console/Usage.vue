<script setup lang="ts">
import { computed, ref, reactive, onMounted } from 'vue'
import { useConsoleStore } from '~/composables/useConsoleStore'
import { endpoints } from '~/src/api'
import type { UsageLog, UsageStats } from '~/src/api'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'

const store = useConsoleStore()
const { t, can, formatDate, activityLogs, activityModels, activityFilters, activityTypeLabel, actionLabel, activityDetail, filterActivity, resetActivityFilters: resetOldFilters, users, groups } = store
const isAdmin = computed(() => can('logs.read'))

// --- Admin mode: paginated usage logs ---
const usageLogs = ref<UsageLog[]>([])
const usageStats = ref<UsageStats | null>(null)
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const totalPages = computed(() => Math.ceil(total.value / pageSize.value) || 1)
const adminFilters = reactive({ user_id: '', model: '', channel_id: '', group_id: '', status: '', start: '', end: '' })
const adminModels = ref<string[]>([])
const selectClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
const inputClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'

async function loadUsageLogs(resetPage = false) {
  if (resetPage) page.value = 1
  const query = new URLSearchParams()
  query.set('page', String(page.value))
  query.set('page_size', String(pageSize.value))
  for (const [key, value] of Object.entries(adminFilters)) {
    if (value) query.set(key, key === 'start' || key === 'end' ? new Date(value).toISOString() : value)
  }
  try {
    const result = await endpoints.getUsageLogs(`?${query}`)
    usageLogs.value = result.data
    total.value = result.total
    if (resetPage || !adminModels.value.length) {
      adminModels.value = [...new Set(result.data.map((item) => item.model).filter(Boolean))].sort()
    }
  } catch { usageLogs.value = [] }
}

async function loadUsageStats() {
  const query = new URLSearchParams()
  query.set('breakdown', '1')
  for (const [key, value] of Object.entries(adminFilters)) {
    if (value) query.set(key, key === 'start' || key === 'end' ? new Date(value).toISOString() : value)
  }
  try { usageStats.value = await endpoints.getUsageStats(`?${query}`) } catch { usageStats.value = null }
}

async function applyAdminFilters() { page.value = 1; await Promise.all([loadUsageLogs(), loadUsageStats()]) }
function resetAdminFilters() { Object.assign(adminFilters, { user_id: '', model: '', channel_id: '', group_id: '', status: '', start: '', end: '' }); applyAdminFilters() }
function prevPage() { if (page.value > 1) { page.value--; loadUsageLogs() } }
function nextPage() { if (page.value < totalPages.value) { page.value++; loadUsageLogs() } }

const breakdownTokens = computed(() => {
  if (!usageStats.value?.breakdown?.length) return []
  const max = Math.max(...usageStats.value.breakdown.map((b) => b.total_tokens), 1)
  return usageStats.value.breakdown.map((b) => ({
    label: new Date(b.period).toLocaleDateString(store.locale === 'en-US' ? 'en-US' : 'zh-CN', { weekday: 'short' }),
    tokens: b.total_tokens, cost: b.cost,
    height: Math.max(b.total_tokens ? 8 : 2, Math.round(b.total_tokens / max * 100)),
  }))
})

const statusLabel = (code: number) => code >= 200 && code < 400 ? t('success') : String(code)
const statusVariant = (code: number) => code >= 200 && code < 400 ? 'secondary' as const : 'destructive' as const

onMounted(() => { if (isAdmin.value) { loadUsageLogs(true); loadUsageStats() } })

// --- Regular user mode: activity logs (from store) ---
const oldSelectClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
const oldInputClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
</script>

<template>
  <!-- Admin: Stats Cards -->
  <template v-if="isAdmin && usageStats">
    <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <Card>
        <CardContent class="pt-6">
          <span class="text-xs text-muted-foreground">{{ t('totalRequestsLabel') }}</span>
          <div class="text-2xl font-semibold">{{ usageStats.total_requests.toLocaleString() }}</div>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('successLabel') }}: {{ usageStats.success_count.toLocaleString() }} · {{ t('errorLabel') }}: {{ usageStats.error_count.toLocaleString() }}</p>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="pt-6">
          <span class="text-xs text-muted-foreground">{{ t('totalTokensLabel') }}</span>
          <div class="text-2xl font-semibold">{{ usageStats.total_tokens.toLocaleString() }}</div>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('promptLabel') }}: {{ usageStats.prompt_tokens.toLocaleString() }} · {{ t('completionLabel') }}: {{ usageStats.completion_tokens.toLocaleString() }}</p>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="pt-6">
          <span class="text-xs text-muted-foreground">{{ t('totalCostLabel') }}</span>
          <div class="text-2xl font-semibold">{{ Number(usageStats.total_cost).toFixed(6) }}</div>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('basedOnCurrentPricing') }}</p>
        </CardContent>
      </Card>
      <Card>
        <CardContent class="pt-6">
          <span class="text-xs text-muted-foreground">{{ t('avgResponseTime') }}</span>
          <div class="text-2xl font-semibold">{{ Math.round(usageStats.avg_duration_ms) }} ms</div>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('averageLatency') }}</p>
        </CardContent>
      </Card>
    </section>

    <!-- Admin: Breakdown Chart -->
    <Card v-if="breakdownTokens.length > 1" class="mt-4">
      <CardContent class="pt-6">
        <div class="mb-4 flex items-center justify-between">
          <div>
            <h2 class="text-sm font-semibold">{{ t('usageTrend') }}</h2>
            <p class="text-xs text-muted-foreground">{{ t('last7DaysTokenAndCost') }}</p>
          </div>
          <div class="flex items-center gap-4 text-xs text-muted-foreground">
            <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-green-600" />{{ t('tokenLabel') }}</span>
            <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-orange-500" />{{ t('costLabel') }}</span>
          </div>
        </div>
        <div class="flex items-end justify-between gap-2">
          <div v-for="day in breakdownTokens" :key="day.label" class="flex flex-1 flex-col items-center gap-2">
            <div class="flex h-40 w-full items-end justify-center gap-1">
              <span class="w-2 rounded-t bg-green-600/80 transition-all" :style="{ height: `${day.height}%` }" :title="`${day.tokens.toLocaleString()} tokens`" />
              <span class="w-2 rounded-t bg-orange-500/80 transition-all" :style="{ height: `${Math.max(day.cost ? 8 : 2, 0)}%` }" :title="`${t('costLabel')} ${day.cost.toFixed(6)}`" />
            </div>
            <b class="text-xs font-medium">{{ day.label }}</b>
            <small class="font-mono text-xs text-muted-foreground">{{ day.tokens ? day.tokens.toLocaleString() : '-' }}</small>
          </div>
        </div>
      </CardContent>
    </Card>
  </template>

  <!-- Admin: Filters -->
  <Card v-if="isAdmin" class="mt-4">
    <CardContent class="pt-6">
      <form class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4" @submit.prevent="applyAdminFilters">
        <div v-if="can('users.read')" class="flex flex-col gap-2">
          <Label>{{ t('userLabel') }}</Label>
          <select v-model="adminFilters.user_id" :class="selectClass">
            <option value="">{{ t('allUsers') }}</option>
            <option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('modelLabel') }}</Label>
          <input v-model="adminFilters.model" :placeholder="t('allModels')" :class="inputClass">
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('groupLabel') }}</Label>
          <select v-model="adminFilters.group_id" :class="selectClass">
            <option value="">{{ t('allGroups') }}</option>
            <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('statusLabel') }}</Label>
          <select v-model="adminFilters.status" :class="selectClass">
            <option value="">{{ t('allStatuses') }}</option>
            <option value="success">{{ t('success') }}</option>
            <option value="error">{{ t('errorLabel') }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('startTime') }}</Label>
          <input v-model="adminFilters.start" type="datetime-local" :class="inputClass">
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('endTime') }}</Label>
          <input v-model="adminFilters.end" type="datetime-local" :class="inputClass">
        </div>
        <div class="flex items-end gap-2">
          <Button type="submit" :disabled="busy">{{ t('filterLabel') }}</Button>
          <Button type="button" variant="outline" :disabled="busy" @click="resetAdminFilters">{{ t('resetFiltersLabel') }}</Button>
        </div>
      </form>
    </CardContent>
  </Card>

  <!-- Admin: Logs Table -->
  <section v-if="isAdmin" class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <div class="flex items-center justify-between border-b border-border px-4 py-3">
      <div>
        <h2 class="text-sm font-semibold">{{ t('usageLogs') }}</h2>
        <p class="text-xs text-muted-foreground">{{ t('usageLogsDesc') }} <span class="ml-2">({{ t('totalLabel') }}: {{ total.toLocaleString() }})</span></p>
      </div>
      <div v-if="totalPages > 1" class="flex items-center gap-1 text-sm">
        <Button variant="outline" size="icon" :disabled="page <= 1" @click="prevPage"><ChevronLeft class="h-4 w-4" /></Button>
        <span class="mx-2 text-xs text-muted-foreground">{{ page }} / {{ totalPages }}</span>
        <Button variant="outline" size="icon" :disabled="page >= totalPages" @click="nextPage"><ChevronRight class="h-4 w-4" /></Button>
      </div>
    </div>
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('createdAt') }}</TableHead>
          <TableHead>{{ t('userLabel') }}</TableHead>
          <TableHead>{{ t('modelLabel') }}</TableHead>
          <TableHead>{{ t('groupLabel') }}</TableHead>
          <TableHead>{{ t('statusLabel') }}</TableHead>
          <TableHead>{{ t('tokens') }}</TableHead>
          <TableHead>{{ t('durationMs') }}</TableHead>
          <TableHead>{{ t('costLabel') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="item in usageLogs" :key="item.request_id">
          <TableCell class="text-xs whitespace-nowrap">{{ formatDate(item.created_at) }}</TableCell>
          <TableCell class="text-xs">{{ item.user_name || item.user_id?.slice(0, 8) || '-' }}</TableCell>
          <TableCell><code class="font-mono text-xs">{{ item.model }}</code></TableCell>
          <TableCell class="text-xs">{{ item.group_name || '-' }}</TableCell>
          <TableCell><Badge :variant="statusVariant(item.status_code)" class="text-xs">{{ statusLabel(item.status_code) }}</Badge></TableCell>
          <TableCell class="font-mono text-xs">{{ item.total_tokens ?? '-' }}</TableCell>
          <TableCell class="font-mono text-xs">{{ item.duration_ms }} ms</TableCell>
          <TableCell class="font-mono text-xs">{{ Number(item.cost).toFixed(6) }}</TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!usageLogs.length" :text="t('noMatchingLogs')" />
  </section>

  <!-- Regular user: Activity logs (original view) -->
  <template v-if="!isAdmin">
    <Card>
      <CardContent class="pt-6">
        <form class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4" @submit.prevent="filterActivity">
          <div v-if="can('users.read')" class="flex flex-col gap-2">
            <Label>{{ t('userLabel') }}</Label>
            <select v-model="activityFilters.user_id" :class="oldSelectClass">
              <option value="">{{ t('allUsers') }}</option>
              <option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option>
            </select>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('modelLabel') }}</Label>
            <select v-model="activityFilters.model" :class="oldSelectClass">
              <option value="">{{ t('allModels') }}</option>
              <option v-for="m in activityModels" :key="m" :value="m">{{ m }}</option>
            </select>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('groupLabel') }}</Label>
            <select v-model="activityFilters.group_id" :class="oldSelectClass">
              <option value="">{{ t('allGroups') }}</option>
              <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('typeLabel') }}</Label>
            <select v-model="activityFilters.type" :class="oldSelectClass">
              <option value="">{{ t('allTypes') }}</option>
              <option value="request">{{ activityTypeLabel['request'] }}</option>
              <option value="login">{{ activityTypeLabel['login'] }}</option>
              <option value="register">{{ activityTypeLabel['register'] }}</option>
              <option value="logout">{{ activityTypeLabel['logout'] }}</option>
              <option value="topup">{{ activityTypeLabel['topup'] }}</option>
              <option value="operation">{{ activityTypeLabel['operation'] }}</option>
            </select>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('startTime') }}</Label>
            <input v-model="activityFilters.start" type="datetime-local" :class="oldInputClass">
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('endTime') }}</Label>
            <input v-model="activityFilters.end" type="datetime-local" :class="oldInputClass">
          </div>
          <div class="flex items-end gap-2">
            <Button type="submit" :disabled="busy">{{ t('filterLabel') }}</Button>
            <Button type="button" variant="outline" :disabled="busy" @click="resetOldFilters">{{ t('resetFiltersLabel') }}</Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
      <div class="border-b border-border px-4 py-3">
        <h2 class="text-sm font-semibold">{{ t('usageLogs') }}</h2>
        <p class="text-xs text-muted-foreground">{{ t('usageLogsDesc') }}</p>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('createdAt') }}</TableHead>
            <TableHead>{{ t('typeLabel') }}</TableHead>
            <TableHead>{{ t('userLabel') }}</TableHead>
            <TableHead>{{ t('modelLabel') }} / Action</TableHead>
            <TableHead>{{ t('groupLabel') }}</TableHead>
            <TableHead>Status / Duration</TableHead>
            <TableHead>Usage / Details</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="item in activityLogs" :key="`${item.type}-${item.id}`">
            <TableCell class="text-xs">{{ formatDate(item.created_at) }}</TableCell>
            <TableCell><Badge variant="outline">{{ activityTypeLabel[item.type] }}</Badge></TableCell>
            <TableCell>{{ item.user_name }}</TableCell>
            <TableCell><code v-if="item.model" class="font-mono text-xs">{{ item.model }}</code><span v-else class="text-sm">{{ actionLabel(item) }}</span></TableCell>
            <TableCell>{{ item.group_name || '-' }}</TableCell>
            <TableCell>
              <Badge v-if="item.status_code != null" :variant="item.status_code < 400 ? 'secondary' : 'destructive'">{{ item.status_code }}</Badge>
              <span v-if="item.duration_ms != null" class="ml-1 text-xs text-muted-foreground">{{ item.duration_ms }} ms</span>
              <span v-if="item.status_code == null" class="text-sm text-green-600 dark:text-green-500">{{ t('success') }}</span>
            </TableCell>
            <TableCell><code class="font-mono text-xs">{{ activityDetail(item) }}</code></TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <Empty v-if="!activityLogs.length" :text="t('noMatchingLogs')" />
    </section>
  </template>
</template>