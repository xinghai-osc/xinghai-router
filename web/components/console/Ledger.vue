<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, ledger, formatDate } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('balanceLedger') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('ledgerDesc') }}</p>
    </div>
  </section>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('createdAt') }}</TableHead>
          <TableHead>{{ t('typeLabel') }}</TableHead>
          <TableHead>Change</TableHead>
          <TableHead>{{ t('balanceLabel') }}</TableHead>
          <TableHead>Note</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="item in ledger" :key="item.id">
          <TableCell>{{ formatDate(item.created_at) }}</TableCell>
          <TableCell><Badge variant="outline">{{ item.kind }}</Badge></TableCell>
          <TableCell :class="item.amount < 0 ? 'text-destructive' : 'text-green-600 dark:text-green-500'">{{ item.amount }}</TableCell>
          <TableCell>{{ item.balance_after }}</TableCell>
          <TableCell class="text-xs text-muted-foreground">{{ item.note || item.request_id }}</TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!ledger.length" :text="t('noLedgerEntries')" />
  </section>
</template>
