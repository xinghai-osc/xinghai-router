<script setup lang="ts">
import { RefreshCw, WalletCards } from 'lucide-vue-next'
import { endpoints, type AdminLedgerEntry } from '~/src/api'
import { formatDateTime, formatMoney, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { can } = useAccount()
const allowed = computed(() => can('users.read'))

const page = ref(1)
const pageSize = ref('50')
const search = ref('')
const status = ref('')

useHead({ title: () => `${t('nav.walletLedger')} · ${settings.value.name}` })

function query(): string {
  const params = new URLSearchParams({ page: String(page.value), page_size: pageSize.value })
  if (search.value.trim()) params.set('q', search.value.trim())
  if (status.value) params.set('status', status.value)
  return `?${params.toString()}`
}

const ledger = useResource(
  () => endpoints.getAdminWalletLedger(query()),
  { data: [] as AdminLedgerEntry[], total: 0, page: 1, page_size: 50 },
)

watch(page, () => { void ledger.refresh() })
watch(search, () => { page.value = 1; void ledger.refresh() })
watch(status, () => { page.value = 1; void ledger.refresh() })
watch(pageSize, () => { page.value = 1; void ledger.refresh() })

function kindLabel(kind: string): string {
  const key = `console.kind${kind.charAt(0).toUpperCase()}${kind.slice(1)}`
  const translated = t(key)
  return translated === key ? kind : translated
}

function statusTone(value: AdminLedgerEntry['settlement_status']): 'neutral' | 'success' | 'warn' | 'danger' {
  if (value === 'pending' || value === 'processing') return 'warn'
  if (value === 'failed') return 'danger'
  if (value === 'settled') return 'success'
  return 'neutral'
}

function statusLabel(value: AdminLedgerEntry['settlement_status']): string {
  if (value === 'pending' || value === 'processing') return t('console.settlementPending')
  if (value === 'failed') return t('console.settlementFailed')
  if (value === 'settled') return t('console.settlementSettled')
  return t('console.settlementNotApplicable')
}
</script>

<template>
  <UiEmptyState
    v-if="!allowed"
    :icon="WalletCards"
    :title="t('admin.noAccessTitle')"
    :description="t('admin.noAccessBody')"
  />
  <div v-else class="space-y-4">
    <UiCard :title="t('nav.walletLedger')" :description="t('admin.walletLedgerLead')" flush>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="ledger.refresh">
          <RefreshCw class="size-4" />
          {{ t('common.refresh') }}
        </UiButton>
      </template>
      <div class="space-y-4 px-5 py-4">
        <div class="grid gap-3 md:grid-cols-[1fr_12rem]">
          <UiInput v-model="search" :placeholder="t('admin.walletLedgerSearchPlaceholder')" />
          <UiSelect
            v-model="status"
            :options="[
              { value: '', label: t('admin.walletLedgerAllStatuses') },
              { value: 'pending', label: t('console.settlementPending') },
              { value: 'settled', label: t('console.settlementSettled') },
              { value: 'failed', label: t('console.settlementFailed') },
            ]"
            :placeholder="t('admin.walletLedgerFilterStatus')"
          />
        </div>
        <ConsoleOpsListState
          :pending="ledger.pending.value"
          :error="ledger.error.value"
          :empty="!ledger.data.value.data.length"
          :rows="7"
          :empty-icon="WalletCards"
          :empty-title="t('admin.walletLedgerEmptyTitle')"
          :empty-description="t('admin.walletLedgerEmptyBody')"
        >
          <UiTable>
            <thead>
              <tr>
                <th>{{ t('common.createdAt') }}</th>
                <th>{{ t('admin.walletLedgerUser') }}</th>
                <th>{{ t('console.kind') }}</th>
                <th class="num">{{ t('console.amount') }}</th>
                <th class="num">{{ t('console.balanceAfter') }}</th>
                <th>{{ t('admin.walletLedgerStatus') }}</th>
                <th>{{ t('console.relatedRequest') }}</th>
                <th>{{ t('console.note') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in ledger.data.value.data" :key="entry.id">
                <td class="text-muted whitespace-nowrap">{{ formatDateTime(entry.created_at) }}</td>
                <td><div class="font-medium text-ink">{{ entry.user_name }}</div><div class="text-[12px] text-muted">{{ entry.user_email }}</div></td>
                <td><UiBadge :tone="entry.amount.startsWith('-') ? 'danger' : 'success'">{{ kindLabel(entry.kind) }}</UiBadge></td>
                <td class="num font-medium" :class="entry.amount.startsWith('-') ? 'text-danger' : 'text-success'">{{ formatMoney(entry.amount) }}</td>
                <td class="num">{{ formatMoney(entry.balance_after) }}</td>
                <td><UiBadge :tone="statusTone(entry.settlement_status)" dot>{{ statusLabel(entry.settlement_status) }}</UiBadge></td>
                <td><code v-if="entry.request_id" class="font-mono text-[12px] text-muted">{{ shortId(entry.request_id) }}</code><span v-else class="text-faint">{{ t('common.none') }}</span></td>
                <td class="max-w-56 text-muted">{{ entry.note || t('common.none') }}</td>
              </tr>
            </tbody>
          </UiTable>
          <ConsoleOpsPagination v-model:page="page" v-model:pageSize="pageSize" :total="ledger.data.value.total" :page-size-options="['20', '50', '100']" />
        </ConsoleOpsListState>
      </div>
    </UiCard>
  </div>
</template>
