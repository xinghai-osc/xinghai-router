<script setup lang="ts">
import { CalendarCheck } from 'lucide-vue-next'
import { endpoints, type AdminCheckin } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()
const allowed = computed(() => can('users.read'))
const canWithdraw = computed(() => can('wallets.manage'))
const page = ref(1)
const pageSize = ref('50')
const search = ref('')
const target = ref<AdminCheckin | null>(null)

function query() {
  const params = new URLSearchParams({ page: String(page.value), page_size: pageSize.value })
  if (search.value.trim()) params.set('q', search.value.trim())
  return `?${params.toString()}`
}
const records = useResource(() => endpoints.getAdminCheckins(query()), { data: [] as AdminCheckin[], total: 0, page: 1, page_size: 50 })
watch([page, pageSize], () => { void records.refresh() })
watch(search, () => { page.value = 1; void records.refresh() })

async function withdraw() {
  if (!target.value) return
  const record = target.value
  const ok = await run(() => endpoints.withdrawAdminCheckin(record.user_id, record.checkin_date))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  target.value = null
  toast.success(t('admin.checkinWithdrawn'))
  await records.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />
  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.checkinsLead')">
      <template #actions>
        <ConsoleOpsSearch v-model="search" :placeholder="t('admin.checkinsSearchPlaceholder')" />
        <UiButton variant="secondary" size="sm" @click="records.refresh()">{{ t('common.refresh') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>
    <UiAlert v-if="!canWithdraw" tone="info">{{ t('admin.checkinsReadOnlyNotice') }}</UiAlert>
    <ConsoleOpsListState :pending="records.pending.value" :error="records.error.value" :empty="!records.data.value.data.length" :empty-icon="CalendarCheck" :empty-title="t('admin.checkinsEmptyTitle')" :empty-description="t('admin.checkinsEmptyBody')">
      <UiTable v-if="records.data.value.data.length">
        <thead><tr><th>{{ t('admin.email') }}</th><th>{{ t('common.name') }}</th><th>{{ t('admin.checkinDate') }}</th><th class="num">{{ t('admin.checkinStreak') }}</th><th class="num">{{ t('admin.checkinReward') }}</th><th>{{ t('common.createdAt') }}</th><th v-if="canWithdraw">{{ t('common.actions') }}</th></tr></thead>
        <tbody><tr v-for="record in records.data.value.data" :key="`${record.user_id}-${record.checkin_date}`"><td class="font-medium text-ink">{{ record.email }}</td><td class="text-muted">{{ record.user_name }}</td><td>{{ record.checkin_date }}</td><td class="num">{{ record.streak }}</td><td class="num text-success">+{{ formatMoney(record.reward, 4) }}</td><td class="text-muted whitespace-nowrap">{{ formatDateTime(record.created_at) }}</td><td v-if="canWithdraw"><UiButton variant="danger" size="sm" @click="target = record">{{ t('admin.withdrawCheckin') }}</UiButton></td></tr></tbody>
      </UiTable>
      <ConsoleOpsPagination v-model:page="page" v-model:page-size="pageSize" :total="records.data.value.total" :page-size-options="['20', '50', '100']" />
    </ConsoleOpsListState>
    <UiDialog :open="target !== null" size="sm" :title="t('admin.withdrawCheckin')">
      <p class="text-sm text-muted">{{ t('admin.confirmWithdrawCheckin', { reward: target ? formatMoney(target.reward, 4) : '' }) }}</p>
      <template #footer><UiButton variant="secondary" @click="target = null">{{ t('common.cancel') }}</UiButton><UiButton variant="danger" :loading="busy" @click="withdraw">{{ t('admin.withdrawCheckin') }}</UiButton></template>
    </UiDialog>
  </div>
</template>
