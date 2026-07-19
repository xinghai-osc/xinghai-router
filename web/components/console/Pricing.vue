<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, busy, pricing, pricingForm, newAPIPricingForm, can, savePricing, syncNewAPIPricing } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('modelPricing') }}</h2>
      <p>{{ t('pricingDesc') }}</p>
    </div>
  </section>
  <form v-if="can('pricing.manage')" class="panel pricing-form" @submit.prevent="syncNewAPIPricing">
    <label>{{ t('newapiUrl') }}<input v-model="newAPIPricingForm.base_url" required type="url" placeholder="https://newapi.example.com" /></label>
    <label>{{ t('loginToken') }}<input v-model="newAPIPricingForm.api_key" type="password" :placeholder="t('optional')" /></label>
    <label>{{ t('perQuotaPrice') }}<input v-model.number="newAPIPricingForm.price_per_quota_unit" required type="number" min="0" step="any" /><small>{{ t('newapiPricingHint') }}</small></label>
    <button class="button primary" :disabled="busy">{{ t('syncFromNewapi') }}</button>
  </form>
  <form v-if="can('pricing.manage')" class="panel pricing-form" @submit.prevent="savePricing">
    <label>{{ t('modelLabel') }}<input v-model="pricingForm.model" required placeholder="e.g. kimi-k3" /></label>
    <label>{{ t('inputPrice') }}<input v-model.number="pricingForm.input_per_million" type="number" min="0" step="any" placeholder="0" /></label>
    <label>{{ t('cachedInput') }}<input v-model.number="pricingForm.cached_input_per_million" type="number" min="0" step="any" placeholder="0" /></label>
    <label>{{ t('outputPrice') }}<input v-model.number="pricingForm.output_per_million" type="number" min="0" step="any" placeholder="0" /></label>
    <label>{{ t('multiplierLabel') }}<input v-model.number="pricingForm.multiplier" type="number" min="0.01" step="any" placeholder="1" /></label>
    <button class="button primary">{{ t('saveRule') }}</button>
  </form>
  <section class="panel table-panel">
    <table>
      <thead>
        <tr>
          <th>{{ t('modelLabel') }}</th>
          <th>{{ t('inputPrice') }}</th>
          <th>{{ t('cachedInput') }}</th>
          <th>{{ t('outputPrice') }}</th>
          <th>{{ t('multiplierLabel') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in pricing" :key="item.id">
          <td><code>{{ item.model }}</code></td>
          <td>{{ item.input_per_million }}</td>
          <td>{{ item.cached_input_per_million }}</td>
          <td>{{ item.output_per_million }}</td>
          <td>{{ item.multiplier }}</td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!pricing.length" :text="t('noPricingRules')" />
  </section>
</template>
