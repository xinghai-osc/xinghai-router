<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, busy, pricing, pricingForm, newAPIPricingForm, can, savePricing, syncNewAPIPricing } = store
</script>

<template>
  <section class="flex flex-wrap items-center justify-between gap-4">
    <div>
      <h2 class="text-lg font-semibold">{{ t('modelPricing') }}</h2>
      <p class="text-sm text-muted-foreground">{{ t('pricingDesc') }}</p>
    </div>
  </section>
  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    <Card v-if="can('pricing.manage')">
      <CardHeader>
        <CardTitle>{{ t('syncFromNewapi') }}</CardTitle>
        <CardDescription>{{ t('newapiPricingHint') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="flex flex-col gap-4" @submit.prevent="syncNewAPIPricing">
          <div class="flex flex-col gap-2">
            <Label>{{ t('newapiUrl') }}</Label>
            <Input v-model="newAPIPricingForm.base_url" required type="url" placeholder="https://newapi.example.com" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('loginToken') }}</Label>
            <Input v-model="newAPIPricingForm.api_key" type="password" :placeholder="t('optional')" />
          </div>
          <div class="flex flex-col gap-2">
            <Label>{{ t('perQuotaPrice') }}</Label>
            <Input v-model.number="newAPIPricingForm.price_per_quota_unit" required type="number" min="0" step="any" />
          </div>
          <Button type="submit" :disabled="busy">{{ t('syncFromNewapi') }}</Button>
        </form>
      </CardContent>
    </Card>
    <Card v-if="can('pricing.manage')">
      <CardHeader>
        <CardTitle>{{ t('saveRule') }}</CardTitle>
        <CardDescription>{{ t('pricingDesc') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="flex flex-col gap-4" @submit.prevent="savePricing">
          <div class="flex flex-col gap-2">
            <Label>{{ t('modelLabel') }}</Label>
            <Input v-model="pricingForm.model" required placeholder="e.g. kimi-k3" />
          </div>
          <div class="grid grid-cols-2 gap-4">
            <div class="flex flex-col gap-2">
              <Label>{{ t('inputPrice') }}</Label>
              <Input v-model.number="pricingForm.input_per_million" type="number" min="0" step="any" placeholder="0" />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('cachedInput') }}</Label>
              <Input v-model.number="pricingForm.cached_input_per_million" type="number" min="0" step="any" placeholder="0" />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('outputPrice') }}</Label>
              <Input v-model.number="pricingForm.output_per_million" type="number" min="0" step="any" placeholder="0" />
            </div>
            <div class="flex flex-col gap-2">
              <Label>{{ t('multiplierLabel') }}</Label>
              <Input v-model.number="pricingForm.multiplier" type="number" min="0.01" step="any" placeholder="1" />
            </div>
          </div>
          <Button type="submit">{{ t('saveRule') }}</Button>
        </form>
      </CardContent>
    </Card>
  </div>
  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('modelLabel') }}</TableHead>
          <TableHead>{{ t('inputPrice') }}</TableHead>
          <TableHead>{{ t('cachedInput') }}</TableHead>
          <TableHead>{{ t('outputPrice') }}</TableHead>
          <TableHead>{{ t('multiplierLabel') }}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="item in pricing" :key="item.id">
          <TableCell><code class="font-mono text-xs">{{ item.model }}</code></TableCell>
          <TableCell>{{ item.input_per_million }}</TableCell>
          <TableCell>{{ item.cached_input_per_million }}</TableCell>
          <TableCell>{{ item.output_per_million }}</TableCell>
          <TableCell>{{ item.multiplier }}</TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!pricing.length" :text="t('noPricingRules')" />
  </section>
</template>
