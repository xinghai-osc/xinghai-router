<script setup lang="ts">
import { Blocks } from 'lucide-vue-next'
import { endpoints, type ModelProvider, type ProviderForm } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('system.manage'))

const providers = useResource(() => endpoints.getAdminProviders(), { data: [] as ModelProvider[] })

const dialogOpen = ref(false)
const editingId = ref('')
const formError = ref('')
const form = reactive({ name: '', slug: '', prefixes: '', priority: '0' })

function openCreate() {
  editingId.value = ''
  formError.value = ''
  form.name = ''
  form.slug = ''
  form.prefixes = ''
  form.priority = '0'
  dialogOpen.value = true
}

function openEdit(provider: ModelProvider) {
  editingId.value = provider.id
  formError.value = ''
  form.name = provider.name
  form.slug = provider.slug
  form.prefixes = provider.prefixes.join(', ')
  form.priority = String(provider.priority)
  dialogOpen.value = true
}

function parsePrefixes(value: string): string[] {
  const seen = new Set<string>()
  for (const entry of value.split(',')) {
    const prefix = entry.trim().toLowerCase()
    if (prefix) seen.add(prefix)
  }
  return [...seen]
}

async function save() {
  formError.value = ''
  const name = form.name.trim()
  if (!name) { formError.value = t('admin.nameRequired'); return }

  const slug = form.slug.trim().toLowerCase()
  if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(slug)) { formError.value = t('admin.slugRequired'); return }

  const prefixes = parsePrefixes(form.prefixes)
  if (!prefixes.length) { formError.value = t('admin.prefixesRequired'); return }

  const priority = Number(form.priority)
  if (!Number.isInteger(priority) || priority < 0 || priority > 10000) {
    formError.value = t('admin.priorityInvalid')
    return
  }

  const payload: ProviderForm = { name, slug, prefixes, priority }
  if (editingId.value) payload.id = editingId.value

  const ok = await run(() => endpoints.saveProvider(payload))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.providerSaved'))
  dialogOpen.value = false
  await providers.refresh()
}

const removeTarget = ref<ModelProvider | null>(null)
const removeOpen = ref(false)

function askRemove(provider: ModelProvider) {
  removeTarget.value = provider
  removeOpen.value = true
}

async function remove() {
  const target = removeTarget.value
  if (!target) return
  const ok = await run(() => endpoints.removeProvider(target.id))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.providerDeleted'))
  removeOpen.value = false
  await providers.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.providersLead')">
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="providers.refresh()">{{ t('common.refresh') }}</UiButton>
        <UiButton size="sm" @click="openCreate">{{ t('admin.createProvider') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <ConsoleOpsListState
      :pending="providers.pending.value"
      :error="providers.error.value"
      :empty="!providers.data.value.data.length"
      :empty-icon="Blocks"
      :empty-title="t('admin.providersEmptyTitle')"
      :empty-description="t('admin.providersEmptyBody')"
    >
      <UiTable>
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('admin.slug') }}</th>
            <th>{{ t('admin.prefixes') }}</th>
            <th class="num">{{ t('admin.priority') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="provider in providers.data.value.data" :key="provider.id">
            <td class="font-medium text-ink">{{ provider.name }}</td>
            <td class="font-mono text-[13px] text-muted">{{ provider.slug }}</td>
            <td>
              <div v-if="provider.prefixes.length" class="flex flex-wrap gap-1">
                <UiBadge v-for="prefix in provider.prefixes" :key="prefix" tone="outline">{{ prefix }}</UiBadge>
              </div>
              <span v-else class="text-faint">{{ t('common.none') }}</span>
            </td>
            <td class="num">{{ provider.priority }}</td>
            <td>
              <div class="flex items-center gap-1">
                <UiButton variant="ghost" size="sm" @click="openEdit(provider)">{{ t('common.edit') }}</UiButton>
                <UiButton variant="ghost" size="sm" @click="askRemove(provider)">{{ t('common.delete') }}</UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiSlidePanel
      v-model:open="dialogOpen"
      :title="editingId ? t('admin.editProvider') : t('admin.createProvider')"
    >
      <div class="space-y-4">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('common.name')" required>
            <UiInput v-model="form.name" />
          </UiField>
          <UiField :label="t('admin.slug')" :hint="t('admin.slugHint')" required>
            <UiInput v-model="form.slug" mono />
          </UiField>
        </div>

        <UiField :label="t('admin.prefixes')" :hint="t('admin.prefixesHint')" required>
          <UiInput v-model="form.prefixes" mono />
        </UiField>

        <UiField :label="t('admin.priority')">
          <UiInput v-model="form.priority" type="number" mono />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiDialog v-model:open="removeOpen" size="sm" :title="t('common.delete')">
      <p class="text-sm text-muted">{{ t('admin.confirmDeleteProvider', { name: removeTarget?.name ?? '' }) }}</p>
      <template #footer>
        <UiButton variant="secondary" @click="removeOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton variant="danger" :loading="busy" @click="remove">{{ t('common.delete') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
