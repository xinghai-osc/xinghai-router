<script setup lang="ts">
import { Plus, RotateCcw, Trash2 } from 'lucide-vue-next'
import {
  endpoints,
  type ResetCard,
  type ResetCardUpdate,
  type SubscriptionPlan,
} from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('admin.resetCardTitle')} · ${settings.value.name}` })

const PAGE_SIZE = 50
const page = ref(1)

const { data: cardsData, pending, error, refresh } = useResource(
  () => endpoints.getResetCards(`?page=${page.value}&page_size=${PAGE_SIZE}`),
  { data: [] as ResetCard[], total: 0, page: 1, page_size: PAGE_SIZE },
)

const { data: plansData } = useResource(
  () => endpoints.getAdminSubscriptionPlans(),
  { data: [] as SubscriptionPlan[] },
)

const planOptions = computed(() => plansData.value.data.map(plan => ({ value: plan.id, label: plan.name })))

const totalPages = computed(() => Math.max(1, Math.ceil(cardsData.value.total / PAGE_SIZE)))

watch(page, () => refresh())

function go(p: number) {
  if (p < 1 || p > totalPages.value || p === page.value) return
  page.value = p
}

const createOpen = ref(false)
const editOpen = ref(false)
const editing = ref<ResetCard | null>(null)
const removing = ref<ResetCard | null>(null)
const issueUser = ref('')
const issueSubscriptions = ref<{ id: string; plan_name: string; status: string }[]>([])

const createForm = reactive({
  subscription_id: '',
  quantity: '1',
  expires_at: '',
  note: '',
})
const createError = ref('')

const subOptions = computed(() => issueSubscriptions.value.map(sub => ({
  value: sub.id,
  label: `${sub.plan_name}${sub.status === 'active' ? '' : ` (${t(`system.status${sub.status.charAt(0).toUpperCase() + sub.status.slice(1)}`)})`}`,
})))

async function openCreate() {
  createOpen.value = true
  createError.value = ''
  createForm.subscription_id = ''
  createForm.quantity = '1'
  createForm.expires_at = ''
  createForm.note = ''
  issueUser.value = ''
  issueSubscriptions.value = []
  if (issueUser.value) await loadIssueSubscriptions()
}

async function loadIssueSubscriptions() {
  issueSubscriptions.value = []
  if (!issueUser.value.trim()) return
  const result = await endpoints.getAdminUserSubscriptions(issueUser.value.trim())
  issueSubscriptions.value = result.data
}

async function submitCreate() {
  createError.value = ''
  if (!createForm.subscription_id) {
    createError.value = t('admin.resetCardSubscriptionRequired')
    return
  }
  const quantity = Number(createForm.quantity.trim())
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000) {
    createError.value = t('admin.resetCardQuantityHint')
    return
  }
  let result: { quantity: number } | null = null
  const ok = await run(async () => {
    result = await endpoints.createResetCards({
      subscription_id: createForm.subscription_id,
      quantity,
      expires_at: createForm.expires_at,
      note: createForm.note.trim(),
    })
  })
  if (!ok || !result) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.resetCardCodesCreated', { count: result.quantity }))
  createOpen.value = false
  page.value = 1
  await refresh()
}

// ---- Batch by plan ----

const batchOpen = ref(false)
const batchForm = reactive({
  plan_id: '',
  status: 'active',
  quantity: '1',
  expires_at: '',
  note: '',
})
const batchError = ref('')

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.resetCardBatchStatusActive') },
  { value: 'inactive', label: t('admin.resetCardBatchStatusInactive') },
  { value: 'all', label: t('admin.resetCardBatchStatusAll') },
])

async function openBatch() {
  batchOpen.value = true
  batchError.value = ''
  batchForm.plan_id = ''
  batchForm.status = 'active'
  batchForm.quantity = '1'
  batchForm.expires_at = ''
  batchForm.note = ''
}

async function submitBatch() {
  batchError.value = ''
  if (!batchForm.plan_id) {
    batchError.value = t('admin.resetCardBatchPlanRequired')
    return
  }
  const quantity = Number(batchForm.quantity.trim())
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000) {
    batchError.value = t('admin.resetCardQuantityHint')
    return
  }
  let result: { subscriptions: number; quantity: number } | null = null
  const ok = await run(async () => {
    result = await endpoints.createResetCardsByPlan({
      plan_id: batchForm.plan_id,
      status: batchForm.status,
      quantity,
      expires_at: batchForm.expires_at,
      note: batchForm.note.trim(),
    })
  })
  if (!ok || !result) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.resetCardBatchSuccess', { subscriptions: result.subscriptions, quantity: result.quantity }))
  batchOpen.value = false
  page.value = 1
  await refresh()
}

const editForm = reactive({
  enabled: true,
  expires_at: '',
  note: '',
})
const editError = ref('')

// ---- Batch to all subscriptions ----

const allOpen = ref(false)
const allConfirmOpen = ref(false)
const allForm = reactive({
  status: 'active',
  quantity: '1',
  expires_at: '',
  note: '',
})
const allError = ref('')

const allStatusLabel = computed(() => {
  const key = `admin.resetCardBatchStatus${allForm.status.charAt(0).toUpperCase()}${allForm.status.slice(1)}`
  return t(key)
})

async function openAll() {
  allOpen.value = true
  allError.value = ''
  allForm.status = 'active'
  allForm.quantity = '1'
  allForm.expires_at = ''
  allForm.note = ''
}

async function submitAll() {
  allError.value = ''
  const quantity = Number(allForm.quantity.trim())
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000) {
    allError.value = t('admin.resetCardQuantityHint')
    return
  }
  allOpen.value = false
  allConfirmOpen.value = true
}

async function confirmAll() {
  allConfirmOpen.value = false
  const quantity = Number(allForm.quantity.trim())
  let result: { subscriptions: number; quantity: number } | null = null
  const ok = await run(async () => {
    result = await endpoints.createResetCardsAll({
      status: allForm.status,
      quantity,
      expires_at: allForm.expires_at,
      note: allForm.note.trim(),
    })
  })
  if (!ok || !result) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.resetCardBatchSuccess', { subscriptions: result.subscriptions, quantity: result.quantity }))
  page.value = 1
  await refresh()
}

function openEdit(card: ResetCard) {
  editing.value = card
  editForm.enabled = card.enabled
  editForm.expires_at = card.expires_at ?? ''
  editForm.note = card.note
  editError.value = ''
  editOpen.value = true
}

async function submitEdit() {
  if (!editing.value) return
  editError.value = ''
  const payload: ResetCardUpdate = {
    enabled: editForm.enabled,
    expires_at: editForm.expires_at || null,
    note: editForm.note.trim(),
  }
  const ok = await run(() => endpoints.updateResetCard(editing.value!.id, payload))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.resetCardUpdated'))
  editOpen.value = false
  await refresh()
}

async function confirmDelete() {
  if (!removing.value) return
  const ok = await run(() => endpoints.deleteResetCard(removing.value!.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.resetCardDeleted'))
  removing.value = null
  await refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('admin.resetCardTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('admin.resetCardLead') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <UiButton variant="secondary" size="sm" @click="openAll">
            <RotateCcw class="size-4" />
            {{ t('admin.resetCardBatchAll') }}
          </UiButton>
          <UiButton variant="secondary" size="sm" @click="openBatch">
            <RotateCcw class="size-4" />
            {{ t('admin.resetCardBatchByPlan') }}
          </UiButton>
          <UiButton size="sm" @click="openCreate">
            <Plus class="size-4" />
            {{ t('admin.resetCardNew') }}
          </UiButton>
        </div>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="cardsData.data.length === 0"
        :empty-icon="RotateCcw"
        :empty-title="t('admin.resetCardEmptyTitle')"
        :empty-description="t('admin.resetCardEmptyBody')"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('admin.resetCardSubscription') }}</th>
              <th>{{ t('admin.resetCardUser') }}</th>
              <th>{{ t('admin.resetCardStatus') }}</th>
              <th class="num">{{ t('admin.resetCardExpiresAt') }}</th>
              <th class="num">{{ t('admin.resetCardUsedAt') }}</th>
              <th class="num">{{ t('common.createdAt') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="card in cardsData.data" :key="card.id">
              <td class="font-medium text-ink">{{ card.plan_name }}</td>
              <td>
                <p class="font-medium text-ink">{{ card.user_email || '—' }}</p>
                <p v-if="card.user_name" class="text-[13px] text-muted">{{ card.user_name }}</p>
              </td>
              <td>
                <UiBadge :tone="card.used_at ? 'neutral' : (card.enabled ? 'success' : 'neutral')" dot>
                  {{ card.used_at ? t('admin.resetCardUsed') : (card.enabled ? t('common.enabled') : t('common.disabled')) }}
                </UiBadge>
              </td>
              <td class="num text-muted">{{ formatDateTime(card.expires_at) }}</td>
              <td class="num text-muted">{{ formatDateTime(card.used_at) }}</td>
              <td class="num text-muted">{{ formatDateTime(card.created_at) }}</td>
              <td>
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEdit(card)">{{ t('common.edit') }}</UiButton>
                  <UiButton v-if="!card.used_at" variant="ghost" size="sm" @click="removing = card">
                    <Trash2 class="size-3.5" />
                  </UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </ConsoleSystemDataState>

      <div v-if="totalPages > 1" class="flex items-center justify-between">
        <p class="text-[13px] text-muted">
          {{ t('admin.pageOf', { page, pages: totalPages }) }}
        </p>
        <div class="flex items-center gap-1">
          <UiButton variant="secondary" size="sm" :disabled="page <= 1" @click="go(page - 1)">
            {{ t('common.prev') }}
          </UiButton>
          <UiButton variant="secondary" size="sm" :disabled="page >= totalPages" @click="go(page + 1)">
            {{ t('common.next') }}
          </UiButton>
        </div>
      </div>

      <UiDialog v-model:open="batchOpen" :title="t('admin.resetCardBatchByPlan')" size="md">
        <div class="space-y-4">
          <UiAlert v-if="batchError" tone="danger" :title="batchError" />
          <UiField :label="t('admin.resetCardBatchPlan')" required>
            <UiSelect
              v-model="batchForm.plan_id"
              :options="planOptions"
              :placeholder="t('common.selectPlaceholder')"
            />
          </UiField>
          <UiField :label="t('admin.resetCardBatchStatus')">
            <UiSelect v-model="batchForm.status" :options="statusOptions" />
          </UiField>
          <UiField :label="t('admin.resetCardQuantity')" :hint="t('admin.resetCardQuantityHint')" required>
            <UiInput v-model="batchForm.quantity" type="number" />
          </UiField>
          <UiField :label="t('admin.resetCardExpiresAt')" :hint="t('admin.resetCardExpiresAtHint')">
            <UiInput v-model="batchForm.expires_at" type="datetime-local" />
          </UiField>
          <UiField :label="t('admin.resetCardNote')">
            <UiTextarea v-model="batchForm.note" :rows="2" />
          </UiField>
        </div>

        <template #footer>
          <UiButton variant="secondary" @click="batchOpen = false">{{ t('admin.resetCardFormCancel') }}</UiButton>
          <UiButton :loading="busy" @click="submitBatch">{{ t('admin.resetCardFormSubmit') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog v-model:open="allOpen" :title="t('admin.resetCardBatchAll')" size="md">
        <div class="space-y-4">
          <UiAlert v-if="allError" tone="danger" :title="allError" />
          <p class="text-[13px] text-muted">{{ t('admin.resetCardBatchAllLead') }}</p>
          <UiField :label="t('admin.resetCardBatchStatus')">
            <UiSelect v-model="allForm.status" :options="statusOptions" />
          </UiField>
          <UiField :label="t('admin.resetCardQuantity')" :hint="t('admin.resetCardQuantityHint')" required>
            <UiInput v-model="allForm.quantity" type="number" />
          </UiField>
          <UiField :label="t('admin.resetCardExpiresAt')" :hint="t('admin.resetCardExpiresAtHint')">
            <UiInput v-model="allForm.expires_at" type="datetime-local" />
          </UiField>
          <UiField :label="t('admin.resetCardNote')">
            <UiTextarea v-model="allForm.note" :rows="2" />
          </UiField>
        </div>

        <template #footer>
          <UiButton variant="secondary" @click="allOpen = false">{{ t('admin.resetCardFormCancel') }}</UiButton>
          <UiButton :loading="busy" @click="submitAll">{{ t('admin.resetCardBatchAllNext') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog :open="allConfirmOpen" size="sm" :title="t('admin.resetCardBatchAllConfirmTitle')">
        <p class="text-sm text-muted">
          {{ t('admin.resetCardBatchAllConfirmBody', { status: allStatusLabel, quantity: allForm.quantity }) }}
        </p>
        <template #footer>
          <UiButton variant="secondary" @click="allConfirmOpen = false">{{ t('admin.resetCardFormCancel') }}</UiButton>
          <UiButton :loading="busy" @click="confirmAll">{{ t('admin.resetCardBatchAllConfirm') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog v-model:open="createOpen" :title="t('admin.resetCardNew')" size="md">
        <div class="space-y-4">
          <UiAlert v-if="createError" tone="danger" :title="createError" />
          <UiField :label="t('admin.resetCardUser')" :hint="t('admin.resetCardUserHint')" required>
            <UiInput
              v-model="issueUser"
              :placeholder="t('admin.resetCardUserPlaceholder')"
              @change="loadIssueSubscriptions"
              @keyup.enter="loadIssueSubscriptions"
            />
          </UiField>
          <UiField :label="t('admin.resetCardSubscription')" :hint="t('admin.resetCardLeadHint')" required>
            <UiSelect
              v-model="createForm.subscription_id"
              :options="subOptions"
              :placeholder="t('common.selectPlaceholder')"
            />
          </UiField>
          <UiField :label="t('admin.resetCardQuantity')" :hint="t('admin.resetCardQuantityHint')" required>
            <UiInput v-model="createForm.quantity" type="number" />
          </UiField>
          <UiField :label="t('admin.resetCardExpiresAt')" :hint="t('admin.resetCardExpiresAtHint')">
            <UiInput v-model="createForm.expires_at" type="datetime-local" />
          </UiField>
          <UiField :label="t('admin.resetCardNote')">
            <UiTextarea v-model="createForm.note" :rows="2" />
          </UiField>
        </div>

        <template #footer>
          <UiButton variant="secondary" @click="createOpen = false">{{ t('admin.resetCardFormCancel') }}</UiButton>
          <UiButton :loading="busy" @click="submitCreate">{{ t('admin.resetCardFormSubmit') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog v-model:open="editOpen" :title="t('admin.resetCardEdit')" size="md">
        <div v-if="editing" class="space-y-4">
          <div class="rounded-control border border-line bg-sunken px-3 py-2.5 text-sm">
            <div class="flex items-center justify-between gap-4">
              <span class="text-muted">{{ t('admin.resetCardSubscription') }}</span>
              <span class="font-medium text-ink">{{ editing.plan_name }}</span>
            </div>
            <div class="mt-1.5 flex items-center justify-between gap-4">
              <span class="text-muted">{{ t('admin.resetCardUser') }}</span>
              <span class="font-medium text-ink">{{ editing.user_email || '—' }}</span>
            </div>
          </div>

          <UiField :label="t('admin.resetCardExpiresAt')" :hint="t('admin.resetCardExpiresAtHint')">
            <UiInput v-model="editForm.expires_at" type="datetime-local" />
          </UiField>

          <UiField :label="t('admin.resetCardNote')">
            <UiTextarea v-model="editForm.note" :rows="2" />
          </UiField>

          <UiField :label="t('admin.resetCardEnabled')">
            <UiSwitch v-model="editForm.enabled" />
          </UiField>

          <UiAlert v-if="editError" tone="danger" :title="editError" />
        </div>

        <template #footer>
          <UiButton variant="secondary" @click="editOpen = false">{{ t('admin.resetCardFormCancel') }}</UiButton>
          <UiButton :loading="busy" @click="submitEdit">{{ t('admin.resetCardFormSubmit') }}</UiButton>
        </template>
      </UiDialog>

      <ConsoleSystemConfirmDialog
        :open="removing !== null"
        :body="t('admin.resetCardDeleteConfirm')"
        :busy="busy"
        @update:open="value => { if (!value) removing = null }"
        @confirm="confirmDelete"
      />
    </div>
  </ConsoleSystemGate>
</template>
