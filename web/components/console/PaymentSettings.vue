<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, busy, paymentSettings, paymentSettingsForm, paymentMethodForm, savePaymentSettings, createPaymentMethod, updatePaymentMethod, deletePaymentMethod } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('paymentSettings') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('paymentSettingsDesc') }}</p>
    </div>
  </section>

  <Card class="mt-4">
    <CardHeader>
      <CardTitle>{{ t('paymentSettings') }}</CardTitle>
    </CardHeader>
    <CardContent>
      <form class="flex flex-col gap-4" @submit.prevent="savePaymentSettings">
        <div class="flex items-center gap-2">
          <Checkbox id="pay-enabled" :model-value="paymentSettingsForm.enabled" @update:model-value="v => paymentSettingsForm.enabled = !!v" />
          <Label for="pay-enabled">{{ t('enableOnlinePayment') }}</Label>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('epayBaseUrl') }}</Label>
          <Input v-model="paymentSettingsForm.base_url" type="url" required placeholder="https://pay.example.com" />
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('publicBaseUrl') }}</Label>
          <Input v-model="paymentSettingsForm.public_base_url" type="url" required placeholder="https://router.example.com" />
        </div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div class="flex flex-col gap-2">
            <Label>{{ t('merchantId') }}</Label>
            <Input v-model="paymentSettingsForm.merchant_id" required />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('merchantKey') }}</Label>
            <Input v-model="paymentSettingsForm.merchant_key" type="password" :required="!paymentSettings.has_merchant_key" :placeholder="paymentSettings.has_merchant_key ? t('leaveBlankUnchanged') : t('requiredField')" autocomplete="new-password" />
          </div>
        </div>
        <Button type="submit" :disabled="busy" class="w-fit">{{ t('saveSettings') }}</Button>
      </form>
    </CardContent>
  </Card>

  <Card class="mt-4">
    <CardHeader>
      <CardTitle>{{ t('addPaymentMethod') }}</CardTitle>
      <CardDescription>{{ t('paymentMethodCodeHint') }}</CardDescription>
    </CardHeader>
    <CardContent>
      <form class="flex flex-col gap-4" @submit.prevent="createPaymentMethod">
        <div class="grid gap-4 sm:grid-cols-2">
          <div class="flex flex-col gap-2">
            <Label>{{ t('paymentMethodCode') }}</Label>
            <Input v-model="paymentMethodForm.code" required maxlength="50" placeholder="alipay" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('paymentMethodName') }}</Label>
            <Input v-model="paymentMethodForm.name" required maxlength="100" placeholder="支付宝" />
          </div>
        </div>
        <div class="flex items-center gap-2">
          <Checkbox id="pm-enabled" :model-value="paymentMethodForm.enabled" @update:model-value="v => paymentMethodForm.enabled = !!v" />
          <Label for="pm-enabled">{{ t('enabled') }}</Label>
        </div>
        <Button type="submit" :disabled="busy" class="w-fit">{{ t('addPaymentMethod') }}</Button>
      </form>
    </CardContent>
  </Card>

  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <div class="border-b border-border px-4 py-3">
      <h2 class="text-sm font-semibold">{{ t('paymentMethods') }}</h2>
      <p class="text-xs text-muted-foreground">{{ t('paymentMethodsDesc') }}</p>
    </div>
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('paymentMethodCode') }}</TableHead>
          <TableHead>{{ t('paymentMethodName') }}</TableHead>
          <TableHead>{{ t('accountStatus') }}</TableHead>
          <TableHead />
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="method in paymentSettings.methods" :key="method.id">
          <TableCell><Input v-model="method.code" maxlength="50" class="h-8 max-w-[180px]" /></TableCell>
          <TableCell><Input v-model="method.name" maxlength="100" class="h-8 max-w-[200px]" /></TableCell>
          <TableCell>
            <div class="flex items-center gap-2">
              <Checkbox :id="`m-${method.id}`" :model-value="method.enabled" @update:model-value="v => method.enabled = !!v" />
              <Badge :variant="method.enabled ? 'secondary' : 'destructive'">{{ method.enabled ? t('enabled') : t('disabled') }}</Badge>
            </div>
          </TableCell>
          <TableCell class="text-right">
            <Button variant="link" size="sm" @click="updatePaymentMethod(method)">{{ t('saveLabel') }}</Button>
            <Button variant="link" size="sm" class="text-destructive" @click="deletePaymentMethod(method)">{{ t('remove') }}</Button>
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!paymentSettings.methods.length" :text="t('noPaymentMethods')" />
  </section>
</template>
