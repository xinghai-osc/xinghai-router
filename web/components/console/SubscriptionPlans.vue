<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, busy, groups, subscriptionPlans, subscriptionPlanForm, editingPlanID, showPlanModal, formatDate, savePlan, openPlanModal, editPlan, deletePlan } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('subscriptionPlans') }}</h2>
      <p>{{ t('subscriptionPlansAdminDesc') }}</p>
    </div>
    <button class="button primary" @click="openPlanModal">+ {{ t('createPlan') }}</button>
  </section>

  <section class="panel table-panel">
    <table>
      <thead><tr><th>{{ t('planName') }}</th><th>{{ t('priceLabel') }}</th><th>{{ t('billingPeriod') }}</th><th>{{ t('creditAmount') }}</th><th>{{ t('groupLabel') }}</th><th>{{ t('models') }}</th><th>{{ t('sortOrder') }}</th><th>{{ t('accountStatus') }}</th><th></th></tr></thead>
      <tbody>
        <tr v-for="plan in subscriptionPlans" :key="plan.id">
          <td><b>{{ plan.name }}</b><small>{{ plan.description }}</small></td>
          <td>¥{{ plan.price }} <small>{{ plan.currency }}</small></td>
          <td><span class="pill">{{ plan.billing_period === 'year' ? t('yearPlan') : t('monthPlan') }}</span></td>
          <td>{{ plan.credit_amount }}</td>
          <td>{{ plan.group_name || '—' }}</td>
          <td>{{ plan.model_whitelist.length ? plan.model_whitelist.length : t('all') }}</td>
          <td>{{ plan.sort_order }}</td>
          <td><span :class="['state', plan.enabled ? 'good' : 'bad']">{{ plan.enabled ? t('enabled') : t('disabled') }}</span></td>
          <td>
            <button class="text-button" @click="editPlan(plan)">{{ t('edit') }}</button>
            <button class="text-button danger" @click="deletePlan(plan)">{{ t('remove') }}</button>
          </td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!subscriptionPlans.length" :text="t('noSubscriptionPlans')" />
  </section>

  <div v-if="showPlanModal" class="modal-backdrop" @click.self="showPlanModal = false">
    <form class="modal" @submit.prevent="savePlan">
      <div class="modal-title"><h2>{{ editingPlanID ? t('editPlan') : t('createPlan') }}</h2><button type="button" @click="showPlanModal = false">×</button></div>
      <label>{{ t('planName') }}<input v-model="subscriptionPlanForm.name" required maxlength="100" /></label>
      <label>{{ t('planDescription') }}<input v-model="subscriptionPlanForm.description" maxlength="500" /></label>
      <label>{{ t('priceLabel') }}<input v-model="subscriptionPlanForm.price" required /></label>
      <label>{{ t('currency') }}<input v-model="subscriptionPlanForm.currency" required maxlength="8" /></label>
      <label>{{ t('billingPeriod') }}<select v-model="subscriptionPlanForm.billing_period"><option value="month">{{ t('monthPlan') }}</option><option value="year">{{ t('yearPlan') }}</option></select></label>
      <label>{{ t('creditAmount') }}<input v-model="subscriptionPlanForm.credit_amount" type="number" min="0" step="any" /><small>{{ t('creditAmountHint') }}</small></label>
      <label>{{ t('groupLabel') }}<select v-model="subscriptionPlanForm.group_id"><option value="">{{ t('none') }}</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
      <label>{{ t('models') }} <small>{{ t('modelsCommaHint') }}</small><input v-model="subscriptionPlanForm.model_whitelist" :placeholder="t('modelsPlaceholder')" /></label>
      <label>{{ t('maxRequestsPerPeriod') }} <small>{{ t('optional') }}</small><input v-model="subscriptionPlanForm.max_requests_per_period" type="number" min="0" step="1" :placeholder="t('unlimited')" /></label>
      <label>{{ t('maxTokensPerPeriod') }} <small>{{ t('optional') }}</small><input v-model="subscriptionPlanForm.max_tokens_per_period" type="number" min="0" step="1" :placeholder="t('unlimited')" /></label>
      <label>{{ t('sortOrder') }}<input v-model.number="subscriptionPlanForm.sort_order" type="number" min="0" step="1" /></label>
      <label class="payment-enabled"><input v-model="subscriptionPlanForm.enabled" type="checkbox" />{{ t('enabled') }}</label>
      <button class="button primary full" :disabled="busy">{{ t('saveLabel') }}</button>
    </form>
  </div>
</template>
