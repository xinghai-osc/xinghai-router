<script setup lang="ts">
import { MessageSquareText } from 'lucide-vue-next'
import { endpoints, type ConversationLog, type ConversationLogDetail } from '~/src/api'
import { formatDateTime, shortId } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()

const allowed = computed(() => can('logs.read'))

const filters = reactive({
  user_id: '',
  model: '',
  start: '',
  end: '',
})

const page = ref(1)
const pageSize = ref('50')
const pageSizeOptions = ['20', '50', '100', '200'].map(value => ({ value, label: value }))

function toRfc3339(value: string): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toISOString()
}

function filterParams(): URLSearchParams {
  const params = new URLSearchParams()
  if (filters.user_id.trim()) params.set('user_id', filters.user_id.trim())
  if (filters.model.trim()) params.set('model', filters.model.trim())
  const start = toRfc3339(filters.start)
  if (start) params.set('start', start)
  const end = toRfc3339(filters.end)
  if (end) params.set('end', end)
  return params
}

function logsQuery(): string {
  const params = filterParams()
  params.set('page', String(page.value))
  params.set('page_size', pageSize.value)
  return `?${params.toString()}`
}

const conversations = useResource(() => endpoints.getConversationLogs(logsQuery()), {
  data: [] as ConversationLog[], total: 0, page: 1, page_size: 50,
})

const totalPages = computed(() => Math.max(1, Math.ceil(conversations.data.value.total / Math.max(1, conversations.data.value.page_size))))

async function applyFilters() {
  page.value = 1
  await conversations.refresh()
}

async function resetFilters() {
  filters.user_id = ''
  filters.model = ''
  filters.start = ''
  filters.end = ''
  await applyFilters()
}

async function goToPage(next: number) {
  if (next < 1 || next > totalPages.value) return
  page.value = next
  await conversations.refresh()
}

watch(pageSize, applyFilters)

const statusTone = (code: number): 'danger' | 'warn' | 'success' =>
  (code >= 400 ? 'danger' : code >= 300 ? 'warn' : 'success')

const detailTarget = ref<ConversationLog | null>(null)
const detailOpen = ref(false)
const detail = ref<ConversationLogDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')

async function openDetail(log: ConversationLog) {
  detailTarget.value = log
  detailOpen.value = true
  detail.value = null
  detailError.value = ''
  detailLoading.value = true
  try {
    detail.value = await endpoints.getConversationLogDetail(log.id)
  } catch (cause) {
    detailError.value = cause instanceof Error ? cause.message : t('admin.conversationLoadFailed')
  } finally {
    detailLoading.value = false
  }
}

function prettyJson(value: unknown): string {
  if (value === null || value === undefined) return '—'
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.conversationLead')">
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="conversations.refresh()">{{ t('common.refresh') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <UiCard :title="t('common.filter')">
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <UiField :label="t('admin.filterUserId')">
          <UiInput v-model="filters.user_id" mono />
        </UiField>
        <UiField :label="t('admin.model')">
          <UiInput v-model="filters.model" mono />
        </UiField>
        <UiField :label="t('admin.filterStart')">
          <UiInput v-model="filters.start" type="datetime-local" />
        </UiField>
        <UiField :label="t('admin.filterEnd')">
          <UiInput v-model="filters.end" type="datetime-local" />
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
      :pending="conversations.pending.value"
      :error="conversations.error.value"
      :empty="!conversations.data.value.data.length"
      :empty-icon="MessageSquareText"
      :empty-title="t('admin.conversationEmptyTitle')"
      :empty-description="t('admin.conversationEmptyBody')"
    >
      <UiTable dense>
        <thead>
          <tr>
            <th>{{ t('admin.time') }}</th>
            <th>{{ t('admin.conversationRequestId') }}</th>
            <th>{{ t('admin.user') }}</th>
            <th>{{ t('admin.model') }}</th>
            <th>{{ t('admin.conversationStatusCode') }}</th>
            <th>{{ t('admin.conversationStream') }}</th>
            <th class="num">{{ t('admin.duration') }}</th>
            <th>{{ t('common.detail') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in conversations.data.value.data" :key="log.id">
            <td class="text-muted whitespace-nowrap">{{ formatDateTime(log.created_at) }}</td>
            <td class="font-mono text-[13px] text-muted">{{ shortId(log.request_id) }}</td>
            <td class="text-muted">{{ log.user_id ? shortId(log.user_id) : '—' }}</td>
            <td class="font-medium text-ink">{{ log.model }}</td>
            <td>
              <UiBadge :tone="statusTone(log.status_code)">{{ log.status_code }}</UiBadge>
            </td>
            <td class="text-muted">{{ log.stream ? t('admin.conversationYes') : t('admin.conversationNo') }}</td>
            <td class="num">{{ t('admin.durationMs', { value: log.duration_ms }) }}</td>
            <td>
              <UiButton variant="ghost" size="sm" @click="openDetail(log)">{{ t('common.detail') }}</UiButton>
            </td>
          </tr>
        </tbody>
      </UiTable>

      <div class="flex flex-wrap items-center justify-between gap-3 pt-3">
        <p class="text-[13px] text-muted">{{ t('common.totalItems', { total: conversations.data.value.total }) }}</p>
        <div class="flex items-center gap-2">
          <UiSelect v-model="pageSize" :options="pageSizeOptions" :placeholder="t('common.selectPlaceholder')" class="w-20" />
          <UiButton
            variant="secondary"
            size="sm"
            :disabled="conversations.data.value.page <= 1"
            @click="goToPage(conversations.data.value.page - 1)"
          >
            {{ t('common.prev') }}
          </UiButton>
          <span class="numeric text-[13px] text-muted">
            {{ t('admin.pageOf', { page: conversations.data.value.page, pages: totalPages }) }}
          </span>
          <UiButton
            variant="secondary"
            size="sm"
            :disabled="conversations.data.value.page >= totalPages"
            @click="goToPage(conversations.data.value.page + 1)"
          >
            {{ t('common.next') }}
          </UiButton>
        </div>
      </div>
    </ConsoleOpsListState>

    <UiSlidePanel v-model:open="detailOpen" :title="t('admin.conversationDetailTitle')" size="lg">
      <div v-if="detailLoading" class="space-y-3">
        <UiSkeleton :rows="6" class="h-6" />
      </div>
      <div v-else-if="detailError" class="space-y-3">
        <UiAlert tone="danger" :title="t('admin.conversationLoadFailed')">{{ detailError }}</UiAlert>
      </div>
      <div v-else-if="detail" class="space-y-4 text-sm">
        <div class="grid gap-3 sm:grid-cols-2">
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('admin.conversationRequestId') }}</div>
            <div class="break-all font-mono text-ink">{{ detail.request_id }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('admin.user') }}</div>
            <div class="font-mono text-ink">{{ detail.user_id ? shortId(detail.user_id) : '—' }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('admin.model') }}</div>
            <div class="font-medium text-ink">{{ detail.model }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('admin.conversationStatusCode') }}</div>
            <div>
              <UiBadge :tone="statusTone(detail.status_code)">{{ detail.status_code }}</UiBadge>
            </div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('admin.conversationStream') }}</div>
            <div class="text-muted">{{ detail.stream ? t('admin.conversationYes') : t('admin.conversationNo') }}</div>
          </div>
          <div>
            <div class="mb-1 text-xs text-muted">{{ t('admin.duration') }}</div>
            <div class="numeric text-ink">{{ t('admin.durationMs', { value: detail.duration_ms }) }}</div>
          </div>
        </div>

        <div>
          <div class="mb-1 text-xs text-muted">{{ t('admin.conversationRequestBody') }}</div>
          <pre class="max-h-80 overflow-auto rounded-control bg-sunken p-3 font-mono text-[12px] text-ink whitespace-pre-wrap break-all">{{ prettyJson(detail.request_body) }}</pre>
        </div>

        <div>
          <div class="mb-1 text-xs text-muted">{{ t('admin.conversationResponseBody') }}</div>
          <pre class="max-h-80 overflow-auto rounded-control bg-sunken p-3 font-mono text-[12px] text-ink whitespace-pre-wrap break-all">{{ prettyJson(detail.response_body) }}</pre>
        </div>
      </div>
    </UiSlidePanel>
  </div>
</template>
