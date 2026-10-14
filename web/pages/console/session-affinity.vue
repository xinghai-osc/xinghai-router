<script setup lang="ts">
import { Link2 } from 'lucide-vue-next'
import { endpoints, type SessionAffinity } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { toast } = useToast()
const { can } = useAccount()
const { busy, run } = useAction()

const canManage = computed(() => can('system.manage'))

const filters = reactive({
  session: '',
  user_id: '',
  model: '',
  channel_id: '',
})

const page = ref(1)
const pageSize = ref('50')

function affinityQuery(): string {
  const params = new URLSearchParams()
  if (filters.session.trim()) params.set('session', filters.session.trim())
  if (filters.user_id.trim()) params.set('user_id', filters.user_id.trim())
  if (filters.model.trim()) params.set('model', filters.model.trim())
  if (filters.channel_id.trim()) params.set('channel_id', filters.channel_id.trim())
  params.set('page', String(page.value))
  params.set('page_size', pageSize.value)
  return `?${params.toString()}`
}

const assignments = useResource(() => endpoints.getSessionAffinity(affinityQuery()), {
  data: [] as SessionAffinity[], total: 0, page: 1, page_size: 50,
})

async function applyFilters() {
  if (page.value !== 1) page.value = 1
  else await assignments.refresh()
}

async function resetFilters() {
  filters.session = ''
  filters.user_id = ''
  filters.model = ''
  filters.channel_id = ''
  await applyFilters()
}

watch(page, () => { void assignments.refresh() })
watch(pageSize, applyFilters)

const removeTarget = ref<SessionAffinity | null>(null)
const removeOpen = ref(false)
const clearOpen = ref(false)

function askRemove(row: SessionAffinity) {
  removeTarget.value = row
  removeOpen.value = true
}

async function confirmRemove() {
  const target = removeTarget.value
  if (!target) return
  const ok = await run(() => endpoints.deleteSessionAffinity(target.session, target.user_id))
  if (ok) {
    toast.success(t('admin.sessionAffinityDeleted'))
    removeOpen.value = false
    removeTarget.value = null
    await assignments.refresh()
  } else {
    toast.error(t('common.actionFailed'))
  }
}

async function confirmClear() {
  const ok = await run(async () => {
    const result = await endpoints.clearSessionAffinity()
    toast.success(result.removed > 0 ? t('admin.sessionAffinityCleared', { count: result.removed }) : t('admin.sessionAffinityEmptyTitle'))
  })
  if (ok) {
    clearOpen.value = false
    await assignments.refresh()
  } else {
    toast.error(t('common.actionFailed'))
  }
}
</script>

<template>
  <ConsoleSystemGate permission="logs.read">
    <div class="space-y-4">
      <ConsoleOpsPageHeader :lead="t('admin.sessionAffinityLead')">
        <template #actions>
          <UiButton variant="secondary" size="sm" @click="assignments.refresh()">{{ t('common.refresh') }}</UiButton>
          <UiButton v-if="canManage" variant="danger" size="sm" @click="clearOpen = true">{{ t('admin.sessionAffinityClear') }}</UiButton>
        </template>
      </ConsoleOpsPageHeader>

      <UiCard :title="t('common.filter')" :description="t('admin.sessionAffinityFiltersHint')">
        <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <UiField :label="t('admin.sessionAffinityFilterSession')">
            <UiInput v-model="filters.session" mono />
          </UiField>
          <UiField :label="t('admin.filterUserId')">
            <UiInput v-model="filters.user_id" mono />
          </UiField>
          <UiField :label="t('admin.model')">
            <UiInput v-model="filters.model" mono />
          </UiField>
          <UiField :label="t('admin.sessionAffinityFilterChannel')">
            <UiInput v-model="filters.channel_id" mono />
          </UiField>
        </div>

        <template #footer>
          <div class="flex justify-end gap-2">
            <UiButton variant="secondary" size="sm" @click="resetFilters">{{ t('common.reset') }}</UiButton>
            <UiButton size="sm" @click="applyFilters">{{ t('common.filter') }}</UiButton>
          </div>
        </template>
      </UiCard>

      <ConsoleOpsListState
        :pending="assignments.pending.value"
        :error="assignments.error.value"
        :empty="!assignments.data.value.data.length"
        :empty-icon="Link2"
        :empty-title="t('admin.sessionAffinityEmptyTitle')"
        :empty-description="t('admin.sessionAffinityEmptyBody')"
      >
        <UiTable dense>
          <thead>
            <tr>
              <th>{{ t('admin.sessionAffinitySession') }}</th>
              <th>{{ t('admin.user') }}</th>
              <th>{{ t('admin.group') }}</th>
              <th>{{ t('admin.model') }}</th>
              <th>{{ t('admin.sessionAffinityChannel') }}</th>
              <th>{{ t('admin.sessionAffinityKey') }}</th>
              <th>{{ t('admin.sessionAffinityExpiresAt') }}</th>
              <th v-if="canManage">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in assignments.data.value.data" :key="`${row.session}:${row.user_id}:${row.group_id}:${row.model}`">
              <td class="font-mono text-[13px] text-ink" :title="row.session">
                {{ row.session_prefix }}<span v-if="row.session_length > row.session_prefix.length" class="text-faint">…</span>
              </td>
              <td class="text-muted">{{ row.user_name || row.user_id || '—' }}</td>
              <td class="text-muted">{{ row.group_name || row.group_id || '—' }}</td>
              <td class="font-medium text-ink">{{ row.model }}</td>
              <td class="text-muted">
                <span v-if="row.channel_name">{{ row.channel_name }}</span>
                <span v-else class="font-mono">{{ row.channel_id }}</span>
              </td>
              <td class="text-muted">{{ row.channel_key_name || row.channel_key_id || '—' }}</td>
              <td class="whitespace-nowrap text-muted">{{ formatDateTime(row.expires_at) }}</td>
              <td v-if="canManage">
                <UiButton variant="ghost" size="sm" @click="askRemove(row)">{{ t('admin.sessionAffinityDelete') }}</UiButton>
              </td>
            </tr>
          </tbody>
        </UiTable>

        <ConsoleOpsPagination
          v-model:page="page"
          v-model:page-size="pageSize"
          :total="assignments.data.value.total"
        />
      </ConsoleOpsListState>

      <ConsoleSystemConfirmDialog
        v-model:open="removeOpen"
        :title="t('admin.sessionAffinityDelete')"
        :body="t('admin.sessionAffinityDeleteConfirm')"
        :confirm-label="t('admin.sessionAffinityDelete')"
        :busy="busy"
        @confirm="confirmRemove"
      />

      <ConsoleSystemConfirmDialog
        v-model:open="clearOpen"
        :title="t('admin.sessionAffinityClearTitle')"
        :body="t('admin.sessionAffinityClearConfirm')"
        :confirm-label="t('admin.sessionAffinityClear')"
        :busy="busy"
        @confirm="confirmClear"
      />
    </div>
  </ConsoleSystemGate>
</template>
