<script setup lang="ts">
import { WalletCards, ReceiptText } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, account, ledger, payments, paymentMethods, paymentsEnabled, paymentMessage, paymentForm, busy, personalCost, formatDate, openConsole, createPayment } = store
</script>

<template>
  <Card class="overflow-hidden">
    <CardContent class="flex items-center justify-between gap-4 pt-6">
      <div>
        <span class="text-xs text-muted-foreground">{{ t('availableBalance') }}</span>
        <div class="text-3xl font-bold tracking-tight">{{ Number(account?.balance ?? 0).toFixed(4) }}</div>
        <p class="text-sm text-muted-foreground">{{ t('balanceForModelCalls') }}</p>
      </div>
      <WalletCards :size="64" class="text-muted-foreground" />
    </CardContent>
  </Card>

  <div class="mt-4 grid gap-4 sm:grid-cols-3">
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('currentBalance') }}</span>
        <div class="text-2xl font-semibold">{{ Number(account?.balance ?? 0).toFixed(4) }}</div>
        <p class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><WalletCards :size="13" />{{ t('accountAvailableQuota') }}</p>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('reservedAmount') }}</span>
        <div class="text-2xl font-semibold">{{ Number(account?.reserved ?? 0).toFixed(4) }}</div>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('reservedForConcurrent') }}</p>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('cumulativeSpending') }}</span>
        <div class="text-2xl font-semibold">{{ personalCost.toFixed(6) }}</div>
        <p class="mt-1 flex items-center gap-1 text-xs text-muted-foreground"><ReceiptText :size="13" />{{ t('recent100Records') }}</p>
      </CardContent>
    </Card>
  </div>

  <Card class="mt-4">
    <CardHeader>
      <CardTitle>{{ t('onlineTopup') }}</CardTitle>
      <CardDescription>{{ paymentsEnabled ? t('onlineTopupDesc') : t('paymentNotConfigured') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <form class="flex flex-col gap-4 sm:flex-row sm:items-end" @submit.prevent="createPayment">
        <div class="flex flex-col gap-2">
          <Label>{{ t('topupAmount') }}</Label>
          <Input v-model.number="paymentForm.amount" type="number" min="1" max="100000" step="0.01" required class="w-40" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('paymentMethod') }}</Label>
          <select v-model="paymentForm.type" required class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring">
            <option v-for="method in paymentMethods" :key="method.id" :value="method.code">{{ method.name }}</option>
          </select>
        </div>
        <Button type="submit" :disabled="busy || !paymentsEnabled || !paymentForm.type">{{ t('goToPay') }}</Button>
      </form>
      <p v-if="paymentMessage" class="mt-3 text-sm font-medium text-primary">{{ paymentMessage }}</p>
    </CardContent>
  </Card>

  <section v-if="payments.length" class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <div class="border-b border-border px-4 py-3">
      <h2 class="text-sm font-semibold">{{ t('topupOrders') }}</h2>
      <p class="text-xs text-muted-foreground">{{ t('topupOrdersDesc') }}</p>
    </div>
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('createdAt') }}</TableHead>
          <TableHead>{{ t('orderNumber') }}</TableHead>
          <TableHead>{{ t('paymentMethod') }}</TableHead>
          <TableHead>{{ t('topupAmount') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="item in payments.slice(0, 10)" :key="item.order_no">
          <TableCell>{{ formatDate(item.created_at) }}</TableCell>
          <TableCell><code class="font-mono text-xs">{{ item.order_no }}</code></TableCell>
          <TableCell>{{ paymentMethods.find((method) => method.code === item.payment_type)?.name ?? item.payment_type }}</TableCell>
          <TableCell>{{ item.amount }}</TableCell>
          <TableCell>
            <Badge :variant="item.status === 'paid' ? 'secondary' : 'destructive'">{{ item.status }}</Badge>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </section>

  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <div class="flex items-center justify-between border-b border-border px-4 py-3">
      <div>
        <h2 class="text-sm font-semibold">{{ t('balanceLedger') }}</h2>
        <p class="text-xs text-muted-foreground">{{ t('ledgerDesc') }}</p>
      </div>
      <Button variant="link" size="sm" @click="openConsole('ledger')">{{ t('viewAll') }}</Button>
    </div>
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
        <TableRow v-for="item in ledger.slice(0, 10)" :key="item.id">
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
