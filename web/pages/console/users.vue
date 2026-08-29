<script setup lang="ts">
import { Users } from 'lucide-vue-next'
import { endpoints, type AdminUserSubscription, type Group, type SubscriptionPlan, type User, type UserCreate, type UserUpdate } from '~/src/api'
import { formatDateTime, formatMoney, formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('users.read'))
const canManage = computed(() => can('users.manage'))
const canAdjustWallet = computed(() => can('wallets.manage'))

// Must stay in sync with availablePermissions in internal/app/admin.go —
// a permission missing here would be stripped from the user on save.
const PERMISSIONS = [
  { value: 'users.read', labelKey: 'admin.permUsersRead' },
  { value: 'users.manage', labelKey: 'admin.permUsersManage' },
  { value: 'keys.manage', labelKey: 'admin.permKeysManage' },
  { value: 'channels.read', labelKey: 'admin.permChannelsRead' },
  { value: 'channels.manage', labelKey: 'admin.permChannelsManage' },
  { value: 'logs.read', labelKey: 'admin.permLogsRead' },
  { value: 'audit.read', labelKey: 'admin.permAuditRead' },
  { value: 'pricing.read', labelKey: 'admin.permPricingRead' },
  { value: 'pricing.manage', labelKey: 'admin.permPricingManage' },
  { value: 'wallets.manage', labelKey: 'admin.permWalletsManage' },
  { value: 'routes.manage', labelKey: 'admin.permRoutesManage' },
  { value: 'quotas.manage', labelKey: 'admin.permQuotasManage' },
  { value: 'system.manage', labelKey: 'admin.permSystemManage' },
]

const page = ref(1)
const pageSize = ref('50')

function pageQuery(): string {
  const params = new URLSearchParams()
  params.set('page', String(page.value))
  params.set('page_size', pageSize.value)
  const term = search.value.trim()
  if (term) params.set('q', term)
  return `?${params.toString()}`
}

const users = useResource(() => endpoints.getAdminUsers(pageQuery()), { data: [] as User[], total: 0, page: 1, page_size: 50 })

watch(page, () => { void users.refresh() })
watch(pageSize, async () => {
  if (page.value !== 1) page.value = 1
  else await users.refresh()
})
const groups = useResource(() => endpoints.getAdminGroups('?page_size=100'), { data: [] as Group[], total: 0, page: 1, page_size: 100 })

const search = ref('')

watch(search, () => { page.value = 1; void users.refresh() })

const groupOptions = computed(() => groups.data.value.data)
const groupNames = computed(() => new Map(groupOptions.value.map(group => [group.id, group.display_name || group.name])))

const roleOptions = computed(() => [
  { value: 'user', label: t('admin.roleUser') },
  { value: 'operator', label: t('admin.roleOperator') },
  { value: 'admin', label: t('admin.roleAdmin') },
])

const roleTone = (role: string): 'clay' | 'warn' | 'neutral' =>
  (role === 'admin' ? 'clay' : role === 'operator' ? 'warn' : 'neutral')
const roleLabel = (role: string) => roleOptions.value.find(option => option.value === role)?.label ?? role

const dialogOpen = ref(false)
const editing = ref<User | null>(null)
const creating = ref(false)
const formError = ref('')
const form = reactive({
  id: '',
  name: '',
  email: '',
  role: 'user',
  enabled: true,
  password: '',
  balance: '',
  note: '',
  permissions: [] as string[],
  groups: [] as string[],
  leaderboardOptIn: true,
  leaderboardMaskName: false,
  dataUsageEnabled: true,
  maxConcurrency: '',
  inviterId: '',
})

function openCreate() {
  creating.value = true
  editing.value = null
  formError.value = ''
  form.id = ''
  form.name = ''
  form.email = ''
  form.role = 'user'
  form.enabled = true
  form.password = ''
  form.balance = ''
  form.note = ''
  form.permissions = []
  form.groups = []
  form.leaderboardOptIn = true
  form.leaderboardMaskName = false
  form.dataUsageEnabled = true
  form.maxConcurrency = ''
  form.inviterId = ''
  dialogOpen.value = true
}

function openManage(user: User) {
  creating.value = false
  editing.value = user
  formError.value = ''
  form.id = ''
  form.name = user.name
  form.email = user.email
  form.role = user.role
  form.enabled = user.enabled
  form.password = ''
  form.balance = ''
  form.note = ''
  form.permissions = [...user.permissions]
  form.groups = [...user.groups]
  form.leaderboardOptIn = user.leaderboard_opt_in
  form.leaderboardMaskName = user.leaderboard_mask_name
  form.dataUsageEnabled = user.data_usage_enabled
  form.maxConcurrency = user.max_concurrency === null ? '' : String(user.max_concurrency)
  form.inviterId = user.inviter_id ?? ''
  dialogOpen.value = true
}

function togglePermission(permission: string, checked: boolean) {
  const next = new Set(form.permissions)
  if (checked) next.add(permission)
  else next.delete(permission)
  form.permissions = PERMISSIONS.filter(item => next.has(item.value)).map(item => item.value)
}

function buildUpdate(): UserUpdate | null {
  const name = form.name.trim()
  const email = form.email.trim()
  if (!name) { formError.value = t('admin.nameRequired'); return null }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) { formError.value = t('admin.emailRequired'); return null }
  if (form.password && form.password.length < 8) { formError.value = t('admin.passwordTooShort'); return null }

  const rawConcurrency = form.maxConcurrency.trim()
  const maxConcurrency = rawConcurrency === '' ? null : Number(rawConcurrency)
  if (maxConcurrency !== null && (!Number.isInteger(maxConcurrency) || maxConcurrency < 1 || maxConcurrency > 10000)) {
    formError.value = t('admin.userConcurrencyInvalid')
    return null
  }

  const rawInviterId = form.inviterId.trim()
  const inviterId = rawInviterId === '' ? null : Number(rawInviterId)
  if (inviterId !== null && (!Number.isSafeInteger(inviterId) || inviterId <= 0)) {
    formError.value = t('admin.inviterInvalid')
    return null
  }

  const update: UserUpdate = {
    name,
    email,
    role: form.role,
    enabled: form.enabled,
    permissions: form.permissions,
    groups: form.groups,
    leaderboard_opt_in: form.leaderboardOptIn,
    leaderboard_mask_name: form.leaderboardMaskName,
    data_usage_enabled: form.dataUsageEnabled,
    max_concurrency: maxConcurrency,
    inviter_id: inviterId,
  }
  if (form.password) update.password = form.password

  const id = String(form.id ?? '').trim()
  if (id) {
    const value = Number(id)
    if (!Number.isSafeInteger(value) || value <= 0) { formError.value = t('admin.idInvalid'); return null }
    update.id = value
  }

  const balance = String(form.balance ?? '').trim()
  if (balance) {
    const amount = Number(balance)
    if (!Number.isFinite(amount) || amount < 0) { formError.value = t('admin.balanceInvalid'); return null }
    update.balance = amount
    const note = form.note.trim()
    if (note) update.note = note
  } else if (form.note.trim()) {
    formError.value = t('admin.noteNeedsBalance')
    return null
  }

  return update
}

async function save() {
  formError.value = ''
  if (creating.value) {
    const update = buildUpdate()
    if (!update || !update.password) { if (!update?.password) formError.value = t('admin.passwordRequired'); return }
    const create: UserCreate = { name: update.name!, email: update.email!, password: update.password, role: update.role!, enabled: update.enabled!, permissions: update.permissions ?? [], groups: update.groups ?? [] }
    const ok = await run(() => endpoints.createUser(create))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    toast.success(t('admin.userCreated'))
  } else {
    const target = editing.value
    if (!target) return
    const update = buildUpdate()
    if (!update) return
    const ok = await run(() => endpoints.updateUser(target.id, update))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    toast.success(t('admin.userSaved'))
  }
  dialogOpen.value = false
  await users.refresh()
}

const balanceOpen = ref(false)
const balanceUser = ref<User | null>(null)
const balanceForm = reactive({ amount: '', note: '' })
const balanceError = ref('')

function openAdjustBalance(user: User) {
  balanceUser.value = user
  balanceForm.amount = ''
  balanceForm.note = ''
  balanceError.value = ''
  balanceOpen.value = true
}

const parsedAmount = computed(() => {
  const raw = balanceForm.amount.trim()
  if (!raw) return null
  const value = Number(raw)
  if (!Number.isFinite(value) || value === 0 || Math.abs(value) > 1_000_000_000) return null
  return value
})

const projectedBalance = computed(() => {
  const user = balanceUser.value
  if (!user || parsedAmount.value === null) return null
  const next = Number(user.balance) + parsedAmount.value
  return Number.isFinite(next) ? next : null
})

async function submitAdjustBalance() {
  const target = balanceUser.value
  if (!target) return
  balanceError.value = ''
  const amount = parsedAmount.value
  if (amount === null) { balanceError.value = t('admin.adjustAmountInvalid'); return }
  const note = balanceForm.note.trim()
  if (!note || note.length > 500) { balanceError.value = t('admin.adjustNoteRequired'); return }
  const ok = await run(() => endpoints.adjustBalance(target.id, amount, note))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.balanceAdjusted'))
  balanceOpen.value = false
  await users.refresh()
}

const STATUS_KEYS = {
  pending: 'system.statusPending',
  active: 'system.statusActive',
  expired: 'system.statusExpired',
  cancelled: 'system.statusCancelled',
} as const

const STATUS_TONES = {
  pending: 'warn',
  active: 'success',
  expired: 'neutral',
  cancelled: 'danger',
} as const

const subscriptionsOpen = ref(false)
const subsUser = ref<User | null>(null)
const subs = ref<AdminUserSubscription[]>([])
const subsPending = ref(false)
const subsError = ref('')
const subPlans = ref<SubscriptionPlan[]>([])
const editingSub = ref<AdminUserSubscription | null>(null)
const resetQuotaTarget = ref<AdminUserSubscription | null>(null)
const voidTarget = ref<AdminUserSubscription | null>(null)
const deleteTarget = ref<AdminUserSubscription | null>(null)
const issueCardTarget = ref<AdminUserSubscription | null>(null)
const issueCardForm = reactive({ quantity: '1', expires_at: '', note: '' })
const issueCardError = ref('')
const subForm = reactive({
  plan_id: '',
  start_at: '',
  end_at: '',
  auto_renew: false,
  remaining_requests: '' as string | number,
  remaining_credit: '' as string | number,
})
const subFormError = ref('')

const subPlanOptions = computed(() => subPlans.value.map(plan => ({ value: plan.id, label: plan.name })))

function subQuotaLines(sub: AdminUserSubscription): string[] {
  const lines: string[] = []
  if (sub.max_requests_per_period !== null) {
    lines.push(t('system.remainingRequests', { remaining: formatNumber(sub.remaining_requests), max: formatNumber(sub.max_requests_per_period) }))
  }
  if (sub.max_credit_per_period !== null) {
    lines.push(t('system.remainingCredit', { remaining: formatMoney(sub.remaining_credit), max: formatMoney(sub.max_credit_per_period) }))
  }
  for (const quota of sub.model_usage) {
    if (quota.max_requests_per_period !== null) {
      lines.push(t('system.remainingModelRequests', { model: quota.model, remaining: formatNumber(quota.remaining_requests), max: formatNumber(quota.max_requests_per_period) }))
    }
    if (quota.max_credit_per_period !== null) {
      lines.push(t('system.remainingModelCredit', { model: quota.model, remaining: formatMoney(quota.remaining_credit), max: formatMoney(quota.max_credit_per_period) }))
    }
  }
  if (!lines.length) lines.push(t('system.unlimited'))
  return lines
}

function toLocalInput(iso: string | null): string {
  if (!iso) return ''
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function resetSubForm() {
  subForm.plan_id = ''
  subForm.start_at = ''
  subForm.end_at = ''
  subForm.auto_renew = false
  subForm.remaining_requests = ''
  subForm.remaining_credit = ''
  subFormError.value = ''
}

async function loadSubscriptions() {
  if (!subsUser.value) return
  subsPending.value = true
  subsError.value = ''
  try {
    const result = await endpoints.getAdminUserSubscriptions(subsUser.value.id)
    subs.value = result.data
  } catch {
    subsError.value = t('common.actionFailed')
  } finally {
    subsPending.value = false
  }
}

async function openSubscriptions(user: User) {
  subsUser.value = user
  editingSub.value = null
  resetSubForm()
  subscriptionsOpen.value = true
  await loadSubscriptions()
  try {
    const result = await endpoints.getAdminSubscriptionPlans()
    subPlans.value = result.data
  } catch {
    subPlans.value = []
  }
}

function startEditSubscription(sub: AdminUserSubscription) {
  editingSub.value = sub
  subFormError.value = ''
  subForm.plan_id = sub.plan_id
  subForm.start_at = toLocalInput(sub.current_period_start)
  subForm.end_at = toLocalInput(sub.current_period_end)
  subForm.auto_renew = sub.auto_renew
  subForm.remaining_requests = sub.remaining_requests === null ? '' : String(sub.remaining_requests)
  subForm.remaining_credit = sub.remaining_credit === null ? '' : String(sub.remaining_credit)
}

async function saveSubscription() {
  subFormError.value = ''
  if (subForm.start_at && Number.isNaN(new Date(subForm.start_at).getTime())) {
    subFormError.value = t('admin.invalidTime')
    return
  }
  if (subForm.end_at && Number.isNaN(new Date(subForm.end_at).getTime())) {
    subFormError.value = t('admin.invalidTime')
    return
  }
  const startIso = subForm.start_at ? new Date(subForm.start_at).toISOString() : ''
  const endIso = subForm.end_at ? new Date(subForm.end_at).toISOString() : ''
  if (startIso && endIso && new Date(endIso) <= new Date(startIso)) {
    subFormError.value = t('admin.startAfterEnd')
    return
  }
  const target = subsUser.value
  if (!target) return
  if (editingSub.value) {
    const remainingRequests = String(subForm.remaining_requests).trim()
    const remainingCredit = String(subForm.remaining_credit).trim()
    if ((remainingRequests && (!Number.isInteger(Number(remainingRequests)) || Number(remainingRequests) < 0))
      || (remainingCredit && (!Number.isFinite(Number(remainingCredit)) || Number(remainingCredit) < 0))) {
      subFormError.value = t('admin.invalidRemainingQuota')
      return
    }
    const ok = await run(() => endpoints.updateAdminSubscription(editingSub.value!.id, {
      current_period_start: startIso,
      current_period_end: endIso,
      auto_renew: subForm.auto_renew,
      ...(remainingRequests ? { remaining_requests: Number(remainingRequests) } : {}),
      ...(remainingCredit ? { remaining_credit: Number(remainingCredit) } : {}),
    }))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    toast.success(t('admin.subscriptionUpdated'))
  } else {
    if (!subForm.plan_id) { subFormError.value = t('admin.planRequired'); return }
    const ok = await run(() => endpoints.createAdminUserSubscription(target.id, {
      plan_id: subForm.plan_id,
      start_at: startIso,
      end_at: endIso,
      auto_renew: subForm.auto_renew,
    }))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    toast.success(t('admin.subscriptionAdded'))
  }
  editingSub.value = null
  resetSubForm()
  await loadSubscriptions()
}

async function resetSubscriptionQuota() {
  const target = resetQuotaTarget.value
  if (!target) return
  resetQuotaTarget.value = null
  const ok = await run(() => endpoints.resetAdminSubscriptionQuota(target.id))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.subscriptionQuotaReset'))
  await loadSubscriptions()
}

async function voidSubscription() {
  const target = voidTarget.value
  if (!target) return
  voidTarget.value = null
  const ok = await run(() => endpoints.voidAdminSubscription(target.id))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.subscriptionVoided'))
  await loadSubscriptions()
}

async function deleteSubscription() {
  const target = deleteTarget.value
  if (!target) return
  deleteTarget.value = null
  const ok = await run(() => endpoints.deleteAdminSubscription(target.id))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.subscriptionDeleted'))
  await loadSubscriptions()
}

function openIssueCard(sub: AdminUserSubscription) {
  issueCardTarget.value = sub
  issueCardForm.quantity = '1'
  issueCardForm.expires_at = ''
  issueCardForm.note = ''
  issueCardError.value = ''
}

async function issueResetCards() {
  const target = issueCardTarget.value
  if (!target) return
  issueCardError.value = ''
  const quantity = Number(issueCardForm.quantity.trim())
  if (!Number.isInteger(quantity) || quantity < 1 || quantity > 1000) {
    issueCardError.value = t('admin.resetCardQuantityHint')
    return
  }
  let result: { quantity: number } | null = null
  const ok = await run(async () => {
    result = await endpoints.createResetCards({
      subscription_id: target.id,
      quantity,
      expires_at: issueCardForm.expires_at,
      note: issueCardForm.note.trim(),
    })
  })
  if (!ok || !result) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.resetCardCodesCreated', { count: result.quantity }))
  issueCardTarget.value = null
  await loadSubscriptions()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.usersLead')">
      <template #actions>
        <UiButton v-if="canManage" size="sm" @click="openCreate">{{ t('admin.createUser') }}</UiButton>
        <ConsoleOpsSearch v-model="search" :placeholder="t('admin.usersSearchPlaceholder')" />
        <UiButton variant="secondary" size="sm" @click="users.refresh()">{{ t('common.refresh') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <UiAlert v-if="!canManage" tone="info">{{ t('admin.readOnlyNotice') }}</UiAlert>

    <ConsoleOpsListState
      :pending="users.pending.value"
      :error="users.error.value"
      :empty="!users.data.value.data.length"
      :empty-icon="Users"
      :empty-title="t('admin.usersEmptyTitle')"
      :empty-description="t('admin.usersEmptyBody')"
    >
      <div v-if="!users.data.value.data.length" class="rounded-card border border-line bg-surface">
        <UiEmptyState :title="t('admin.noResultsTitle')" :description="t('admin.noResultsBody')" />
      </div>

      <UiTable v-else>
        <thead>
          <tr>
            <th>{{ t('admin.id') }}</th>
            <th>{{ t('admin.email') }}</th>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('admin.inviter') }}</th>
            <th>{{ t('admin.role') }}</th>
            <th>{{ t('common.status') }}</th>
            <th class="num">{{ t('admin.balance') }}</th>
            <th class="num">{{ t('admin.maxConcurrency') }}</th>
            <th>{{ t('admin.groups') }}</th>
            <th>{{ t('common.createdAt') }}</th>
            <th v-if="canManage || canAdjustWallet">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users.data.value.data" :key="user.id">
            <td class="font-mono text-[13px] text-faint">{{ user.id }}</td>
            <td class="font-medium text-ink">{{ user.email }}</td>
            <td class="text-muted">{{ user.name }}</td>
            <td>
              <span v-if="user.inviter_id" class="text-muted">{{ user.inviter_name || user.inviter_email || user.inviter_id }}</span>
              <span v-else class="text-faint">{{ t('common.none') }}</span>
            </td>
            <td><UiBadge :tone="roleTone(user.role)">{{ roleLabel(user.role) }}</UiBadge></td>
            <td>
              <UiBadge :tone="user.enabled ? 'success' : 'neutral'" dot>
                {{ user.enabled ? t('common.enabled') : t('common.disabled') }}
              </UiBadge>
            </td>
            <td class="num">{{ formatMoney(user.balance) }}</td>
            <td class="num">{{ user.max_concurrency ?? t('admin.concurrencyPlaceholder') }}</td>
            <td>
              <div v-if="user.groups.length" class="flex flex-wrap gap-1">
                <UiBadge v-for="id in user.groups" :key="id" tone="outline">{{ groupNames.get(id) ?? id }}</UiBadge>
              </div>
              <span v-else class="text-faint">{{ t('common.none') }}</span>
            </td>
            <td class="text-muted whitespace-nowrap">{{ formatDateTime(user.created_at) }}</td>
            <td v-if="canManage || canAdjustWallet">
              <div class="flex gap-1">
                <UiButton v-if="canManage" variant="ghost" size="sm" @click="openManage(user)">{{ t('admin.manage') }}</UiButton>
                <UiButton v-if="canAdjustWallet" variant="ghost" size="sm" @click="openAdjustBalance(user)">{{ t('admin.adjustBalance') }}</UiButton>
                <UiButton v-if="canManage" variant="ghost" size="sm" @click="openSubscriptions(user)">{{ t('admin.subscriptions') }}</UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </UiTable>

      <ConsoleOpsPagination
        v-model:page="page"
        v-model:page-size="pageSize"
        :total="users.data.value.total"
        :page-size-options="['20', '50', '100']"
      />
    </ConsoleOpsListState>

    <UiSlidePanel v-model:open="dialogOpen" size="lg" :title="creating ? t('admin.createUser') : t('admin.manageUser')" :description="creating ? t('admin.createUserLead') : t('admin.manageUserLead')">
      <div class="space-y-4">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('common.name')" required>
            <UiInput v-model="form.name" />
          </UiField>
          <UiField :label="t('admin.email')" required>
            <UiInput v-model="form.email" type="email" autocomplete="off" />
          </UiField>
          <UiField v-if="!creating" :label="t('admin.id')" :hint="t('admin.idHint')">
            <UiInput v-model="form.id" type="number" mono :placeholder="editing?.id" />
          </UiField>
          <UiField :label="t('admin.role')">
            <UiSelect v-model="form.role" :options="roleOptions" :placeholder="t('common.selectPlaceholder')" />
          </UiField>
          <UiField :label="t('admin.newPassword')" :hint="creating ? t('admin.passwordRequired') : t('admin.newPasswordHint')" :required="creating">
            <UiInput v-model="form.password" type="password" autocomplete="new-password" />
          </UiField>
          <UiField :label="t('admin.balance')" :hint="t('admin.balanceHint')">
            <UiInput v-model="form.balance" type="number" mono />
          </UiField>
          <UiField :label="t('admin.userConcurrency')" :hint="t('admin.userConcurrencyHint')">
            <UiInput v-model="form.maxConcurrency" type="number" min="1" max="10000" step="1" mono :placeholder="t('admin.concurrencyPlaceholder')" />
          </UiField>
          <UiField :label="t('admin.inviter')" :hint="t('admin.inviterHint')">
            <UiInput v-model="form.inviterId" type="number" min="1" step="1" mono :placeholder="t('admin.inviterPlaceholder')" />
          </UiField>
          <UiField :label="t('admin.note')" :hint="t('admin.noteHint')">
            <UiInput v-model="form.note" />
          </UiField>
        </div>

        <UiCheckbox v-model="form.enabled">{{ t('admin.accountEnabled') }}</UiCheckbox>

        <UiField :label="t('admin.preferences')">
          <div class="space-y-3 rounded-control border border-line bg-sunken px-3 py-2.5">
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="text-sm text-ink">{{ t('console.dataUsageEnabled') }}</p>
                <p class="text-[13px] text-muted">{{ t('console.dataUsageEnabledHint') }}</p>
              </div>
              <UiSwitch v-model="form.dataUsageEnabled" :label="t('console.dataUsageEnabled')" />
            </div>
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="text-sm text-ink">{{ t('console.leaderboardOptIn') }}</p>
                <p class="text-[13px] text-muted">{{ t('console.leaderboardOptInHint') }}</p>
              </div>
              <UiSwitch v-model="form.leaderboardOptIn" :label="t('console.leaderboardOptIn')" />
            </div>
            <div class="flex items-start justify-between gap-4">
              <div class="min-w-0">
                <p class="text-sm text-ink">{{ t('console.leaderboardMaskName') }}</p>
                <p class="text-[13px] text-muted">{{ t('console.leaderboardMaskNameHint') }}</p>
              </div>
              <UiSwitch v-model="form.leaderboardMaskName" :disabled="!form.leaderboardOptIn" :label="t('console.leaderboardMaskName')" />
            </div>
          </div>
        </UiField>

        <UiField :label="t('admin.permissions')">
          <div class="grid gap-x-4 gap-y-2 rounded-control border border-line bg-sunken px-3 py-2.5 sm:grid-cols-2">
            <UiCheckbox
              v-for="permission in PERMISSIONS"
              :key="permission.value"
              :model-value="form.permissions.includes(permission.value)"
              @update:model-value="togglePermission(permission.value, $event)"
            >
              {{ t(permission.labelKey) }}
            </UiCheckbox>
          </div>
        </UiField>

        <UiField :label="t('admin.groups')">
          <ConsoleOpsGroupPicker v-model="form.groups" :options="groupOptions" />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ creating ? t('admin.createUser') : t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel
      v-model:open="subscriptionsOpen"
      size="lg"
      :title="t('admin.manageSubscriptions')"
      :description="t('admin.manageSubscriptionsLead')"
    >
      <div class="space-y-4">
        <UiAlert v-if="subsError" tone="danger">{{ subsError }}</UiAlert>

        <UiCard :title="editingSub ? t('admin.editSubscription') : t('admin.addSubscription')">
          <div class="space-y-4">
            <UiAlert v-if="subFormError" tone="danger">{{ subFormError }}</UiAlert>
            <div class="grid gap-4 sm:grid-cols-2">
              <UiField v-if="editingSub" :label="t('admin.plan')">
                <UiInput :model-value="editingSub.plan_name" disabled />
              </UiField>
              <UiField v-else :label="t('admin.plan')" required>
                <UiSelect
                  v-model="subForm.plan_id"
                  :options="subPlanOptions"
                  :placeholder="t('common.selectPlaceholder')"
                />
              </UiField>
              <UiField :label="t('admin.periodStart')">
                <UiInput v-model="subForm.start_at" type="datetime-local" />
              </UiField>
              <UiField :label="t('admin.periodEnd')">
                <UiInput v-model="subForm.end_at" type="datetime-local" />
              </UiField>
            </div>
            <UiCheckbox v-model="subForm.auto_renew">{{ t('admin.autoRenew') }}</UiCheckbox>
            <div v-if="editingSub" class="grid gap-4 sm:grid-cols-2">
              <UiField v-if="editingSub.max_requests_per_period !== null" :label="t('admin.remainingRequests')" :hint="t('admin.remainingRequestsHint', { max: formatNumber(editingSub.max_requests_per_period) })">
                <UiInput v-model="subForm.remaining_requests" type="number" min="0" :max="editingSub.max_requests_per_period" mono />
              </UiField>
              <UiField v-if="editingSub.max_credit_per_period !== null" :label="t('admin.remainingCredit')" :hint="t('admin.remainingCreditHint', { max: formatMoney(editingSub.max_credit_per_period) })">
                <UiInput v-model="subForm.remaining_credit" type="number" min="0" :max="editingSub.max_credit_per_period" step="any" mono />
              </UiField>
            </div>
            <div class="flex justify-end gap-2">
              <UiButton v-if="editingSub" variant="secondary" @click="editingSub = null; resetSubForm()">
                {{ t('common.cancel') }}
              </UiButton>
              <UiButton :loading="busy" @click="saveSubscription">
                {{ editingSub ? t('common.save') : t('admin.addSubscription') }}
              </UiButton>
            </div>
          </div>
        </UiCard>

        <div class="min-w-0 space-y-1">
          <h3 class="text-sm font-semibold text-ink">{{ t('admin.subscriptions') }}</h3>
        </div>

        <UiSkeleton v-if="subsPending" :rows="3" />

        <div v-else-if="!subsError && !subs.length" class="rounded-card border border-line bg-surface p-6 text-center">
          <p class="text-[13px] text-muted">{{ t('admin.noSubscriptionsBody') }}</p>
        </div>

        <UiTable v-else-if="subs.length">
          <thead>
            <tr>
              <th>{{ t('admin.plan') }}</th>
              <th>{{ t('system.remainingQuota') }}</th>
              <th>{{ t('common.status') }}</th>
              <th class="num">{{ t('admin.periodStart') }}</th>
              <th class="num">{{ t('admin.periodEnd') }}</th>
              <th>{{ t('admin.autoRenew') }}</th>
              <th>{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="sub in subs" :key="sub.id">
              <td class="font-medium text-ink">{{ sub.plan_name }}</td>
              <td>
                <p v-for="line in subQuotaLines(sub)" :key="line" class="whitespace-nowrap text-[13px] text-muted">
                  {{ line }}
                </p>
              </td>
              <td>
                <UiBadge :tone="STATUS_TONES[sub.status]" dot>{{ t(STATUS_KEYS[sub.status]) }}</UiBadge>
              </td>
              <td class="num text-muted whitespace-nowrap">{{ formatDateTime(sub.current_period_start) }}</td>
              <td class="num text-muted whitespace-nowrap">{{ formatDateTime(sub.current_period_end) }}</td>
              <td>{{ sub.auto_renew ? t('system.yes') : t('system.no') }}</td>
              <td>
                <div class="flex gap-1">
                  <UiButton variant="ghost" size="sm" @click="startEditSubscription(sub)">{{ t('common.edit') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="openIssueCard(sub)">{{ t('admin.issueResetCard') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="resetQuotaTarget = sub">{{ t('admin.resetSubscriptionQuota') }}</UiButton>
                  <UiButton variant="ghost" size="sm" @click="voidTarget = sub">{{ t('admin.void') }}</UiButton>
                  <UiButton variant="ghost" size="sm" class="text-danger" @click="deleteTarget = sub">
                    {{ t('admin.delete') }}
                  </UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>
      </div>
    </UiSlidePanel>

    <UiDialog v-model:open="balanceOpen" size="sm" :title="t('admin.adjustBalance')" :description="t('admin.adjustBalanceLead')">
      <div class="space-y-4">
        <UiAlert v-if="balanceError" tone="danger">{{ balanceError }}</UiAlert>
        <div v-if="balanceUser" class="rounded-control border border-line bg-sunken px-3 py-2.5 text-sm">
          <div class="flex items-center justify-between gap-4">
            <span class="text-muted">{{ t('admin.email') }}</span>
            <span class="font-medium text-ink">{{ balanceUser.email }}</span>
          </div>
          <div class="mt-1.5 flex items-center justify-between gap-4">
            <span class="text-muted">{{ t('admin.currentBalance') }}</span>
            <span class="font-medium text-ink">{{ formatMoney(balanceUser.balance) }}</span>
          </div>
          <div v-if="projectedBalance !== null" class="mt-1.5 flex items-center justify-between gap-4 border-t border-line pt-1.5">
            <span class="text-muted">{{ t('admin.newBalance') }}</span>
            <span class="font-medium text-ink">{{ formatMoney(projectedBalance) }}</span>
          </div>
        </div>
        <UiField :label="t('admin.adjustAmount')" :hint="t('admin.adjustAmountHint')" required>
          <UiInput v-model="balanceForm.amount" type="number" mono />
        </UiField>
        <UiField :label="t('admin.note')" required>
          <UiInput v-model="balanceForm.note" />
        </UiField>
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="balanceOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="submitAdjustBalance">{{ t('common.confirm') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog :open="resetQuotaTarget !== null" size="sm" :title="t('admin.resetSubscriptionQuota')">
      <p class="text-sm text-muted">{{ t('admin.confirmResetSubscriptionQuota') }}</p>
      <template #footer>
        <UiButton variant="secondary" @click="resetQuotaTarget = null">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="resetSubscriptionQuota">{{ t('common.confirm') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog :open="voidTarget !== null" size="sm" :title="t('admin.void')">
      <p class="text-sm text-muted">{{ t('admin.confirmVoidSubscription') }}</p>
      <template #footer>
        <UiButton variant="secondary" @click="voidTarget = null">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="voidSubscription">{{ t('admin.void') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog :open="deleteTarget !== null" size="sm" :title="t('common.delete')">
      <p class="text-sm text-muted">{{ t('admin.confirmDeleteSubscription') }}</p>
      <template #footer>
        <UiButton variant="secondary" @click="deleteTarget = null">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="deleteSubscription">{{ t('common.delete') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog :open="issueCardTarget !== null" :title="t('admin.issueResetCard')" size="md">
      <div class="space-y-4">
        <p v-if="issueCardTarget" class="text-[13px] text-muted">
          {{ t('admin.issueResetCardLead', { plan: issueCardTarget.plan_name }) }}
        </p>
        <UiAlert v-if="issueCardError" tone="danger" :title="issueCardError" />
        <UiField :label="t('admin.resetCardQuantity')" :hint="t('admin.resetCardQuantityHint')" required>
          <UiInput v-model="issueCardForm.quantity" type="number" min="1" max="1000" />
        </UiField>
        <UiField :label="t('admin.resetCardExpiresAt')" :hint="t('admin.resetCardExpiresAtHint')">
          <UiInput v-model="issueCardForm.expires_at" type="datetime-local" />
        </UiField>
        <UiField :label="t('admin.resetCardNote')">
          <UiTextarea v-model="issueCardForm.note" :rows="2" />
        </UiField>
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="issueCardTarget = null">{{ t('admin.resetCardFormCancel') }}</UiButton>
        <UiButton :loading="busy" @click="issueResetCards">{{ t('admin.resetCardFormSubmit') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
