<script setup lang="ts">
import { Users } from 'lucide-vue-next'
import { endpoints, type Group, type User, type UserUpdate } from '~/src/api'
import { formatDateTime, formatMoney } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('users.read'))
const canManage = computed(() => can('system.manage'))

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

const users = useResource(() => endpoints.getAdminUsers(), { data: [] as User[] })
const groups = useResource(() => endpoints.getAdminGroups(), { data: [] as Group[] })

const search = ref('')

const groupOptions = computed(() => groups.data.value.data)
const groupNames = computed(() => new Map(groupOptions.value.map(group => [group.id, group.name])))

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) return users.data.value.data
  return users.data.value.data.filter(user =>
    user.email.toLowerCase().includes(term) || user.name.toLowerCase().includes(term))
})

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
const formError = ref('')
const form = reactive({
  name: '',
  email: '',
  role: 'user',
  enabled: true,
  password: '',
  balance: '',
  note: '',
  permissions: [] as string[],
  groups: [] as string[],
})

function openManage(user: User) {
  editing.value = user
  formError.value = ''
  form.name = user.name
  form.email = user.email
  form.role = user.role
  form.enabled = user.enabled
  form.password = ''
  form.balance = ''
  form.note = ''
  form.permissions = [...user.permissions]
  form.groups = [...user.groups]
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

  const update: UserUpdate = {
    name,
    email,
    role: form.role,
    enabled: form.enabled,
    permissions: form.permissions,
    groups: form.groups,
  }
  if (form.password) update.password = form.password

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
  const target = editing.value
  if (!target) return
  formError.value = ''
  const update = buildUpdate()
  if (!update) return

  const ok = await run(() => endpoints.updateUser(target.id, update))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.userSaved'))
  dialogOpen.value = false
  await users.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.usersLead')">
      <template #actions>
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
      <div v-if="!filtered.length" class="rounded-card border border-line bg-surface">
        <UiEmptyState :title="t('admin.noResultsTitle')" :description="t('admin.noResultsBody')" />
      </div>

      <UiTable v-else>
        <thead>
          <tr>
            <th>{{ t('admin.id') }}</th>
            <th>{{ t('admin.email') }}</th>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('admin.role') }}</th>
            <th>{{ t('common.status') }}</th>
            <th class="num">{{ t('admin.balance') }}</th>
            <th>{{ t('admin.groups') }}</th>
            <th>{{ t('common.createdAt') }}</th>
            <th v-if="canManage">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in filtered" :key="user.id">
            <td class="font-mono text-[13px] text-faint">{{ user.id }}</td>
            <td class="font-medium text-ink">{{ user.email }}</td>
            <td class="text-muted">{{ user.name }}</td>
            <td><UiBadge :tone="roleTone(user.role)">{{ roleLabel(user.role) }}</UiBadge></td>
            <td>
              <UiBadge :tone="user.enabled ? 'success' : 'neutral'" dot>
                {{ user.enabled ? t('common.enabled') : t('common.disabled') }}
              </UiBadge>
            </td>
            <td class="num">{{ formatMoney(user.balance) }}</td>
            <td>
              <div v-if="user.groups.length" class="flex flex-wrap gap-1">
                <UiBadge v-for="id in user.groups" :key="id" tone="outline">{{ groupNames.get(id) ?? id }}</UiBadge>
              </div>
              <span v-else class="text-faint">{{ t('common.none') }}</span>
            </td>
            <td class="text-muted whitespace-nowrap">{{ formatDateTime(user.created_at) }}</td>
            <td v-if="canManage">
              <UiButton variant="ghost" size="sm" @click="openManage(user)">{{ t('admin.manage') }}</UiButton>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiSlidePanel v-model:open="dialogOpen" size="lg" :title="t('admin.manageUser')" :description="t('admin.manageUserLead')">
      <div class="space-y-4">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('common.name')" required>
            <UiInput v-model="form.name" />
          </UiField>
          <UiField :label="t('admin.email')" required>
            <UiInput v-model="form.email" type="email" autocomplete="off" />
          </UiField>
          <UiField :label="t('admin.role')">
            <UiSelect v-model="form.role" :options="roleOptions" :placeholder="t('common.selectPlaceholder')" />
          </UiField>
          <UiField :label="t('admin.newPassword')" :hint="t('admin.newPasswordHint')">
            <UiInput v-model="form.password" type="password" autocomplete="new-password" />
          </UiField>
          <UiField :label="t('admin.balance')" :hint="t('admin.balanceHint')">
            <UiInput v-model="form.balance" type="number" mono />
          </UiField>
          <UiField :label="t('admin.note')" :hint="t('admin.noteHint')">
            <UiInput v-model="form.note" />
          </UiField>
        </div>

        <UiCheckbox v-model="form.enabled">{{ t('admin.accountEnabled') }}</UiCheckbox>

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
        <UiButton :loading="busy" @click="save">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>
  </div>
</template>
