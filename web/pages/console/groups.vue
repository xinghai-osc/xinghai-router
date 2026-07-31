<script setup lang="ts">
import { Layers } from 'lucide-vue-next'
import { endpoints, type Group } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('users.read'))
const canManage = computed(() => can('system.manage'))

const groups = useResource(() => endpoints.getAdminGroups(), { data: [] as Group[] })

const drafts = reactive<Record<string, string>>({})
const publicDrafts = reactive<Record<string, boolean>>({})

watch(() => groups.data.value.data, (list) => {
  for (const group of list) {
    drafts[group.id] = String(group.multiplier)
    publicDrafts[group.id] = group.public
  }
}, { immediate: true })

function isDirty(group: Group) {
  const draft = drafts[group.id]
  const publicChanged = publicDrafts[group.id] !== group.public
  const multiplierChanged = draft !== undefined && draft.trim() !== '' && Number(draft) !== group.multiplier
  return multiplierChanged || publicChanged
}

function validMultiplier(value: number) {
  return Number.isFinite(value) && value > 0 && value <= 1000
}

const savingId = ref('')

async function saveGroup(group: Group) {
  const value = Number(drafts[group.id])
  if (!validMultiplier(value)) { toast.error(t('admin.multiplierInvalid')); return }
  savingId.value = group.id
  const publicValue = publicDrafts[group.id] ?? group.public
  const ok = await run(() => endpoints.updateGroup(group.id, value, null, publicValue))
  savingId.value = ''
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.groupUpdated'))
  await groups.refresh()
}

const createOpen = ref(false)
const createError = ref('')
const createForm = reactive({ name: '', multiplier: '1', public: false })

function openCreate() {
  createError.value = ''
  createForm.name = ''
  createForm.multiplier = '1'
  createForm.public = false
  createOpen.value = true
}

async function create() {
  createError.value = ''
  const name = createForm.name.trim()
  if (!name) { createError.value = t('admin.groupNameRequired'); return }
  const multiplier = Number(createForm.multiplier)
  if (!validMultiplier(multiplier)) { createError.value = t('admin.multiplierInvalid'); return }

  const ok = await run(() => endpoints.createGroup(name, multiplier, null, createForm.public))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.groupCreated'))
  createOpen.value = false
  await groups.refresh()
}

const importOpen = ref(false)
const importError = ref('')
const importText = ref('')

function openImport() {
  importError.value = ''
  importText.value = ''
  importOpen.value = true
}

function parseImport(): Record<string, number> | null {
  let parsed: unknown
  try {
    parsed = JSON.parse(importText.value)
  } catch {
    importError.value = t('admin.importInvalidJson')
    return null
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    importError.value = t('admin.importInvalidJson')
    return null
  }
  const entries = Object.entries(parsed as Record<string, unknown>)
  if (!entries.length) { importError.value = t('admin.importEmpty'); return null }

  const payload: Record<string, number> = {}
  for (const [name, value] of entries) {
    const multiplier = Number(value)
    if (!name.trim()) { importError.value = t('admin.groupNameRequired'); return null }
    if (typeof value !== 'number' || !validMultiplier(multiplier)) {
      importError.value = t('admin.importInvalidValue')
      return null
    }
    payload[name.trim()] = multiplier
  }
  return payload
}

async function runImport() {
  importError.value = ''
  const payload = parseImport()
  if (!payload) return

  const ok = await run(() => endpoints.importGroups(payload))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.importDone', { count: Object.keys(payload).length }))
  importOpen.value = false
  await groups.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.groupsLead')">
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="groups.refresh()">{{ t('common.refresh') }}</UiButton>
        <template v-if="canManage">
          <UiButton variant="secondary" size="sm" @click="openImport">{{ t('admin.importGroups') }}</UiButton>
          <UiButton size="sm" @click="openCreate">{{ t('admin.createGroup') }}</UiButton>
        </template>
      </template>
    </ConsoleOpsPageHeader>

    <UiAlert v-if="!canManage" tone="info">{{ t('admin.readOnlyNotice') }}</UiAlert>

    <ConsoleOpsListState
      :pending="groups.pending.value"
      :error="groups.error.value"
      :empty="!groups.data.value.data.length"
      :empty-icon="Layers"
      :empty-title="t('admin.groupsEmptyTitle')"
      :empty-description="t('admin.groupsEmptyBody')"
    >
      <UiTable>
        <thead>
          <tr>
            <th>{{ t('admin.groupName') }}</th>
            <th class="num">{{ t('admin.multiplier') }}</th>
            <th class="text-center">{{ t('admin.groupPublic') }}</th>
            <th>{{ t('common.createdAt') }}</th>
            <th v-if="canManage">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="group in groups.data.value.data" :key="group.id">
            <td class="font-medium text-ink">{{ group.name }}</td>
            <td class="num">
              <div v-if="canManage" class="flex justify-end">
                <UiInput
                  v-model="drafts[group.id]"
                  type="number"
                  mono
                  class="w-28"
                  :aria-label="t('admin.multiplier')"
                />
              </div>
              <span v-else>{{ group.multiplier }}</span>
            </td>
            <td class="text-center">
              <UiSwitch
                v-if="canManage"
                v-model="publicDrafts[group.id]"
                :label="t('admin.groupPublic')"
              />
              <UiBadge v-else-if="group.public" tone="outline">{{ t('admin.groupPublic') }}</UiBadge>
              <span v-else class="text-faint">—</span>
            </td>
            <td class="text-muted whitespace-nowrap">{{ formatDateTime(group.created_at) }}</td>
            <td v-if="canManage">
              <UiButton
                variant="ghost"
                size="sm"
                :disabled="!isDirty(group)"
                :loading="savingId === group.id"
                @click="saveGroup(group)"
              >
                {{ t('common.save') }}
              </UiButton>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiSlidePanel v-model:open="createOpen" size="sm" :title="t('admin.createGroup')">
      <div class="space-y-4">
        <UiAlert v-if="createError" tone="danger">{{ createError }}</UiAlert>
        <UiField :label="t('admin.groupName')" required>
          <UiInput v-model="createForm.name" :placeholder="t('admin.groupNamePlaceholder')" />
        </UiField>
        <UiField :label="t('admin.multiplier')" required>
          <UiInput v-model="createForm.multiplier" type="number" mono />
        </UiField>

        <UiField :label="t('admin.groupPublic')" :hint="t('admin.groupPublicHint')">
          <UiSwitch v-model="createForm.public" :label="t('admin.groupPublic')" />
        </UiField>
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="createOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="create">{{ t('common.create') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel
      v-model:open="importOpen"
      :title="t('admin.importGroups')"
      :description="t('admin.importGroupsLead')"
    >
      <div class="space-y-4">
        <UiAlert v-if="importError" tone="danger">{{ importError }}</UiAlert>
        <UiField :label="t('admin.importJson')" required>
          <UiTextarea v-model="importText" mono :rows="8" :placeholder="t('admin.importPlaceholder')" />
        </UiField>
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="importOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="runImport">{{ t('common.submit') }}</UiButton>
      </template>
    </UiSlidePanel>
  </div>
</template>
