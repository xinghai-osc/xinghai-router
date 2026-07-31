<script setup lang="ts">
import { Receipt } from 'lucide-vue-next'
import { endpoints, type LedgerEntry } from '~/src/api'
import { formatDateTime, formatMoney, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const KIND_LABELS: Record<string, string> = {
  topup: 'console.kindTopup',
  reservation: 'console.kindReservation',
  charge: 'console.kindCharge',
  release: 'console.kindRelease',
  refund: 'console.kindRefund',
  adjustment: 'console.kindAdjustment',
}

const { t } = useI18n()
const { settings } = useSiteSettings()

useHead({ title: () => `${t('nav.ledger')} · ${settings.value.name}` })

const { data: ledger, pending, error } = useResource(
  () => endpoints.getAccountLedger(),
  { data: [] as LedgerEntry[] },
)

const entries = computed(() => ledger.value.data.map(entry => ({
  entry,
  amount: Number(entry.amount ?? 0),
  // Kinds outside the schema vocabulary are shown verbatim rather than mistranslated.
  kindLabel: KIND_LABELS[entry.kind] ? t(KIND_LABELS[entry.kind]) : entry.kind,
})))

/** Keeps the sign outside the currency symbol: "-¥1.2500" rather than "¥-1.2500". */
function signedMoney(value: number): string {
  if (value === 0) return formatMoney(0, 4)
  return `${value < 0 ? '-' : '+'}${formatMoney(Math.abs(value), 4)}`
}

const totals = computed(() => entries.value.reduce(
  (sum, row) => ({
    credits: sum.credits + (row.amount > 0 ? row.amount : 0),
    debits: sum.debits + (row.amount < 0 ? -row.amount : 0),
  }),
  { credits: 0, debits: 0 },
))
</script>

<template>
  <div class="space-y-4">
    <div class="grid gap-4 sm:grid-cols-2">
      <ConsoleUserStatCard
        :label="t('console.totalCredits')"
        :value="formatMoney(totals.credits)"
        :loading="pending"
      />
      <ConsoleUserStatCard
        :label="t('console.totalDebits')"
        :value="formatMoney(totals.debits)"
        :loading="pending"
      />
    </div>

    <UiCard :title="t('nav.ledger')" :description="t('console.ledgerDescription')" flush>
      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="pending"
          :error="error"
          :empty="!entries.length"
          :rows="6"
          :empty-icon="Receipt"
          :empty-title="t('console.ledgerEmptyTitle')"
          :empty-description="t('console.ledgerEmptyBody')"
        >
          <UiTable>
            <thead>
              <tr>
                <th class="num">{{ t('console.amount') }}</th>
                <th class="num">{{ t('console.balanceAfter') }}</th>
                <th>{{ t('console.kind') }}</th>
                <th>{{ t('console.relatedRequest') }}</th>
                <th>{{ t('console.note') }}</th>
                <th>{{ t('console.time') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="row in entries" :key="row.entry.id">
                <td :class="['num font-medium', row.amount < 0 ? 'text-danger' : 'text-success']">
                  {{ signedMoney(row.amount) }}
                </td>
                <td class="num text-muted">{{ formatMoney(row.entry.balance_after, 4) }}</td>
                <td>
                  <UiBadge :tone="row.amount < 0 ? 'danger' : 'success'">{{ row.kindLabel }}</UiBadge>
                </td>
                <td>
                  <code v-if="row.entry.request_id" class="font-mono text-[13px] text-muted">
                    {{ shortId(row.entry.request_id) }}
                  </code>
                  <span v-else class="text-faint">{{ t('common.none') }}</span>
                </td>
                <td class="max-w-64 truncate text-muted">{{ row.entry.note || '—' }}</td>
                <td class="text-muted">{{ formatDateTime(row.entry.created_at) }}</td>
              </tr>
            </tbody>
          </UiTable>
        </ConsoleUserDataState>
      </div>
    </UiCard>
  </div>
</template>
