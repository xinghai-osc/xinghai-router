<script setup lang="ts">
import { RefreshCw, WalletCards } from 'lucide-vue-next'
import { endpoints, type LedgerEntry } from '~/src/api'
import { formatDateTime, formatMoney, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.ledger')} · ${settings.value.name}` })

const { data: ledger, pending, error, refresh } = useResource(
  () => endpoints.getAccountLedger(),
  { data: [] as LedgerEntry[] },
)

function kindLabel(kind: string): string {
  const key = `console.kind${kind.charAt(0).toUpperCase()}${kind.slice(1)}`
  const translated = t(key)
  return translated === key ? kind : translated
}

function statusTone(status: LedgerEntry['settlement_status']): 'neutral' | 'success' | 'warn' | 'danger' {
  if (status === 'pending' || status === 'processing') return 'warn'
  if (status === 'failed') return 'danger'
  if (status === 'settled') return 'success'
  return 'neutral'
}

function statusLabel(status: LedgerEntry['settlement_status']): string {
  if (status === 'pending' || status === 'processing') return t('console.settlementPending')
  if (status === 'failed') return t('console.settlementFailed')
  if (status === 'settled') return t('console.settlementSettled')
  return t('console.settlementNotApplicable')
}
</script>

<template>
  <div class="space-y-4">
    <UiCard :title="t('nav.ledger')" :description="t('console.ledgerDescription')" flush>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="refresh">
          <RefreshCw class="size-4" />
          {{ t('common.refresh') }}
        </UiButton>
      </template>

      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="pending"
          :error="error"
          :empty="!ledger.data.length"
          :rows="6"
          :empty-icon="WalletCards"
          :empty-title="t('console.ledgerEmptyTitle')"
          :empty-description="t('console.ledgerEmptyBody')"
        >
          <UiTable>
            <thead>
              <tr>
                <th>{{ t('common.createdAt') }}</th>
                <th>{{ t('console.kind') }}</th>
                <th class="num">{{ t('console.amount') }}</th>
                <th class="num">{{ t('console.balanceAfter') }}</th>
                <th>{{ t('console.settlementStatus') }}</th>
                <th>{{ t('console.relatedRequest') }}</th>
                <th>{{ t('console.note') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="entry in ledger.data" :key="entry.id">
                <td class="text-muted whitespace-nowrap">{{ formatDateTime(entry.created_at) }}</td>
                <td><UiBadge :tone="entry.amount.startsWith('-') ? 'danger' : 'success'">{{ kindLabel(entry.kind) }}</UiBadge></td>
                <td class="num font-medium" :class="entry.amount.startsWith('-') ? 'text-danger' : 'text-success'">{{ formatMoney(entry.amount) }}</td>
                <td class="num">{{ formatMoney(entry.balance_after) }}</td>
                <td><UiBadge :tone="statusTone(entry.settlement_status)" dot>{{ statusLabel(entry.settlement_status) }}</UiBadge></td>
                <td>
                  <code v-if="entry.request_id" class="font-mono text-[12px] text-muted">{{ shortId(entry.request_id) }}</code>
                  <span v-else class="text-faint">{{ t('common.none') }}</span>
                </td>
                <td class="max-w-56 text-muted">{{ entry.note || t('common.none') }}</td>
              </tr>
            </tbody>
          </UiTable>
        </ConsoleUserDataState>
      </div>
    </UiCard>
  </div>
</template>
