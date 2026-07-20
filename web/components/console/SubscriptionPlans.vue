<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import { Sheet, SheetContent, SheetFooter, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, busy, groups, subscriptionPlans, subscriptionPlanForm, editingPlanID, showPlanModal, savePlan, openPlanModal, editPlan, deletePlan } = store

const selectClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('subscriptionPlans') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('subscriptionPlansAdminDesc') }}</p>
    </div>
    <Button @click="openPlanModal">+ {{ t('createPlan') }}</Button>
  </section>

  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('planName') }}</TableHead>
          <TableHead>{{ t('priceLabel') }}</TableHead>
          <TableHead>{{ t('billingPeriod') }}</TableHead>
          <TableHead>{{ t('creditAmount') }}</TableHead>
          <TableHead>{{ t('groupLabel') }}</TableHead>
          <TableHead>{{ t('models') }}</TableHead>
          <TableHead>{{ t('sortOrder') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
          <TableHead />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="plan in subscriptionPlans" :key="plan.id">
          <TableCell>
            <div class="font-medium">{{ plan.name }}</div>
            <div class="text-xs text-muted-foreground">{{ plan.description }}</div>
          </TableCell>
          <TableCell>¥{{ plan.price }} <small class="text-muted-foreground">{{ plan.currency }}</small></TableCell>
          <TableCell><Badge variant="outline">{{ plan.billing_period === 'year' ? t('yearPlan') : t('monthPlan') }}</Badge></TableCell>
          <TableCell>{{ plan.credit_amount }}</TableCell>
          <TableCell>{{ plan.group_name || '—' }}</TableCell>
          <TableCell>{{ plan.model_whitelist.length ? plan.model_whitelist.length : t('all') }}</TableCell>
          <TableCell>{{ plan.sort_order }}</TableCell>
          <TableCell><Badge :variant="plan.enabled ? 'secondary' : 'destructive'">{{ plan.enabled ? t('enabled') : t('disabled') }}</Badge></TableCell>
          <TableCell class="text-right">
            <Button variant="link" size="sm" @click="editPlan(plan)">{{ t('edit') }}</Button>
            <Button variant="link" size="sm" class="text-destructive" @click="deletePlan(plan)">{{ t('remove') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!subscriptionPlans.length" :text="t('noSubscriptionPlans')" />
  </section>

  <Sheet :open="showPlanModal" @update:open="v => !v && (showPlanModal = false)">
    <SheetContent side="right" class="w-full sm:max-w-lg">
      <SheetHeader>
        <SheetTitle>{{ editingPlanID ? t('editPlan') : t('createPlan') }}</SheetTitle>
      </SheetHeader>
      <form class="flex flex-1 flex-col overflow-y-auto px-6" @submit.prevent="savePlan">
        <div class="flex flex-col gap-5">
          <div class="flex flex-col gap-2">
            <Label>{{ t('planName') }}</Label>
            <Input v-model="subscriptionPlanForm.name" required maxlength="100" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('planDescription') }}</Label>
            <Input v-model="subscriptionPlanForm.description" maxlength="500" />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-2">
              <Label>{{ t('priceLabel') }}</Label>
              <Input v-model="subscriptionPlanForm.price" required />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('currency') }}</Label>
              <Input v-model="subscriptionPlanForm.currency" required maxlength="8" />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('billingPeriod') }}</Label>
              <select v-model="subscriptionPlanForm.billing_period" :class="selectClass">
                <option value="month">{{ t('monthPlan') }}</option>
                <option value="year">{{ t('yearPlan') }}</option>
              </select>
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('creditAmount') }}</Label>
              <Input v-model="subscriptionPlanForm.credit_amount" type="number" min="0" step="any" />
              <p class="text-xs text-muted-foreground">{{ t('creditAmountHint') }}</p>
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('groupLabel') }}</Label>
              <select v-model="subscriptionPlanForm.group_id" :class="selectClass">
                <option value="">{{ t('none') }}</option>
                <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
              </select>
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('sortOrder') }}</Label>
              <Input v-model.number="subscriptionPlanForm.sort_order" type="number" min="0" step="1" />
            </div>
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('models') }} <span class="text-xs text-muted-foreground">{{ t('modelsCommaHint') }}</span></Label>
            <Input v-model="subscriptionPlanForm.model_whitelist" :placeholder="t('modelsPlaceholder')" />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-2">
              <Label>{{ t('maxRequestsPerPeriod') }} <span class="text-xs text-muted-foreground">{{ t('optional') }}</span></Label>
              <Input v-model="subscriptionPlanForm.max_requests_per_period" type="number" min="0" step="1" :placeholder="t('unlimited')" />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('maxTokensPerPeriod') }} <span class="text-xs text-muted-foreground">{{ t('optional') }}</span></Label>
              <Input v-model="subscriptionPlanForm.max_tokens_per_period" type="number" min="0" step="1" :placeholder="t('unlimited')" />
            </div>
          </div>
          <div class="flex items-center gap-2">
            <Checkbox id="plan-enabled" :model-value="subscriptionPlanForm.enabled" @update:model-value="v => subscriptionPlanForm.enabled = !!v" />
            <Label for="plan-enabled">{{ t('enabled') }}</Label>
          </div>
        </div>
        <SheetFooter class="mt-auto px-0 pb-6">
          <Button type="submit" :disabled="busy" class="w-full">{{ t('saveLabel') }}</Button>
        </SheetFooter>
      </form>
    </SheetContent>
  </Sheet>
</template>
