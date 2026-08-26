<script setup lang="ts">
import { ShieldAlert } from 'lucide-vue-next'
import { endpoints, type RequestContentAudit } from '~/src/api'
import { formatDateTime, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const allowed = computed(() => can('logs.read'))
const filters = reactive({ model: '', decision: '', request_id: '' })
const page = ref(1)
const pageSize = ref('50')
const options = ['20', '50', '100', '200'].map(value => ({ value, label: value }))
function query() {
  const params = new URLSearchParams({ page: String(page.value), page_size: pageSize.value })
  if (filters.model.trim()) params.set('model', filters.model.trim())
  if (filters.decision) params.set('decision', filters.decision)
  if (filters.request_id.trim()) params.set('request_id', filters.request_id.trim())
  return `?${params.toString()}`
}
const audits = useResource(() => endpoints.getRequestAudits(query()), { data: [] as RequestContentAudit[], total: 0, page: 1, page_size: 50 })
const totalPages = computed(() => Math.max(1, Math.ceil(audits.data.value.total / Math.max(1, audits.data.value.page_size))))
const decisionOptions = computed(() => [{ value: '', label: t('common.all') }, { value: 'allow', label: t('system.requestAuditAllow') }, { value: 'audit', label: t('system.requestAuditAudit') }, { value: 'block', label: t('system.requestAuditBlock') }])
async function apply() { page.value = 1; await audits.refresh() }
async function move(next: number) { if (next < 1 || next > totalPages.value) return; page.value = next; await audits.refresh() }
watch(pageSize, apply)
const tone = (decision: RequestContentAudit['decision']): 'success' | 'warn' | 'danger' => decision === 'block' ? 'danger' : decision === 'audit' ? 'warn' : 'success'
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />
  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('system.requestAuditLead')">
      <template #actions><UiButton variant="secondary" size="sm" @click="audits.refresh()">{{ t('common.refresh') }}</UiButton></template>
    </ConsoleOpsPageHeader>
    <UiCard :title="t('common.filter')">
      <div class="grid gap-3 sm:grid-cols-3">
        <UiField :label="t('admin.model')"><UiInput v-model="filters.model" mono /></UiField>
        <UiField :label="t('system.requestAuditDecision')"><UiSelect v-model="filters.decision" :options="decisionOptions" :placeholder="t('common.selectPlaceholder')" /></UiField>
        <UiField :label="t('system.requestAuditRequestId')"><UiInput v-model="filters.request_id" mono /></UiField>
      </div>
      <template #footer><div class="flex justify-end"><UiButton size="sm" @click="apply">{{ t('common.filter') }}</UiButton></div></template>
    </UiCard>
    <ConsoleOpsListState :pending="audits.pending.value" :error="audits.error.value" :empty="!audits.data.value.data.length" :empty-icon="ShieldAlert" :empty-title="t('system.requestAuditEmptyTitle')" :empty-description="t('system.requestAuditEmptyBody')">
      <UiTable dense>
        <thead><tr><th>{{ t('system.requestAuditTime') }}</th><th>{{ t('system.requestAuditRequestId') }}</th><th>{{ t('admin.model') }}</th><th>{{ t('system.requestAuditEndpoint') }}</th><th>{{ t('system.requestAuditDecision') }}</th><th class="num">{{ t('system.requestAuditBytes') }}</th><th>{{ t('common.detail') }}</th></tr></thead>
        <tbody><tr v-for="audit in audits.data.value.data" :key="audit.id"><td class="text-muted whitespace-nowrap">{{ formatDateTime(audit.created_at) }}</td><td class="font-mono text-muted">{{ shortId(audit.request_id) }}</td><td class="font-medium text-ink">{{ audit.model }}</td><td class="font-mono text-muted">{{ audit.endpoint }}</td><td><UiBadge :tone="tone(audit.decision)">{{ audit.decision === 'block' ? t('system.requestAuditBlock') : audit.decision === 'audit' ? t('system.requestAuditAudit') : t('system.requestAuditAllow') }}</UiBadge></td><td class="num">{{ audit.request_bytes }}</td><td><UiTooltip :content="audit.excerpt || audit.content_hash || t('common.none')"><span class="text-muted">{{ audit.excerpt || audit.content_hash ? t('common.detail') : t('common.none') }}</span></UiTooltip></td></tr></tbody>
      </UiTable>
      <div class="flex flex-wrap items-center justify-between gap-3 pt-3"><p class="text-[13px] text-muted">{{ t('common.totalItems', { total: audits.data.value.total }) }}</p><div class="flex items-center gap-2"><UiSelect v-model="pageSize" :options="options" :placeholder="t('common.selectPlaceholder')" class="w-20" /><UiButton variant="secondary" size="sm" :disabled="audits.data.value.page <= 1" @click="move(audits.data.value.page - 1)">{{ t('common.prev') }}</UiButton><span class="numeric text-[13px] text-muted">{{ t('admin.pageOf', { page: audits.data.value.page, pages: totalPages }) }}</span><UiButton variant="secondary" size="sm" :disabled="audits.data.value.page >= totalPages" @click="move(audits.data.value.page + 1)">{{ t('common.next') }}</UiButton></div></div>
    </ConsoleOpsListState>
  </div>
</template>
