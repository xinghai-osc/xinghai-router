<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, adminSubscriptions, formatDate } = store

const statusBadge = (status: string) => ({ pending: t('subscriptionPending'), active: t('subscriptionActive'), expired: t('subscriptionExpired'), cancelled: t('subscriptionCancelled') }[status] ?? status)
const statusClass = (status: string) => ({ pending: 'warn', active: 'good', expired: 'bad', cancelled: 'bad' }[status] ?? 'good')
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('adminSubscriptions') }}</h2>
      <p>{{ t('adminSubscriptionsDesc') }}</p>
    </div>
  </section>
  <section class="panel table-panel">
    <table>
      <thead><tr><th>{{ t('userLabel') }}</th><th>{{ t('planLabel') }}</th><th>{{ t('accountStatus') }}</th><th>{{ t('currentPeriod') }}</th><th>{{ t('autoRenew') }}</th><th>{{ t('createdAt') }}</th></tr></thead>
      <tbody>
        <tr v-for="sub in adminSubscriptions" :key="sub.id">
          <td><b>{{ sub.user_name }}</b><small>{{ sub.email }}</small></td>
          <td>{{ sub.plan_name }}</td>
          <td><span :class="['state', statusClass(sub.status)]">{{ statusBadge(sub.status) }}</span></td>
          <td><span v-if="sub.current_period_start">{{ formatDate(sub.current_period_start) }} → {{ formatDate(sub.current_period_end) }}</span><span v-else>—</span></td>
          <td><span :class="['state', sub.auto_renew ? 'good' : 'bad']">{{ sub.auto_renew ? t('autoRenewOn') : t('autoRenewOff') }}</span></td>
          <td>{{ formatDate(sub.created_at) }}</td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!adminSubscriptions.length" :text="t('noSubscriptions')" />
  </section>
</template>
