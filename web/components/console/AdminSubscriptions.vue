<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, adminSubscriptions, formatDate } = store

const statusBadge = (status: string) => ({ pending: t('subscriptionPending'), active: t('subscriptionActive'), expired: t('subscriptionExpired'), cancelled: t('subscriptionCancelled') }[status] ?? status)
const statusVariant = (status: string) => ({ pending: 'outline', active: 'secondary', expired: 'destructive', cancelled: 'destructive' }[status] ?? 'secondary')
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('adminSubscriptions') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('adminSubscriptionsDesc') }}</p>
    </div>
  </section>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('userLabel') }}</TableHead>
          <TableHead>{{ t('planLabel') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
          <TableHead>{{ t('currentPeriod') }}</TableHead>
          <TableHead>{{ t('autoRenew') }}</TableHead>
          <TableHead>{{ t('createdAt') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="sub in adminSubscriptions" :key="sub.id">
          <TableCell>
            <div class="font-medium">{{ sub.user_name }}</div>
            <div class="text-xs text-muted-foreground">{{ sub.email }}</div>
          </TableCell>
          <TableCell>{{ sub.plan_name }}</TableCell>
          <TableCell><Badge :variant="statusVariant(sub.status)">{{ statusBadge(sub.status) }}</Badge></TableCell>
          <TableCell class="text-xs">
            <span v-if="sub.current_period_start">{{ formatDate(sub.current_period_start) }} → {{ formatDate(sub.current_period_end) }}</span>
            <span v-else>—</span>
          </TableCell>
          <TableCell>
            <Badge :variant="sub.auto_renew ? 'secondary' : 'destructive'">{{ sub.auto_renew ? t('autoRenewOn') : t('autoRenewOff') }}</Badge>
          </TableCell>
          <TableCell class="text-xs">{{ formatDate(sub.created_at) }}</TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!adminSubscriptions.length" :text="t('noSubscriptions')" />
  </section>
</template>
