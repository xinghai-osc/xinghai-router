<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'

const store = useConsoleStore()
const { t, users, groups, activityLogs, activityModels, activityFilters, activityTypeLabel, busy, can, personalTokens, personalCost, personalRequests, usageChart, formatDate, actionLabel, activityDetail, loadActivity, resetActivityFilters } = store
</script>

<template>
  <section class="usage-summary">
    <article>
      <span>{{ t('last7DaysTokens') }}</span>
      <strong>{{ personalTokens.toLocaleString() }}</strong>
      <small>{{ t('inputOutputTotal') }}</small>
    </article>
    <article>
      <span>{{ t('last7DaysCost') }}</span>
      <strong>{{ personalCost.toFixed(6) }}</strong>
      <small>{{ t('basedOnCurrentPricing') }}</small>
    </article>
    <article>
      <span>{{ t('callCount') }}</span>
      <strong>{{ personalRequests }}</strong>
      <small>{{ t('recent100UsageRecords') }}</small>
    </article>
  </section>
  <section class="panel usage-chart">
    <div class="panel-title">
      <div>
        <h2>{{ t('usageTrend') }}</h2>
        <p>{{ t('last7DaysTokenAndCost') }}</p>
      </div>
      <div class="chart-legend">
        <span><i class="token-dot"></i>{{ t('tokenLabel') }}</span>
        <span><i class="cost-dot"></i>{{ t('costLabel') }}</span>
      </div>
    </div>
    <div class="chart-bars">
      <div v-for="day in usageChart" :key="day.key" class="chart-day">
        <div class="chart-values">
          <span :style="{ height: `${day.tokenHeight}%` }" :title="`${day.tokens.toLocaleString()} tokens`"></span>
          <i :style="{ height: `${day.costHeight}%` }" :title="`${t('costLabel')} ${day.cost.toFixed(6)}`"></i>
        </div>
        <b>{{ day.label }}</b>
        <small>{{ day.tokens ? day.tokens.toLocaleString() : '-' }}</small>
      </div>
    </div>
  </section>
  <form class="panel activity-filters" @submit.prevent="loadActivity(true)">
    <label v-if="can('users.read')">{{ t('userLabel') }}
      <select v-model="activityFilters.user_id">
        <option value="">{{ t('allUsers') }}</option>
        <option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option>
      </select>
    </label>
    <label>{{ t('modelLabel') }}
      <select v-model="activityFilters.model">
        <option value="">{{ t('allModels') }}</option>
        <option v-for="model in activityModels" :key="model" :value="model">{{ model }}</option>
      </select>
    </label>
    <label>{{ t('groupLabel') }}
      <select v-model="activityFilters.group_id">
        <option value="">{{ t('allGroups') }}</option>
        <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
      </select>
    </label>
    <label>{{ t('typeLabel') }}
      <select v-model="activityFilters.type">
        <option value="">{{ t('allTypes') }}</option>
        <option value="request">{{ activityTypeLabel['request'] }}</option>
        <option value="login">{{ activityTypeLabel['login'] }}</option>
        <option value="register">{{ activityTypeLabel['register'] }}</option>
        <option value="logout">{{ activityTypeLabel['logout'] }}</option>
        <option value="topup">{{ activityTypeLabel['topup'] }}</option>
        <option value="operation">{{ activityTypeLabel['operation'] }}</option>
      </select>
    </label>
    <label>{{ t('startTime') }}<input v-model="activityFilters.start" type="datetime-local" /></label>
    <label>{{ t('endTime') }}<input v-model="activityFilters.end" type="datetime-local" /></label>
    <div class="activity-filter-actions">
      <button class="button primary" :disabled="busy">{{ t('filterLabel') }}</button>
      <button type="button" class="button ghost" :disabled="busy" @click="resetActivityFilters">{{ t('resetFiltersLabel') }}</button>
    </div>
  </form>
  <section class="panel table-panel">
    <div class="panel-title">
      <div>
        <h2>{{ t('usageLogs') }}</h2>
        <p>{{ t('usageLogsDesc') }}</p>
      </div>
    </div>
    <table>
      <thead>
        <tr>
          <th>{{ t('createdAt') }}</th>
          <th>{{ t('typeLabel') }}</th>
          <th>{{ t('userLabel') }}</th>
          <th>{{ t('modelLabel') }} / Action</th>
          <th>{{ t('groupLabel') }}</th>
          <th>Status / Duration</th>
          <th>Usage / Details</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in activityLogs" :key="`${item.type}-${item.id}`">
          <td>{{ formatDate(item.created_at) }}</td>
          <td><span class="pill">{{ activityTypeLabel[item.type] }}</span></td>
          <td>{{ item.user_name }}</td>
          <td><code v-if="item.model">{{ item.model }}</code><span v-else>{{ actionLabel(item) }}</span></td>
          <td>{{ item.group_name || '-' }}</td>
          <td>
            <span v-if="item.status_code != null" :class="['state', item.status_code < 400 ? 'good' : 'bad']">{{ item.status_code }}</span>
            <small v-if="item.duration_ms != null">{{ item.duration_ms }} ms</small>
            <span v-if="item.status_code == null">{{ t('success') }}</span>
          </td>
          <td><code>{{ activityDetail(item) }}</code></td>
        </tr>
      </tbody>
    </table>
    <Empty v-if="!activityLogs.length" :text="t('noMatchingLogs')" />
  </section>
</template>
