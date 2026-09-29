<script setup lang="ts">
import { Layers, Plus, Users } from 'lucide-vue-next'
import { endpoints, type Workspace, type WorkspaceMember, type WorkspaceRole } from '~/src/api'
import { formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { toast } = useToast()
const { settings } = useSiteSettings()
const { workspaces, current, loading, error, loadWorkspaces, switchWorkspace } = useWorkspace()
const { busy, error: actionError, run } = useAction()
const formOpen = ref(false)
const deleteOpen = ref(false)
const editing = ref<Workspace | null>(null)
const deleting = ref<Workspace | null>(null)
const form = reactive({ name: '', slug: '' })
const formError = ref('')
const memberSpace = ref<Workspace | null>(null)
const members = ref<WorkspaceMember[]>([])
const membersLoading = ref(false)
const membersError = ref('')
const memberFormOpen = ref(false)
const memberRemoveOpen = ref(false)
const editingMember = ref<WorkspaceMember | null>(null)
const removingMember = ref<WorkspaceMember | null>(null)
const memberForm = reactive({ email: '', role: 'member' as 'admin' | 'member' })
const memberFormError = ref('')
let memberRequestId = 0
let active = true

useHead({ title: () => `${t('nav.workspaces')} · ${settings.value.name}` })
onBeforeUnmount(() => { active = false; memberRequestId += 1 })

const memberRoleOptions = computed(() => [
  { value: 'member', label: t('console.workspaceRoleMember') },
  ...(memberSpace.value?.role === 'owner' ? [{ value: 'admin', label: t('console.workspaceRoleAdmin') }] : []),
])
const canAddMember = computed(() => memberSpace.value && !memberSpace.value.is_personal && ['owner', 'admin'].includes(memberSpace.value.role))
const roleLabels: Record<WorkspaceRole, string> = {
  owner: 'console.workspaceRoleOwner', admin: 'console.workspaceRoleAdmin', member: 'console.workspaceRoleMember',
}

function canEdit(workspace: Workspace) {
  return !workspace.is_personal && ['owner', 'admin'].includes(workspace.role)
}

function canRemoveMember(member: WorkspaceMember) {
  return member.role !== 'owner' && (memberSpace.value?.role === 'owner' || (memberSpace.value?.role === 'admin' && member.role === 'member'))
}

function openForm(workspace: Workspace | null = null) {
  editing.value = workspace
  form.name = workspace?.name ?? ''
  form.slug = workspace?.slug ?? ''
  formError.value = ''
  formOpen.value = true
}

async function saveWorkspace() {
  formError.value = ''
  if (!form.name.trim()) { formError.value = t('console.workspaceNameRequired'); return }
  if (!/^[a-z0-9]([a-z0-9-]{0,48}[a-z0-9])?$/.test(form.slug)) { formError.value = t('console.workspaceSlugInvalid'); return }
  const target = editing.value
  const values = { name: form.name.trim(), slug: form.slug }
  const ok = await run(() => target ? endpoints.updateWorkspace(target.id, values) : endpoints.createWorkspace(values))
  if (!active) return
  if (!ok) { formError.value = actionError.value; return }
  formOpen.value = false
  toast.success(t(target ? 'console.workspaceUpdated' : 'console.workspaceCreated'))
  await loadWorkspaces(true)
}

function openDelete(workspace: Workspace) {
  deleting.value = workspace
  actionError.value = ''
  deleteOpen.value = true
}

async function deleteWorkspace() {
  const target = deleting.value
  if (!target || target.is_personal || target.role !== 'owner') return
  const ok = await run(() => endpoints.deleteWorkspace(target.id))
  if (!active || !ok) return
  deleteOpen.value = false
  if (memberSpace.value?.id === target.id) memberSpace.value = null
  toast.success(t('console.workspaceDeleted'))
  await loadWorkspaces(true)
}

async function loadMembers(workspace: Workspace) {
  if (workspace.is_personal) return
  const requestId = ++memberRequestId
  memberSpace.value = workspace
  members.value = []
  membersError.value = ''
  membersLoading.value = true
  try {
    const result = await endpoints.getWorkspaceMembers(workspace.id)
    if (active && requestId === memberRequestId) members.value = result.data
  } catch (cause) {
    if (active && requestId === memberRequestId) membersError.value = cause instanceof Error ? cause.message : t('common.loadFailed')
  } finally {
    if (active && requestId === memberRequestId) membersLoading.value = false
  }
}

function openMemberForm(member: WorkspaceMember | null = null) {
  editingMember.value = member
  memberForm.email = member?.email ?? ''
  memberForm.role = member?.role === 'admin' ? 'admin' : 'member'
  memberFormError.value = ''
  memberFormOpen.value = true
}

async function saveMember() {
  const workspace = memberSpace.value
  if (!workspace || !canAddMember.value) return
  memberFormError.value = ''
  if (!memberForm.email.trim()) { memberFormError.value = t('console.workspaceEmailRequired'); return }
  const member = editingMember.value
  const role = memberForm.role
  const ok = await run(() => member
    ? endpoints.updateWorkspaceMember(workspace.id, member.user_id, role)
    : endpoints.addWorkspaceMember(workspace.id, { email: memberForm.email.trim(), role }))
  if (!active) return
  if (!ok) { memberFormError.value = actionError.value; return }
  memberFormOpen.value = false
  toast.success(t(member ? 'console.workspaceMemberUpdated' : 'console.workspaceMemberAdded'))
  await loadMembers(workspace)
}

function openRemoveMember(member: WorkspaceMember) {
  removingMember.value = member
  actionError.value = ''
  memberRemoveOpen.value = true
}

async function removeMember() {
  const workspace = memberSpace.value
  const member = removingMember.value
  if (!workspace || !member || !canRemoveMember(member)) return
  const ok = await run(() => endpoints.removeWorkspaceMember(workspace.id, member.user_id))
  if (!active || !ok) return
  memberRemoveOpen.value = false
  toast.success(t('console.workspaceMemberRemoved'))
  await loadMembers(workspace)
}
</script>

<template>
  <div class="space-y-5">
    <ConsoleOpsPageHeader :lead="t('console.workspaceDescription')">
      <template #actions>
        <UiButton @click="openForm()"><Plus class="size-4" aria-hidden="true" />{{ t('console.workspaceCreate') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <UiAlert :title="t('console.workspaceScopeTitle')">{{ t('console.workspaceScopeNotice') }}</UiAlert>

    <UiCard>
      <UiSkeleton v-if="loading" :rows="5" />
      <UiAlert v-else-if="error" tone="danger">
        {{ error }}
        <UiButton variant="link" @click="loadWorkspaces(true)">{{ t('common.retry') }}</UiButton>
      </UiAlert>
      <UiEmptyState v-else-if="!workspaces.length" :icon="Layers" :title="t('console.workspaceEmpty')" :description="t('console.workspaceEmptyBody')">
        <UiButton @click="openForm()">{{ t('console.workspaceCreate') }}</UiButton>
      </UiEmptyState>
      <UiTable v-else>
        <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('console.workspaceSlug') }}</th><th>{{ t('console.workspaceRole') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
        <tbody>
          <tr v-for="workspace in workspaces" :key="workspace.id">
            <td>
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-medium">{{ workspace.is_personal ? t('console.workspacePersonal') : workspace.name }}</span>
                <UiBadge v-if="workspace.id === current?.id" tone="success">{{ t('console.workspaceCurrent') }}</UiBadge>
              </div>
            </td>
            <td class="font-mono text-xs">{{ workspace.slug }}</td>
            <td>{{ t(roleLabels[workspace.role]) }}</td>
            <td>
              <div class="flex flex-wrap gap-1">
                <UiButton v-if="workspace.id !== current?.id" size="sm" variant="secondary" :disabled="busy" @click="switchWorkspace(workspace.id)">{{ t('console.workspaceSelect') }}</UiButton>
                <UiButton v-if="!workspace.is_personal" size="sm" variant="ghost" :disabled="busy" @click="loadMembers(workspace)">{{ t('console.workspaceMembers') }}</UiButton>
                <UiButton v-if="canEdit(workspace)" size="sm" variant="ghost" :disabled="busy" @click="openForm(workspace)">{{ t('common.edit') }}</UiButton>
                <UiButton v-if="!workspace.is_personal && workspace.role === 'owner'" size="sm" variant="ghost" class="text-danger" :disabled="busy" @click="openDelete(workspace)">{{ t('common.delete') }}</UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </UiCard>

    <UiCard v-if="memberSpace" :title="t('console.workspaceMembersTitle', { name: memberSpace.name })" :description="t('console.workspaceMembersHint')">
      <template #actions>
        <UiButton v-if="canAddMember" size="sm" :disabled="busy" @click="openMemberForm()"><Plus class="size-4" aria-hidden="true" />{{ t('console.workspaceAddMember') }}</UiButton>
      </template>
      <UiSkeleton v-if="membersLoading" :rows="4" />
      <UiAlert v-else-if="membersError" tone="danger">
        {{ membersError }}
        <UiButton variant="link" @click="loadMembers(memberSpace)">{{ t('common.retry') }}</UiButton>
      </UiAlert>
      <UiEmptyState v-else-if="!members.length" :icon="Users" :title="t('console.workspaceMembersEmpty')" :description="t('console.workspaceMembersHint')" />
      <UiTable v-else>
        <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('console.workspaceEmail') }}</th><th>{{ t('console.workspaceRole') }}</th><th>{{ t('common.createdAt') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
        <tbody>
          <tr v-for="member in members" :key="member.user_id">
            <td>{{ member.name }}</td><td>{{ member.email }}</td><td>{{ t(roleLabels[member.role]) }}</td><td>{{ formatDateTime(member.created_at) }}</td>
            <td>
              <div class="flex gap-1">
                <UiButton v-if="memberSpace.role === 'owner' && member.role !== 'owner'" size="sm" variant="ghost" :disabled="busy" @click="openMemberForm(member)">{{ t('console.workspaceEditRole') }}</UiButton>
                <UiButton v-if="canRemoveMember(member)" size="sm" variant="ghost" class="text-danger" :disabled="busy" @click="openRemoveMember(member)">{{ t('console.workspaceRemoveMember') }}</UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </UiCard>

    <UiDialog v-model:open="formOpen" :title="t(editing ? 'console.workspaceEdit' : 'console.workspaceCreate')">
      <form id="workspace-form" class="space-y-4" @submit.prevent="saveWorkspace">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>
        <UiField :label="t('common.name')" for="workspace-name" required>
          <UiInput id="workspace-name" v-model="form.name" :disabled="busy" :placeholder="t('console.workspaceNamePlaceholder')" />
        </UiField>
        <UiField :label="t('console.workspaceSlug')" for="workspace-slug" :hint="t('console.workspaceSlugHint')" required>
          <UiInput id="workspace-slug" v-model="form.slug" mono :disabled="busy || Boolean(editing && (editing.is_personal || editing.role !== 'owner'))" :placeholder="t('console.workspaceSlugPlaceholder')" />
        </UiField>
      </form>
      <template #footer>
        <UiButton variant="secondary" :disabled="busy" @click="formOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton type="submit" form="workspace-form" :loading="busy">{{ t('common.save') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog v-model:open="deleteOpen" :title="t('console.workspaceDeleteTitle')" :description="t('console.workspaceDeleteBody', { name: deleting?.name ?? '' })">
      <UiAlert v-if="actionError" tone="danger">{{ actionError }}</UiAlert>
      <p class="text-sm text-muted">{{ t('console.workspaceDeleteWarning') }}</p>
      <template #footer>
        <UiButton variant="secondary" :disabled="busy" @click="deleteOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="deleteWorkspace">{{ t('common.delete') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog v-model:open="memberFormOpen" :title="t(editingMember ? 'console.workspaceEditRole' : 'console.workspaceAddMember')" :description="t('console.workspaceMembersHint')">
      <form id="workspace-member-form" class="space-y-4" @submit.prevent="saveMember">
        <UiAlert v-if="memberFormError" tone="danger">{{ memberFormError }}</UiAlert>
        <UiField :label="t('console.workspaceEmail')" for="workspace-email" required>
          <UiInput id="workspace-email" v-model="memberForm.email" type="email" :disabled="busy || Boolean(editingMember)" :placeholder="t('console.workspaceEmailPlaceholder')" />
        </UiField>
        <UiField :label="t('console.workspaceRole')" for="workspace-role">
          <UiSelect id="workspace-role" v-model="memberForm.role" :options="memberRoleOptions" :disabled="busy" />
        </UiField>
      </form>
      <template #footer>
        <UiButton variant="secondary" :disabled="busy" @click="memberFormOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton type="submit" form="workspace-member-form" :loading="busy">{{ t('common.save') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog v-model:open="memberRemoveOpen" :title="t('console.workspaceRemoveMember')" :description="t('console.workspaceRemoveBody', { email: removingMember?.email ?? '' })">
      <UiAlert v-if="actionError" tone="danger">{{ actionError }}</UiAlert>
      <template #footer>
        <UiButton variant="secondary" :disabled="busy" @click="memberRemoveOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="removeMember">{{ t('console.workspaceRemoveMember') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
