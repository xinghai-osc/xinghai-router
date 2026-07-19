<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, busy, publicPlans, userSubscriptions, subscriptionOrders, paymentMethods, subscriptionMessage, subscribeForm, subscribingPlan, formatDate, loadPublicPlans, openSubscribeModal, confirmSubscribe, cancelSubscription } = store

onMounted(async () => { if (!publicPlans.value.length) await loadPublicPlans() })

const planBadge = (period: string) => period === 'year' ? t('yearPlan') : t('monthPlan')
const statusBadge = (status: string) => ({ pending: t('subscriptionPending'), active: t('subscriptionActive'), expired: t('subscriptionExpired'), cancelled: t('subscriptionCancelled') }[status] ?? status)
const statusClass = (status: string) => ({ pending: 'warn', active: 'good', expired: 'bad', cancelled: 'bad' }[status] ?? 'good')
const planCredit = (plan: { credit_amount: string }) => t('subscriptionCredit').replace('{amount}', plan.credit_amount)
const planGroup = (plan: { group_name: string }) => t('subscriptionGroup').replace('{name}', plan.group_name)
const planModels = (plan: { model_whitelist: string[] }) => t('subscriptionModels').replace('{count}', String(plan.model_whitelist.length))
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('subscriptions') }}</h2>
      <p>{{ t('subscriptionsDesc') }}</p>
    </div>
  </section>
  <strong v-if="subscriptionMessage" class="payment-message">{{ subscriptionMessage }}</strong>

  <section v-if="userSubscriptions.length" class="panel table-panel">
    <div class="panel-title"><div><h2>{{ t('mySubscriptions') }}</h2><p>{{ t('mySubscriptionsDesc') }}</p></div></div>
    <table>
      <thead><tr><th>{{ t('planLabel') }}</th><th>{{ t('accountStatus') }}</th><th>{{ t('currentPeriod') }}</th><th>{{ t('autoRenew') }}</th><th></th></tr></thead>
      <tbody>
        <tr v-for="sub in userSubscriptions" :key="sub.id">
          <td><b>{{ sub.plan_name }}</b><small>{{ sub.price }} {{ t('currency_' + (sub.billing_period === 'year' ? 'year' : 'month')) }}</small></td>
          <td><span :class="['state', statusClass(sub.status)]">{{ statusBadge(sub.status) }}</span></td>
          <td><span v-if="sub.current_period_start">{{ formatDate(sub.current_period_start) }} → {{ formatDate(sub.current_period_end) }}</span><span v-else>—</span></td>
          <td><span :class="['state', sub.auto_renew ? 'good' : 'bad']">{{ sub.auto_renew ? t('autoRenewOn') : t('autoRenewOff') }}</span></td>
          <td><button v-if="sub.status === 'active' || sub.status === 'pending'" class="text-button danger" @click="cancelSubscription(sub)">{{ t('cancelSubscription') }}</button></td>
        </tr>
      </tbody>
    </table>
  </section>

  <section class="panel">
    <div class="panel-title"><div><h2>{{ t('subscriptionPlans') }}</h2><p>{{ t('subscriptionPlansDesc') }}</p></div></div>
    <div v-if="publicPlans.length" class="plan-grid">
      <article v-for="plan in publicPlans" :key="plan.id" class="plan-card">
        <header><h3>{{ plan.name }}</h3><span class="pill">{{ planBadge(plan.billing_period) }}</span></header>
        <div class="plan-price"><strong>¥{{ plan.price }}</strong><small>/ {{ planBadge(plan.billing_period) }}</small></div>
        <p class="plan-desc">{{ plan.description || t('subscriptionPlanNoDesc') }}</p>
        <ul>
          <li v-if="Number(plan.credit_amount) > 0">{{ planCredit(plan) }}</li>
          <li v-if="plan.group_name">{{ planGroup(plan) }}</li>
          <li v-if="plan.model_whitelist.length">{{ planModels(plan) }}</li>
          <li v-else>{{ t('subscriptionAllModels') }}</li>
        </ul>
        <button class="button primary full" :disabled="busy || !paymentMethods.length" @click="openSubscribeModal(plan)">{{ t('subscribe') }}</button>
      </article>
    </div>
    <Empty v-else :text="t('noSubscriptionPlans')" />
  </section>

  <section v-if="subscriptionOrders.length" class="panel table-panel">
    <div class="panel-title"><div><h2>{{ t('subscriptionOrders') }}</h2><p>{{ t('subscriptionOrdersDesc') }}</p></div></div>
    <table>
      <thead><tr><th>{{ t('createdAt') }}</th><th>{{ t('orderNumber') }}</th><th>{{ t('planLabel') }}</th><th>{{ t('amount') }}</th><th>{{ t('periodKind') }}</th><th>{{ t('accountStatus') }}</th></tr></thead>
      <tbody>
        <tr v-for="order in subscriptionOrders" :key="order.id">
          <td>{{ formatDate(order.created_at) }}</td>
          <td><code>{{ order.order_no }}</code></td>
          <td>{{ order.plan_name }}</td>
          <td>¥{{ order.amount }}</td>
          <td><span class="pill">{{ order.period_kind === 'renewal' ? t('periodRenewal') : t('periodNew') }}</span></td>
          <td><span :class="['state', order.status === 'paid' ? 'good' : order.status === 'pending' ? 'warn' : 'bad']">{{ order.status }}</span></td>
        </tr>
      </tbody>
    </table>
  </section>

  <div v-if="subscribingPlan" class="modal-backdrop" @click.self="subscribingPlan = null">
    <form class="modal" @submit.prevent="confirmSubscribe">
      <div class="modal-title"><h2>{{ t('subscribeTitle') }} · {{ subscribingPlan.name }}</h2><button type="button" @click="subscribingPlan = null">×</button></div>
      <p class="muted">{{ t('subscribeDesc') }}</p>
      <label>{{ t('paymentMethod') }}<select v-model="subscribeForm.payment_type" required><option v-for="method in paymentMethods" :key="method.id" :value="method.code">{{ method.name }}</option></select></label>
      <label class="payment-enabled"><input v-model="subscribeForm.auto_renew" type="checkbox" />{{ t('enableAutoRenew') }}</label>
      <button class="button primary full" :disabled="busy">{{ t('goToPay') }}</button>
    </form>
  </div>
</template>

<style scoped>
.plan-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 16px; margin-top: 12px; }
.plan-card { display: flex; flex-direction: column; gap: 10px; padding: 20px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface); }
.plan-card header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.plan-card h3 { margin: 0; font-size: 18px; }
.plan-price { display: flex; align-items: baseline; gap: 4px; }
.plan-price strong { font-size: 28px; }
.plan-price small { color: var(--muted); }
.plan-desc { color: var(--muted); margin: 0; min-height: 32px; }
.plan-card ul { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 6px; font-size: 13px; }
.plan-card li::before { content: '✓ '; color: var(--success); }
.payment-message { display: block; margin-bottom: 12px; padding: 8px 12px; border-radius: 8px; background: var(--surface); border: 1px solid var(--border); }
</style>
