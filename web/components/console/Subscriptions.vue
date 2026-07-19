<script setup lang="ts">
import { onMounted } from 'vue'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, busy, publicPlans, userSubscriptions, subscriptionOrders, paymentMethods, subscriptionMessage, subscribeForm, subscribingPlan, formatDate, loadPublicPlans, openSubscribeModal, confirmSubscribe, cancelSubscription } = store

onMounted(async () => { if (!publicPlans.value.length) await loadPublicPlans() })

const planBadge = (period: string) => period === 'year' ? t('yearPlan') : t('monthPlan')
const statusBadge = (status: string) => ({ pending: t('subscriptionPending'), active: t('subscriptionActive'), expired: t('subscriptionExpired'), cancelled: t('subscriptionCancelled') }[status] ?? status)
const statusVariant = (status: string) => ({ pending: 'outline', active: 'secondary', expired: 'destructive', cancelled: 'destructive' }[status] ?? 'secondary')
const planCredit = (plan: { credit_amount: string }) => t('subscriptionCredit').replace('{amount}', plan.credit_amount)
const planGroup = (plan: { group_name: string }) => t('subscriptionGroup').replace('{name}', plan.group_name)
const planModels = (plan: { model_whitelist: string[] }) => t('subscriptionModels').replace('{count}', String(plan.model_whitelist.length))
const selectClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('subscriptions') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('subscriptionsDesc') }}</p>
    </div>
  </section>

  <div v-if="subscriptionMessage" class="mt-4 rounded-md border border-border bg-card p-3 text-sm font-medium text-primary">{{ subscriptionMessage }}</div>

  <section v-if="userSubscriptions.length" class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <div class="border-b border-border px-4 py-3">
      <h2 class="text-sm font-semibold">{{ t('mySubscriptions') }}</h2>
      <p class="text-xs text-muted-foreground">{{ t('mySubscriptionsDesc') }}</p>
    </div>
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('planLabel') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
          <TableHead>{{ t('currentPeriod') }}</TableHead>
          <TableHead>{{ t('autoRenew') }}</TableHead>
          <TableHead />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="sub in userSubscriptions" :key="sub.id">
          <TableCell>
            <div class="font-medium">{{ sub.plan_name }}</div>
            <div class="text-xs text-muted-foreground">{{ sub.price }} {{ t('currency_' + (sub.billing_period === 'year' ? 'year' : 'month')) }}</div>
          </TableCell>
          <TableCell><Badge :variant="statusVariant(sub.status)">{{ statusBadge(sub.status) }}</Badge></TableCell>
          <TableCell class="text-xs">
            <span v-if="sub.current_period_start">{{ formatDate(sub.current_period_start) }} → {{ formatDate(sub.current_period_end) }}</span>
            <span v-else>—</span>
          </TableCell>
          <TableCell>
            <Badge :variant="sub.auto_renew ? 'secondary' : 'destructive'">{{ sub.auto_renew ? t('autoRenewOn') : t('autoRenewOff') }}</Badge>
          </TableCell>
          <TableCell class="text-right">
            <Button v-if="sub.status === 'active' || sub.status === 'pending'" variant="link" size="sm" class="text-destructive" @click="cancelSubscription(sub)">{{ t('cancelSubscription') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </section>

  <Card class="mt-4">
    <CardContent class="pt-6">
      <div class="mb-4">
        <h2 class="text-sm font-semibold">{{ t('subscriptionPlans') }}</h2>
        <p class="text-xs text-muted-foreground">{{ t('subscriptionPlansDesc') }}</p>
      </div>
      <div v-if="publicPlans.length" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="plan in publicPlans" :key="plan.id" class="flex flex-col gap-3 rounded-lg border border-border bg-card p-5">
          <div class="flex items-center justify-between gap-2">
            <h3 class="text-lg font-semibold">{{ plan.name }}</h3>
            <Badge variant="outline">{{ planBadge(plan.billing_period) }}</Badge>
          </div>
          <div class="flex items-baseline gap-1">
            <span class="text-2xl font-bold">¥{{ plan.price }}</span>
            <span class="text-xs text-muted-foreground">/ {{ planBadge(plan.billing_period) }}</span>
          </div>
          <p class="min-h-[2rem] text-sm text-muted-foreground">{{ plan.description || t('subscriptionPlanNoDesc') }}</p>
          <ul class="flex flex-col gap-1 text-sm">
            <li v-if="Number(plan.credit_amount) > 0" class="flex items-center gap-1"><span class="text-green-600">✓</span>{{ planCredit(plan) }}</li>
            <li v-if="plan.group_name" class="flex items-center gap-1"><span class="text-green-600">✓</span>{{ planGroup(plan) }}</li>
            <li v-if="plan.model_whitelist.length" class="flex items-center gap-1"><span class="text-green-600">✓</span>{{ planModels(plan) }}</li>
            <li v-else class="flex items-center gap-1"><span class="text-green-600">✓</span>{{ t('subscriptionAllModels') }}</li>
          </ul>
          <Button class="mt-auto w-full" :disabled="busy || !paymentMethods.length" @click="openSubscribeModal(plan)">{{ t('subscribe') }}</Button>
        </div>
      </div>
      <Empty v-else :text="t('noSubscriptionPlans')" />
    </CardContent>
  </Card>

  <section v-if="subscriptionOrders.length" class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <div class="border-b border-border px-4 py-3">
      <h2 class="text-sm font-semibold">{{ t('subscriptionOrders') }}</h2>
      <p class="text-xs text-muted-foreground">{{ t('subscriptionOrdersDesc') }}</p>
    </div>
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('createdAt') }}</TableHead>
          <TableHead>{{ t('orderNumber') }}</TableHead>
          <TableHead>{{ t('planLabel') }}</TableHead>
          <TableHead>{{ t('amount') }}</TableHead>
          <TableHead>{{ t('periodKind') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="order in subscriptionOrders" :key="order.id">
          <TableCell class="text-xs">{{ formatDate(order.created_at) }}</TableCell>
          <TableCell><code class="font-mono text-xs">{{ order.order_no }}</code></TableCell>
          <TableCell>{{ order.plan_name }}</TableCell>
          <TableCell>¥{{ order.amount }}</TableCell>
          <TableCell><Badge variant="outline">{{ order.period_kind === 'renewal' ? t('periodRenewal') : t('periodNew') }}</Badge></TableCell>
          <TableCell><Badge :variant="order.status === 'paid' ? 'secondary' : order.status === 'pending' ? 'outline' : 'destructive'">{{ order.status }}</Badge></TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </section>

  <Dialog :open="Boolean(subscribingPlan)" @update:open="v => !v && (subscribingPlan = null)">
    <DialogContent v-if="subscribingPlan" class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('subscribeTitle') }} · {{ subscribingPlan.name }}</DialogTitle>
        <DialogDescription>{{ t('subscribeDesc') }}</DialogDescription>
      </DialogHeader>
      <form class="flex flex-col gap-4" @submit.prevent="confirmSubscribe">
        <div class="flex flex-col gap-2">
          <Label>{{ t('paymentMethod') }}</Label>
          <select v-model="subscribeForm.payment_type" required :class="selectClass">
            <option v-for="method in paymentMethods" :key="method.id" :value="method.code">{{ method.name }}</option>
          </select>
        </div>
        <div class="flex items-center gap-2">
          <Checkbox id="auto-renew" :model-value="subscribeForm.auto_renew" @update:model-value="v => subscribeForm.auto_renew = !!v" />
          <Label for="auto-renew">{{ t('enableAutoRenew') }}</Label>
        </div>
        <DialogFooter>
          <Button type="submit" :disabled="busy" class="w-full">{{ t('goToPay') }}</Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
