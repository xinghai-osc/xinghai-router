<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, ledger, formatDate } = store
</script>

<template>
  <section class="toolbar">
    <div>
      <h2>{{ t('balanceLedger') }}</h2>
      <p>{{ t('ledgerDesc') }}</p>
    </div>
  </section>
  <section class="panel table-panel">
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
        <tr v-for="item in ledger" :key="item.id">
          <td>{{ formatDate(item.created_at) }}</td>
          <td><span class="pill">{{ item.kind }}</span></td>
          <td :class="item.amount < 0 ? 'danger' : 'success'">{{ item.amount }}</td>
          <td>{{ item.balance_after }}</td>
          <td>{{ item.note || item.request_id }}</td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!ledger.length" :text="t('noLedgerEntries')" />
  </section>
</template>
