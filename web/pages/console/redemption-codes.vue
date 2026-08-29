<script setup lang="ts">
import { Plus, Ticket, Trash2 } from 'lucide-vue-next'
import {
  endpoints,
  type Page,
  type RedemptionCode,
  type RedemptionCodeForm,
  type RedemptionCodeRedemption,
  type RedemptionCodeUpdate,
  type RedemptionRewardType,
  type SubscriptionPlan,
} from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings } = useSiteSettings()
const { toast } = useToast()
const { busy, run } = useAction()

useHead({ title: () => `${t('admin.redemptionTitle')} · ${settings.value.name}` })

const PAGE_SIZE = 50
const page = ref(1)

const { data: codesData, pending, error, refresh } = useResource(
  () => endpoints.getRedemptionCodes(`?page=${page.value}&page_size=${PAGE_SIZE}`),
  { data: [] as RedemptionCode[], total: 0, page: 1, page_size: PAGE_SIZE },
)

const { data: plansData } = useResource(
  () => endpoints.getAdminSubscriptionPlans(),
  { data: [] as SubscriptionPlan[] },
)

const plans = computed(() => plansData.value.data)

const totalPages = computed(() => Math.max(1, Math.ceil(codesData.value.total / PAGE_SIZE)))

watch(page, () => refresh())

function go(p: number) {
  if (p < 1 || p > totalPages.value || p === page.value) return
  page.value = p
}

const createOpen = ref(false)
const generatedCodes = ref<string[]>([])
const resultsOpen = ref(false)
const editOpen = ref(false)
const editing = ref<RedemptionCode | null>(null)
const removing = ref<RedemptionCode | null>(null)
const redemptionsOpen = ref(false)
const redemptionsTarget = ref<RedemptionCode | null>(null)
const redemptions = ref<RedemptionCodeRedemption[]>([])
const redemptionsPending = ref(false)

const REWARD_TONES: Record<RedemptionRewardType, 'clay' | 'neutral'> = {
  balance: 'neutral',
  subscription: 'clay',
}

function rewardLabel(type: RedemptionRewardType): string {
  return type === 'balance'
    ? t('admin.redemptionRewardBalance')
    : t('admin.redemptionRewardSubscription')
}

function resetCreate() {
  form.reward_type = 'balance'
  form.amount = ''
  form.plan_id = ''
  form.period_days = ''
  form.max_uses = '1'
  form.quantity = '1'
  form.expires_at = ''
  form.note = ''
  createError.value = ''
}

const form = reactive({
  reward_type: 'balance' as RedemptionRewardType,
  amount: '',
  plan_id: '',
  period_days: '',
  max_uses: '1',
  quantity: '1',
  expires_at: '',
  note: '',
})
const createError = ref('')

const rewardTypeOptions = computed(() => [
  { value: 'balance', label: t('admin.redemptionFormRewardTypeBalance') },
  { value: 'subscription', label: t('admin.redemptionFormRewardTypeSubscription') },
])

const planOptions = computed(() => plans.value.map(plan => ({ value: plan.id, label: plan.name })))

async function submitCreate() {
  createError.value = ''
  const quantity = Number(form.quantity)
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000) {
    createError.value = t('admin.redemptionQuantityHint')
    return
  }
  const maxUses = Number(form.max_uses)
  if (!Number.isInteger(maxUses) || maxUses < 1) {
    createError.value = t('admin.redemptionMaxUses')
    return
  }
  if (form.reward_type === 'balance') {
    const val = Number(form.amount)
    if (!Number.isFinite(val) || val <= 0 || val > 1000000) {
      createError.value = t('admin.redemptionAmount')
      return
    }
  } else if (!form.plan_id) {
    createError.value = t('admin.redemptionFormPlan')
    return
  }

  const payload: RedemptionCodeForm = {
    reward_type: form.reward_type,
    amount: form.reward_type === 'balance' ? Number(form.amount).toFixed(2) : '',
    plan_id: form.reward_type === 'subscription' ? form.plan_id : '',
    period_days: form.period_days ? Number(form.period_days) : null,
    max_uses: maxUses,
    quantity,
    expires_at: form.expires_at,
    note: form.note.trim(),
  }

  let result: { codes: string[] } | null = null
  const ok = await run(async () => {
    result = await endpoints.createRedemptionCodes(payload)
  })
  if (!ok || !result) {
    toast.error(t('common.actionFailed'))
    return
  }
  generatedCodes.value = result.codes
  createOpen.value = false
  resetCreate()
  page.value = 1
  await refresh()
  resultsOpen.value = true
}

const editForm = reactive({
  enabled: true,
  expires_at: '',
  note: '',
  max_uses: '1',
})
const editError = ref('')

function openEdit(code: RedemptionCode) {
  editing.value = code
  editForm.enabled = code.enabled
  editForm.expires_at = code.expires_at ?? ''
  editForm.note = code.note
  editForm.max_uses = String(code.max_uses)
  editError.value = ''
  editOpen.value = true
}

async function submitEdit() {
  if (!editing.value) return
  editError.value = ''
  const maxUses = Number(editForm.max_uses)
  if (!Number.isInteger(maxUses) || maxUses < 1) {
    editError.value = t('admin.redemptionMaxUses')
    return
  }
  const payload: RedemptionCodeUpdate = {
    enabled: editForm.enabled,
    expires_at: editForm.expires_at || null,
    note: editForm.note.trim(),
    max_uses: maxUses,
  }
  const ok = await run(() => endpoints.updateRedemptionCode(editing.value!.id, payload))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.redemptionUpdated'))
  editOpen.value = false
  await refresh()
}

async function confirmDelete() {
  if (!removing.value) return
  const ok = await run(() => endpoints.deleteRedemptionCode(removing.value!.id))
  if (!ok) {
    toast.error(t('common.actionFailed'))
    return
  }
  toast.success(t('admin.redemptionDeleted'))
  removing.value = null
  await refresh()
}

async function viewRedemptions(code: RedemptionCode) {
  redemptionsTarget.value = code
  redemptionsOpen.value = true
  redemptionsPending.value = true
  redemptions.value = []
  try {
    const res: Page<RedemptionCodeRedemption> = await endpoints.getRedemptionCodeRedemptions(code.id)
    redemptions.value = res.data
  } catch {
    toast.error(t('common.actionFailed'))
  } finally {
    redemptionsPending.value = false
  }
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('admin.redemptionTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('admin.redemptionLead') }}</p>
        </div>
        <UiButton size="sm" @click="createOpen = true">
          <Plus class="size-4" />
          {{ t('admin.redemptionNew') }}
        </UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="pending"
        :error="error"
        :empty="codesData.data.length === 0"
        :empty-icon="Ticket"
        :empty-title="t('admin.redemptionEmptyTitle')"
        :empty-description="t('admin.redemptionEmptyBody')"
      >
        <UiTable>
          <thead>
            <tr>
              <th>{{ t('admin.redemptionCode') }}</th>
              <th>{{ t('admin.redemptionRewardType') }}</th>
              <th class="num">{{ t('admin.redemptionAmount') }}</th>
              <th>{{ t('admin.redemptionPlan') }}</th>
              <th class="num">{{ t('admin.redemptionUsedCount') }}</th>
              <th>{{ t('admin.redemptionStatus') }}</th>
              <th class="num">{{ t('admin.redemptionExpiresAt') }}</th>
              <th class="num">{{ t('common.createdAt') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="code in codesData.data" :key="code.id">
              <td>
                <div class="flex items-center gap-2">
                  <code class="font-mono text-[13px] text-muted">{{ code.code }}</code>
                  <ConsoleUserCopyButton :value="code.code" :label="t('admin.redemptionCopyCode')" size="icon" variant="ghost" />
                </div>
              </td>
              <td>
                <UiBadge :tone="REWARD_TONES[code.reward_type]">{{ rewardLabel(code.reward_type) }}</UiBadge>
              </td>
              <td class="num">
                <span v-if="code.reward_type === 'balance'">{{ formatMoney(code.amount) }}</span>
                <span v-else class="text-muted">—</span>
              </td>
              <td class="text-muted">{{ code.plan_name || '—' }}</td>
              <td class="num">{{ code.used_count }} / {{ code.max_uses }}</td>
              <td>
                <UiBadge :tone="code.enabled ? 'success' : 'neutral'" dot>
                  {{ code.enabled ? t('common.enabled') : t('common.disabled') }}
                </UiBadge>
              </td>
              <td class="num text-muted">{{ formatDateTime(code.expires_at) }}</td>
              <td class="num text-muted">{{ formatDateTime(code.created_at) }}</td>
              <td>
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEdit(code)">{{ t('common.edit') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="viewRedemptions(code)">
                    {{ t('admin.redemptionViewRedemptions') }}
                  </UiButton>
                  <UiButton
                    v-if="code.used_count === 0"
                    variant="ghost"
                    size="sm"
                    @click="removing = code"
                  >
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
          {{ t('admin.pageOf', { page: page, pages: totalPages }) }}
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

      <UiDialog v-model:open="createOpen" :title="t('admin.redemptionNew')" size="md">
        <div class="space-y-4">
          <UiField :label="t('admin.redemptionFormRewardType')" required>
            <UiSelect v-model="form.reward_type" :options="rewardTypeOptions" />
          </UiField>

          <UiField
            v-if="form.reward_type === 'balance'"
            :label="t('admin.redemptionFormAmount')"
            required
          >
            <UiInput v-model="form.amount" type="number" placeholder="100.00">
              <template #leading>
                <span class="text-[13px]">¥</span>
              </template>
            </UiInput>
          </UiField>

          <template v-else>
            <UiField :label="t('admin.redemptionFormPlan')" required>
              <UiSelect v-model="form.plan_id" :options="planOptions" :placeholder="t('common.selectPlaceholder')" />
            </UiField>
            <UiField :label="t('admin.redemptionPeriodDays')" :hint="t('admin.redemptionPeriodDaysHint')">
              <UiInput v-model="form.period_days" type="number" placeholder="30" />
            </UiField>
          </template>

          <div class="grid gap-4 sm:grid-cols-2">
            <UiField :label="t('admin.redemptionMaxUses')" required>
              <UiInput v-model="form.max_uses" type="number" />
            </UiField>
            <UiField :label="t('admin.redemptionQuantity')" :hint="t('admin.redemptionQuantityHint')" required>
              <UiInput v-model="form.quantity" type="number" />
            </UiField>
          </div>

          <UiField :label="t('admin.redemptionExpiresAt')" :hint="t('admin.redemptionExpiresAtHint')">
            <UiInput v-model="form.expires_at" type="datetime-local" />
          </UiField>

          <UiField :label="t('admin.redemptionNote')">
            <UiTextarea v-model="form.note" :rows="2" />
          </UiField>

          <UiAlert v-if="createError" tone="danger" :title="createError" />
        </div>

        <template #footer>
          <UiButton variant="secondary" @click="createOpen = false">{{ t('admin.redemptionFormCancel') }}</UiButton>
          <UiButton :loading="busy" @click="submitCreate">{{ t('admin.redemptionFormSubmit') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog
        v-model:open="resultsOpen"
        :title="t('admin.redemptionCodesCreated', { count: generatedCodes.length })"
        size="lg"
      >
        <div class="space-y-4">
          <div class="flex items-center justify-between gap-3">
            <p class="text-[13px] text-muted">{{ t('admin.redemptionGeneratedHint') }}</p>
            <ConsoleUserCopyButton
              :value="generatedCodes.join('\n')"
              :label="t('admin.redemptionCopyAll')"
              :success-message="t('admin.redemptionCopiedAll')"
            />
          </div>

          <div class="max-h-72 overflow-y-auto rounded-card border border-line">
            <div
              v-for="code in generatedCodes"
              :key="code"
              class="flex items-center justify-between gap-3 border-b border-line px-3 py-1.5 last:border-b-0"
            >
              <code class="min-w-0 flex-1 truncate font-mono text-[13px] text-ink">{{ code }}</code>
              <ConsoleUserCopyButton :value="code" :label="t('admin.redemptionCopyCode')" size="icon" variant="ghost" />
            </div>
          </div>
        </div>

        <template #footer>
          <UiButton @click="resultsOpen = false">{{ t('common.close') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog v-model:open="editOpen" :title="t('admin.redemptionEdit')" size="md">
        <div v-if="editing" class="space-y-4">
          <UiField :label="t('admin.redemptionCode')">
            <code class="font-mono text-[13px] text-muted">{{ editing.code }}</code>
          </UiField>

          <UiField :label="t('admin.redemptionMaxUses')" required>
            <UiInput v-model="editForm.max_uses" type="number" />
          </UiField>

          <UiField :label="t('admin.redemptionExpiresAt')" :hint="t('admin.redemptionExpiresAtHint')">
            <UiInput v-model="editForm.expires_at" type="datetime-local" />
          </UiField>

          <UiField :label="t('admin.redemptionNote')">
            <UiTextarea v-model="editForm.note" :rows="2" />
          </UiField>

          <UiField :label="t('admin.redemptionEnabled')">
            <UiSwitch v-model="editForm.enabled" />
          </UiField>

          <UiAlert v-if="editError" tone="danger" :title="editError" />
        </div>

        <template #footer>
          <UiButton variant="secondary" @click="editOpen = false">{{ t('admin.redemptionFormCancel') }}</UiButton>
          <UiButton :loading="busy" @click="submitEdit">{{ t('admin.redemptionFormSubmit') }}</UiButton>
        </template>
      </UiDialog>

      <UiDialog v-model:open="redemptionsOpen" :title="t('admin.redemptionRedemptions')" size="md">
        <div v-if="redemptionsTarget" class="space-y-3">
          <div class="flex items-center justify-between gap-3">
            <code class="font-mono text-[13px] text-muted">{{ redemptionsTarget.code }}</code>
            <ConsoleUserCopyButton
              :value="redemptionsTarget.code"
              :label="t('admin.redemptionCopyCode')"
            />
          </div>

          <UiSkeleton v-if="redemptionsPending" :rows="4" />

          <UiEmptyState
            v-else-if="!redemptions.length"
            :icon="Ticket"
            :title="t('admin.redemptionRedemptionsEmptyTitle')"
            :description="t('admin.redemptionRedemptionsEmptyBody')"
          />

          <UiTable v-else>
            <thead>
              <tr>
                <th>{{ t('admin.redemptionRedemptionUser') }}</th>
                <th class="num">{{ t('admin.redemptionRedemptionAmount') }}</th>
                <th>{{ t('admin.redemptionRedemptionPlan') }}</th>
                <th class="num">{{ t('admin.redemptionRedemptionTime') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in redemptions" :key="item.id">
                <td>
                  <p class="font-medium text-ink">{{ item.user_email }}</p>
                  <p v-if="item.user_name" class="text-[13px] text-muted">{{ item.user_name }}</p>
                </td>
                <td class="num">{{ item.amount ? formatMoney(item.amount) : '—' }}</td>
                <td class="text-muted">{{ item.plan_name || '—' }}</td>
                <td class="num text-muted">{{ formatDateTime(item.created_at) }}</td>
              </tr>
            </tbody>
          </UiTable>
        </div>
      </UiDialog>

      <ConsoleSystemConfirmDialog
        :open="removing !== null"
        :body="t('admin.redemptionDeleteConfirm')"
        :busy="busy"
        @update:open="value => { if (!value) removing = null }"
        @confirm="confirmDelete"
      />
    </div>
  </ConsoleSystemGate>
</template>
