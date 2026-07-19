<script setup lang="ts">
import { WalletCards, ReceiptText } from 'lucide-vue-next'
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, account, ledger, payments, paymentMethods, paymentsEnabled, paymentMessage, paymentForm, busy, personalCost, formatDate, openConsole, createPayment } = store
</script>

<template>
  <section class="wallet-hero">
    <div>
      <span>{{ t('availableBalance') }}</span>
      <strong>{{ Number(account?.balance ?? 0).toFixed(4) }}</strong>
      <p>{{ t('balanceForModelCalls') }}</p>
    </div>
    <WalletCards :size="64" />
  </section>
  <div class="metrics wallet-metrics">
    <article>
      <span>{{ t('currentBalance') }}</span>
      <strong>{{ Number(account?.balance ?? 0).toFixed(4) }}</strong>
      <p><WalletCards :size="15" />{{ t('accountAvailableQuota') }}</p>
    </article>
    <article>
      <span>{{ t('reservedAmount') }}</span>
      <strong>{{ Number(account?.reserved ?? 0).toFixed(4) }}</strong>
      <p>{{ t('reservedForConcurrent') }}</p>
    </article>
    <article>
      <span>{{ t('cumulativeSpending') }}</span>
      <strong>{{ personalCost.toFixed(6) }}</strong>
      <p><ReceiptText :size="15" />{{ t('recent100Records') }}</p>
    </article>
  </div>
  <form class="panel payment-form" @submit.prevent="createPayment">
    <div>
      <h2>{{ t('onlineTopup') }}</h2>
      <p>{{ paymentsEnabled ? t('onlineTopupDesc') : t('paymentNotConfigured') }}</p>
      <strong v-if="paymentMessage" class="payment-message">{{ paymentMessage }}</strong>
    </div>
    <label>{{ t('topupAmount') }}<input v-model.number="paymentForm.amount" type="number" min="1" max="100000" step="0.01" required /></label>
    <label>{{ t('paymentMethod') }}
      <select v-model="paymentForm.type" required>
        <option v-for="method in paymentMethods" :key="method.id" :value="method.code">{{ method.name }}</option>
      </select>
    </label>
    <button class="button primary" :disabled="busy || !paymentsEnabled || !paymentForm.type">{{ t('goToPay') }}</button>
  </form>
  <section v-if="payments.length" class="panel table-panel payment-orders">
    <div class="panel-title">
      <div>
        <h2>{{ t('topupOrders') }}</h2>
        <p>{{ t('topupOrdersDesc') }}</p>
      </div>
    </div>
    <table>
      <thead>
        <tr>
          <th>{{ t('createdAt') }}</th>
          <th>{{ t('orderNumber') }}</th>
          <th>{{ t('paymentMethod') }}</th>
          <th>{{ t('topupAmount') }}</th>
          <th>{{ t('accountStatus') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in payments.slice(0, 10)" :key="item.order_no">
          <td>{{ formatDate(item.created_at) }}</td>
          <td><code>{{ item.order_no }}</code></td>
          <td>{{ paymentMethods.find((method) => method.code === item.payment_type)?.name ?? item.payment_type }}</td>
          <td>{{ item.amount }}</td>
          <td><span :class="['state', item.status === 'paid' ? 'good' : 'bad']">{{ item.status }}</span></td>
        </tr>
      </tbody>
    </table>
  </section>
  <section class="panel table-panel">
    <div class="panel-title">
      <div>
        <h2>{{ t('balanceLedger') }}</h2>
        <p>{{ t('ledgerDesc') }}</p>
      </div>
      <button class="text-button" @click="openConsole('ledger')">{{ t('viewAll') }}</button>
    </div>
    <table>
      <thead>
        <tr>
          <th>{{ t('createdAt') }}</th>
          <th>{{ t('typeLabel') }}</th>
          <th>Change</th>
          <th>{{ t('balanceLabel') }}</th>
          <th>Note</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in ledger.slice(0, 10)" :key="item.id">
          <td>{{ formatDate(item.created_at) }}</td>
          <td>{{ item.kind }}</td>
          <td :class="item.amount < 0 ? 'danger' : 'success'">{{ item.amount }}</td>
          <td>{{ item.balance_after }}</td>
          <td>{{ item.note || item.request_id }}</td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!ledger.length" :text="t('noLedgerEntries')" />
  </section>
</template>
