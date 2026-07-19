<script setup lang="ts">
import { useConsoleStore } from '~/composables/useConsoleStore'
import Empty from '~/components/console/Empty.vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

const store = useConsoleStore()
const { t, users, groups, activityLogs, activityModels, activityFilters, activityTypeLabel, busy, can, personalTokens, personalCost, personalRequests, usageChart, formatDate, actionLabel, activityDetail, loadActivity, resetActivityFilters } = store

const selectClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
const inputClass = 'flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring'
</script>

<template>
  <div class="grid gap-4 sm:grid-cols-3">
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('last7DaysTokens') }}</span>
        <div class="text-2xl font-semibold">{{ personalTokens.toLocaleString() }}</div>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('inputOutputTotal') }}</p>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('last7DaysCost') }}</span>
        <div class="text-2xl font-semibold">{{ personalCost.toFixed(6) }}</div>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('basedOnCurrentPricing') }}</p>
      </CardContent>
    </Card>
    <Card>
      <CardContent class="pt-6">
        <span class="text-xs text-muted-foreground">{{ t('callCount') }}</span>
        <div class="text-2xl font-semibold">{{ personalRequests }}</div>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('recent100UsageRecords') }}</p>
      </CardContent>
    </Card>
  </div>

  <Card class="mt-4">
    <CardContent class="pt-6">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 class="text-sm font-semibold">{{ t('usageTrend') }}</h2>
          <p class="text-xs text-muted-foreground">{{ t('last7DaysTokenAndCost') }}</p>
        </div>
        <div class="flex items-center gap-4 text-xs text-muted-foreground">
          <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-green-600" />{{ t('tokenLabel') }}</span>
          <span class="flex items-center gap-1"><span class="h-2 w-2 rounded-full bg-orange-500" />{{ t('costLabel') }}</span>
        </div>
      </div>
      <div class="flex items-end justify-between gap-2">
        <div v-for="day in usageChart" :key="day.key" class="flex flex-1 flex-col items-center gap-2">
          <div class="flex h-40 w-full items-end justify-center gap-1">
            <span class="w-2 rounded-t bg-green-600/80 transition-all" :style="{ height: `${day.tokenHeight}%` }" :title="`${day.tokens.toLocaleString()} tokens`" />
            <span class="w-2 rounded-t bg-orange-500/80 transition-all" :style="{ height: `${day.costHeight}%` }" :title="`${t('costLabel')} ${day.cost.toFixed(6)}`" />
          </div>
          <b class="text-xs font-medium">{{ day.label }}</b>
          <small class="font-mono text-xs text-muted-foreground">{{ day.tokens ? day.tokens.toLocaleString() : '-' }}</small>
        </div>
      </div>
    </CardContent>
  </Card>

  <Card class="mt-4">
    <CardContent class="pt-6">
      <form class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4" @submit.prevent="loadActivity(true)">
        <div v-if="can('users.read')" class="flex flex-col gap-2">
          <Label>{{ t('userLabel') }}</Label>
          <select v-model="activityFilters.user_id" :class="selectClass">
            <option value="">{{ t('allUsers') }}</option>
            <option v-for="user in users" :key="user.id" :value="user.id">{{ user.name }} · {{ user.email }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('modelLabel') }}</Label>
          <select v-model="activityFilters.model" :class="selectClass">
            <option value="">{{ t('allModels') }}</option>
            <option v-for="model in activityModels" :key="model" :value="model">{{ model }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('groupLabel') }}</Label>
          <select v-model="activityFilters.group_id" :class="selectClass">
            <option value="">{{ t('allGroups') }}</option>
            <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('typeLabel') }}</Label>
          <select v-model="activityFilters.type" :class="selectClass">
            <option value="">{{ t('allTypes') }}</option>
            <option value="request">{{ activityTypeLabel['request'] }}</option>
            <option value="login">{{ activityTypeLabel['login'] }}</option>
            <option value="register">{{ activityTypeLabel['register'] }}</option>
            <option value="logout">{{ activityTypeLabel['logout'] }}</option>
            <option value="topup">{{ activityTypeLabel['topup'] }}</option>
            <option value="operation">{{ activityTypeLabel['operation'] }}</option>
          </select>
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('startTime') }}</Label>
          <input v-model="activityFilters.start" type="datetime-local" :class="inputClass">
        </div>
        <div class="flex flex-col gap-2">
          <Label>{{ t('endTime') }}</Label>
          <input v-model="activityFilters.end" type="datetime-local" :class="inputClass">
        </div>
        <div class="flex items-end gap-2">
          <Button type="submit" :disabled="busy">{{ t('filterLabel') }}</Button>
          <Button type="button" variant="outline" :disabled="busy" @click="resetActivityFilters">{{ t('resetFiltersLabel') }}</Button>
        </div>
      </form>
    </CardContent>
  </Card>

  <section class="mt-4 overflow-hidden rounded-lg border border-border bg-card">
    <div class="border-b border-border px-4 py-3">
      <h2 class="text-sm font-semibold">{{ t('usageLogs') }}</h2>
      <p class="text-xs text-muted-foreground">{{ t('usageLogsDesc') }}</p>
    </div>
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{{ t('createdAt') }}</TableHead>
          <TableHead>{{ t('typeLabel') }}</TableHead>
          <TableHead>{{ t('userLabel') }}</TableHead>
          <TableHead>{{ t('modelLabel') }} / Action</TableHead>
          <TableHead>{{ t('groupLabel') }}</TableHead>
          <TableHead>Status / Duration</TableHead>
          <TableHead>Usage / Details</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="item in activityLogs" :key="`${item.type}-${item.id}`">
          <TableCell class="text-xs">{{ formatDate(item.created_at) }}</TableCell>
          <TableCell><Badge variant="outline">{{ activityTypeLabel[item.type] }}</Badge></TableCell>
          <TableCell>{{ item.user_name }}</TableCell>
          <TableCell>
            <code v-if="item.model" class="font-mono text-xs">{{ item.model }}</code>
            <span v-else class="text-sm">{{ actionLabel(item) }}</span>
          </TableCell>
          <TableCell>{{ item.group_name || '-' }}</TableCell>
          <TableCell>
            <Badge v-if="item.status_code != null" :variant="item.status_code < 400 ? 'secondary' : 'destructive'">{{ item.status_code }}</Badge>
            <span v-if="item.duration_ms != null" class="ml-1 text-xs text-muted-foreground">{{ item.duration_ms }} ms</span>
            <span v-if="item.status_code == null" class="text-sm text-green-600 dark:text-green-500">{{ t('success') }}</span>
          </TableCell>
          <TableCell><code class="font-mono text-xs">{{ activityDetail(item) }}</code></TableCell>
        </TableRow>
      </TableBody>
    </Table>
    <Empty v-if="!activityLogs.length" :text="t('noMatchingLogs')" />
  </section>
</template>
