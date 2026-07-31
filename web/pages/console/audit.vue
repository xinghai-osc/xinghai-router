<script setup lang="ts">
import { FileClock } from 'lucide-vue-next'
import { endpoints, type AuditLog } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()

const allowed = computed(() => can('audit.read'))

const logs = useResource(() => endpoints.getAuditLogs(), { data: [] as AuditLog[] })

const search = ref('')

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) return logs.data.value.data
  return logs.data.value.data.filter(log =>
    log.action.toLowerCase().includes(term) || log.actor.toLowerCase().includes(term))
})

function clientLabel(log: AuditLog): string {
  const parts = [
    [log.browser, log.browser_version].filter(Boolean).join(' '),
    [log.operating_system, log.operating_system_version].filter(Boolean).join(' '),
  ].filter(Boolean)
  return parts.length ? parts.join(' · ') : '—'
}

function entityLabel(log: AuditLog): string {
  if (!log.entity_type) return '—'
  return log.entity_id ? `${log.entity_type} · ${log.entity_id}` : log.entity_type
}

const detailsOpen = ref(false)
const detailsTarget = ref<AuditLog | null>(null)

const detailsJson = computed(() => {
  const details = detailsTarget.value?.details
  if (!details || !Object.keys(details).length) return ''
  return JSON.stringify(details, null, 2)
})

function openDetails(log: AuditLog) {
  detailsTarget.value = log
  detailsOpen.value = true
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.auditLead')">
      <template #actions>
        <ConsoleOpsSearch v-model="search" :placeholder="t('admin.auditSearchPlaceholder')" />
        <UiButton variant="secondary" size="sm" @click="logs.refresh()">{{ t('common.refresh') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <ConsoleOpsListState
      :pending="logs.pending.value"
      :error="logs.error.value"
      :empty="!logs.data.value.data.length"
      :empty-icon="FileClock"
      :empty-title="t('admin.auditEmptyTitle')"
      :empty-description="t('admin.auditEmptyBody')"
    >
      <div v-if="!filtered.length" class="rounded-card border border-line bg-surface">
        <UiEmptyState :title="t('admin.noResultsTitle')" :description="t('admin.noResultsBody')" />
      </div>

      <UiTable v-else dense>
        <thead>
          <tr>
            <th>{{ t('admin.action') }}</th>
            <th>{{ t('admin.actor') }}</th>
            <th>{{ t('admin.entity') }}</th>
            <th>{{ t('admin.clientIp') }}</th>
            <th>{{ t('admin.client') }}</th>
            <th>{{ t('admin.request') }}</th>
            <th>{{ t('admin.time') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in filtered" :key="log.id">
            <td class="font-medium text-ink">{{ log.action }}</td>
            <td class="text-muted">{{ log.actor || '—' }}</td>
            <td class="font-mono text-[13px] text-muted">{{ entityLabel(log) }}</td>
            <td class="font-mono text-[13px] text-muted">{{ log.client_ip || '—' }}</td>
            <td class="text-muted">{{ clientLabel(log) }}</td>
            <td class="font-mono text-[13px] text-muted">
              {{ log.request_method ? `${log.request_method} ${log.request_path}` : '—' }}
            </td>
            <td class="text-muted whitespace-nowrap">{{ formatDateTime(log.created_at) }}</td>
            <td>
              <UiButton variant="ghost" size="sm" @click="openDetails(log)">{{ t('common.detail') }}</UiButton>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiDialog v-model:open="detailsOpen" :title="t('admin.auditDetails')" :description="detailsTarget?.action">
      <pre
        v-if="detailsJson"
        class="overflow-x-auto rounded-control border border-line bg-sunken px-3 py-2.5 font-mono text-[13px] leading-relaxed text-ink"
      >{{ detailsJson }}</pre>
      <p v-else class="text-sm text-muted">{{ t('admin.noDetails') }}</p>

      <template #footer>
        <UiButton variant="secondary" @click="detailsOpen = false">{{ t('common.close') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
