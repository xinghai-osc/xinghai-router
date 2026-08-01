<script setup lang="ts">
import { KeyRound, MoreHorizontal, Plus } from 'lucide-vue-next'
import { endpoints, type AccountKeyForm, type ApiKey, type Group } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { toast } = useToast()
const { settings } = useSiteSettings()
const route = useRoute()

useHead({ title: () => `${t('nav.keys')} · ${settings.value.name}` })

const { data: keys, pending, error, refresh } = useResource(
  () => endpoints.getAccountKeys(),
  { data: [] as ApiKey[] },
)
const { data: groups } = useResource(
  () => endpoints.getAccountGroups(),
  { data: [] as string[], groups: [] as Group[] },
)
const { busy, run } = useAction()

const formOpen = ref(false)
const secretOpen = ref(false)
const revealOpen = ref(false)
const revokeOpen = ref(false)
const editing = ref<ApiKey | null>(null)
const revoking = ref<ApiKey | null>(null)
const revealing = ref<ApiKey | null>(null)
const secret = ref('')
const revealedSecret = ref('')
const revealError = ref('')
const formError = ref('')
const form = reactive({ name: '', expiresOn: '', groupId: '' })

const groupOptions = computed(() => [
  { value: '', label: t('console.keyGroupDefault') },
  ...groups.value.groups.map(group => ({
    value: group.id,
    label: group.public ? `${group.name} (${t('admin.groupPublic')})` : group.name,
  })),
])

const STATES = {
  active: { tone: 'success', labelKey: 'console.keyStatusActive' },
  revoked: { tone: 'danger', labelKey: 'console.keyStatusRevoked' },
  expired: { tone: 'warn', labelKey: 'console.keyStatusExpired' },
} as const

function keyState(key: ApiKey): keyof typeof STATES {
  if (key.revoked_at) return 'revoked'
  if (key.expires_at && new Date(key.expires_at).getTime() < Date.now()) return 'expired'
  return 'active'
}

const rows = computed(() => keys.value.data.map(key => ({ key, state: STATES[keyState(key)] })))

/** `<input type="date">` speaks local `YYYY-MM-DD`; the API speaks RFC3339. */
function toDateInput(value: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

function toRfc3339(value: string): string {
  if (!value) return ''
  const date = new Date(`${value}T23:59:59`)
  return Number.isNaN(date.getTime()) ? '' : date.toISOString()
}

function openCreate() {
  editing.value = null
  formError.value = ''
  form.name = ''
  form.expiresOn = ''
  form.groupId = ''
  formOpen.value = true
}

function openEdit(key: ApiKey) {
  editing.value = key
  formError.value = ''
  form.name = key.name
  form.expiresOn = toDateInput(key.expires_at)
  form.groupId = key.group_id
  formOpen.value = true
}

function openRevoke(key: ApiKey) {
  revoking.value = key
  revokeOpen.value = true
}

function openReveal(key: ApiKey) {
  revealing.value = key
  revealedSecret.value = ''
  revealError.value = ''
  revealOpen.value = true
}

async function loadRevealedSecret() {
  const target = revealing.value
  if (!target) return
  revealedSecret.value = ''
  revealError.value = ''
  const ok = await run(async () => { revealedSecret.value = (await endpoints.revealAccountKey(target.id)).key })
  if (!ok) revealError.value = t('console.keyRevealFailed')
}

async function submitForm() {
  const name = form.name.trim()
  if (!name) {
    formError.value = t('console.keyNameRequired')
    return
  }
  const payload: AccountKeyForm = { name, expires_at: toRfc3339(form.expiresOn), group_id: form.groupId }
  const target = editing.value

  if (target) {
    const ok = await run(() => endpoints.updateAccountKey(target.id, payload))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    formOpen.value = false
    toast.success(t('console.keyUpdated'))
    await refresh()
    return
  }

  let created = ''
  const ok = await run(async () => { created = (await endpoints.createAccountKey(payload)).key })
  if (!ok) { toast.error(t('common.actionFailed')); return }
  formOpen.value = false
  secret.value = created
  secretOpen.value = true
  toast.success(t('console.keyCreated'))
  await refresh()
}

async function confirmRevoke() {
  const target = revoking.value
  if (!target) return
  const ok = await run(() => endpoints.revokeAccountKey(target.id))
  revokeOpen.value = false
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('console.keyRevoked'))
  await refresh()
}

watch(secretOpen, (open) => { if (!open) secret.value = '' })
watch(revealOpen, (open) => { if (open && revealing.value) loadRevealedSecret() })
watch(revealOpen, (open) => { if (!open) { revealedSecret.value = ''; revealing.value = null } })

onMounted(() => { if (route.query.create) openCreate() })
</script>

<template>
  <div class="space-y-4">
    <UiCard :title="t('nav.keys')" :description="t('console.keysDescription')" flush>
      <template #actions>
        <UiButton size="sm" @click="openCreate">
          <Plus class="size-4" />
          {{ t('console.createKey') }}
        </UiButton>
      </template>

      <div class="px-5 py-4">
        <ConsoleUserDataState
          :pending="pending"
          :error="error"
          :empty="!keys.data.length"
          :rows="5"
          :empty-icon="KeyRound"
          :empty-title="t('console.keysEmptyTitle')"
          :empty-description="t('console.keysEmptyBody')"
        >
          <template #empty-action>
            <UiButton size="sm" @click="openCreate">{{ t('console.createKey') }}</UiButton>
          </template>

          <UiTable>
            <thead>
              <tr>
                <th>{{ t('common.name') }}</th>
                <th>{{ t('console.keyPrefix') }}</th>
                <th>{{ t('console.group') }}</th>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('console.keyExpiry') }}</th>
                <th>{{ t('console.keyLastUsed') }}</th>
                <th>{{ t('common.createdAt') }}</th>
                <th class="text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="{ key, state } in rows" :key="key.id">
                <td class="font-medium">{{ key.name }}</td>
                <td><code class="font-mono text-[13px] text-muted">{{ key.key_prefix }}</code></td>
                <td class="text-muted">{{ key.group_name || t('console.keyGroupDefault') }}</td>
                <td>
                  <UiBadge :tone="state.tone" dot>{{ t(state.labelKey) }}</UiBadge>
                </td>
                <td class="text-muted">{{ key.expires_at ? formatDateTime(key.expires_at) : t('console.keyNeverExpires') }}</td>
                <td class="text-muted">{{ key.last_used_at ? formatDateTime(key.last_used_at) : t('console.keyNeverUsed') }}</td>
                <td class="text-muted">{{ formatDateTime(key.created_at) }}</td>
                <td class="text-right">
                  <UiDropdownMenu>
                    <template #trigger>
                      <UiButton variant="ghost" size="icon" :aria-label="t('common.actions')">
                        <MoreHorizontal class="size-4" />
                      </UiButton>
                    </template>
                    <UiDropdownItem :disabled="Boolean(key.revoked_at)" @select="openEdit(key)">
                      {{ t('common.edit') }}
                    </UiDropdownItem>
                    <UiDropdownItem :disabled="Boolean(key.revoked_at) || !key.revealable" @select="openReveal(key)">
                      {{ t('console.revealKey') }}
                    </UiDropdownItem>
                    <UiDropdownItem as="separator" />
                    <UiDropdownItem danger :disabled="Boolean(key.revoked_at)" @select="openRevoke(key)">
                      {{ t('console.revokeKey') }}
                    </UiDropdownItem>
                  </UiDropdownMenu>
                </td>
              </tr>
            </tbody>
          </UiTable>
        </ConsoleUserDataState>
      </div>
    </UiCard>

    <UiSlidePanel
      v-model:open="formOpen"
      size="sm"
      :title="editing ? t('console.editKey') : t('console.createKey')"
    >
      <form class="space-y-4" @submit.prevent="submitForm">
        <UiField :label="t('console.keyName')" required :error="formError">
          <UiInput v-model="form.name" :placeholder="t('console.keyNamePlaceholder')" />
        </UiField>

        <UiField :label="t('console.keyExpiry')" :hint="t('console.keyExpiryHint')">
          <UiInput v-model="form.expiresOn" type="date" />
        </UiField>

        <UiField :label="t('console.group')">
          <UiSelect
            v-model="form.groupId"
            :options="groupOptions"
            :placeholder="t('common.selectPlaceholder')"
          />
        </UiField>
      </form>

      <template #footer>
        <UiButton variant="secondary" @click="formOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="submitForm">
          {{ editing ? t('common.save') : t('common.create') }}
        </UiButton>
      </template>
    </UiSlidePanel>

    <UiDialog v-model:open="secretOpen" size="sm" :title="t('console.keyCreatedTitle')">
      <div class="space-y-3">
        <UiAlert tone="warn">{{ t('console.keyCreatedWarning') }}</UiAlert>

        <UiField :label="t('console.keySecret')">
          <div class="flex items-center gap-2">
            <code class="min-w-0 flex-1 truncate rounded-control bg-sunken px-3 py-2 font-mono text-[13px] text-ink">
              {{ secret }}
            </code>
            <ConsoleUserCopyButton :value="secret" :success-message="t('console.keySecretCopied')" />
          </div>
        </UiField>
      </div>

      <template #footer>
        <UiButton @click="secretOpen = false">{{ t('console.keyCreatedDone') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog v-model:open="revealOpen" size="sm" :title="t('console.revealKeyTitle')">
      <div class="space-y-3">
        <UiAlert tone="warn">{{ t('console.revealKeyWarning') }}</UiAlert>

        <UiField :label="t('console.keySecret')">
          <div class="flex items-center gap-2">
            <code class="min-w-0 flex-1 truncate rounded-control bg-sunken px-3 py-2 font-mono text-[13px] text-ink">
              {{ revealedSecret || '••••••••' }}
            </code>
            <ConsoleUserCopyButton
              v-if="revealedSecret"
              :value="revealedSecret"
              :success-message="t('console.keySecretCopied')"
            />
          </div>
        </UiField>

        <p v-if="revealError" class="text-[13px] text-danger">{{ revealError }}</p>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="revealOpen = false">{{ t('common.close') }}</UiButton>
        <UiButton v-if="revealError" :loading="busy" @click="loadRevealedSecret">
          {{ t('common.retry') }}
        </UiButton>
      </template>
    </UiDialog>

    <UiDialog
      v-model:open="revokeOpen"
      size="sm"
      :title="t('console.revokeKeyTitle')"
      :description="revoking?.name"
    >
      <p class="text-[13px] text-muted">{{ t('console.revokeKeyBody') }}</p>

      <template #footer>
        <UiButton variant="secondary" @click="revokeOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="confirmRevoke">{{ t('console.revokeKey') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
