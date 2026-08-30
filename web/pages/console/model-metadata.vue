<script setup lang="ts">
import { Database } from 'lucide-vue-next'
import { endpoints, type CatalogModel, type ModelMetadata, type ModelMetadataForm } from '~/src/api'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { settings: site } = useSiteSettings()
const { toast } = useToast()
const { busy, error: actionError, run } = useAction()

useHead({ title: () => `${t('admin.modelMetadataTitle')} · ${site.value.name}` })

const metadata = useResource(() => endpoints.getAdminModelMetadata(), { data: [] as ModelMetadata[] })
const catalog = useResource(() => endpoints.getModelCatalog(), { data: [] as CatalogModel[], groups: [] })

const dialogOpen = ref(false)
const removing = ref<ModelMetadata | null>(null)
const editing = ref<ModelMetadata | null>(null)
const form = reactive<ModelMetadataForm>({ model: '', description: '', input_modalities: [], output_modalities: [], context_window: null })
const inputModalities = ref('')
const outputModalities = ref('')
const contextWindow = ref('')
const search = ref('')
const MAX_CONTEXT_WINDOW = 1_000_000_000_000

const modelOptions = computed(() => {
  const current = editing.value?.model
  const configured = new Set(metadata.data.value.data.map(item => item.model))
  return catalog.data.value.data
    .map(model => model.model)
    .filter(model => !configured.has(model) || model === current)
    .sort()
    .map(model => ({ value: model, label: model }))
})
const rows = computed(() => {
  const saved = new Map(metadata.data.value.data.map(item => [item.model, item]))
  const models = catalog.data.value.data.map(model => saved.get(model.model) ?? ({ id: '', model: model.model, description: '', input_modalities: [], output_modalities: [], context_window: null } as ModelMetadata))
  for (const item of metadata.data.value.data) if (!models.some(model => model.model === item.model)) models.push(item)
  const query = search.value.trim().toLowerCase()
  return query ? models.filter(model => model.model.toLowerCase().includes(query) || model.description.toLowerCase().includes(query)) : models
})

function openCreate(model = '') {
  editing.value = null
  form.model = model
  form.description = ''
  inputModalities.value = ''
  outputModalities.value = ''
  contextWindow.value = ''
  dialogOpen.value = true
}

function openEdit(item: ModelMetadata) {
  if (!item.id) {
    openCreate(item.model)
    return
  }
  editing.value = item
  form.model = item.model
  form.description = item.description
  inputModalities.value = item.input_modalities.join(', ')
  outputModalities.value = item.output_modalities.join(', ')
  contextWindow.value = item.context_window == null ? '' : String(item.context_window)
  dialogOpen.value = true
}

function parseModalities(value: string) {
  return [...new Set(value.split(',').map(item => item.trim().toLowerCase()).filter(Boolean))]
}

function parseContext(value: string): number | null {
  const raw = value.trim()
  if (!/^\d+$/.test(raw)) return null
  const parsed = Number(raw)
  return Number.isSafeInteger(parsed) && parsed > 0 && parsed <= MAX_CONTEXT_WINDOW ? parsed : null
}

async function save() {
  if (!form.model.trim()) {
    toast.error(t('admin.modelMetadataModelRequired'))
    return
  }
  if (contextWindow.value.trim() && parseContext(contextWindow.value) == null) {
    toast.error(t('admin.modelMetadataContextInvalid'))
    return
  }
  const allowedModalities = new Set(['text', 'image', 'audio', 'video', 'file'])
  const input = parseModalities(inputModalities.value)
  const output = parseModalities(outputModalities.value)
  if ([...input, ...output].some(value => !allowedModalities.has(value))) {
    toast.error(t('admin.modelMetadataModalitiesInvalid'))
    return
  }
  const payload: ModelMetadataForm = {
    model: form.model.trim(),
    description: form.description.trim(),
    input_modalities: input,
    output_modalities: output,
    context_window: parseContext(contextWindow.value),
  }
  const ok = await run(() => editing.value?.id
    ? endpoints.updateAdminModelMetadata(editing.value.id, payload)
    : endpoints.createAdminModelMetadata(payload))
  if (!ok) {
    toast.error(actionError.value || t('common.actionFailed'))
    return
  }
  toast.success(t('admin.modelMetadataSaved'))
  dialogOpen.value = false
  await metadata.refresh()
}

async function remove() {
  const item = removing.value
  if (!item) return
  const ok = await run(() => endpoints.deleteAdminModelMetadata(item.id))
  if (!ok) {
    toast.error(actionError.value || t('common.actionFailed'))
    return
  }
  removing.value = null
  toast.success(t('admin.modelMetadataDeleted'))
  await metadata.refresh()
}
</script>

<template>
  <ConsoleSystemGate permission="system.manage">
    <div class="space-y-4">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0 space-y-1">
          <h2 class="text-lg font-semibold text-ink">{{ t('admin.modelMetadataTitle') }}</h2>
          <p class="text-[13px] text-muted">{{ t('admin.modelMetadataLead') }}</p>
        </div>
        <UiButton size="sm" :disabled="!modelOptions.length" @click="openCreate()">{{ t('admin.modelMetadataNew') }}</UiButton>
      </div>

      <ConsoleSystemDataState
        :pending="metadata.pending.value || catalog.pending.value"
        :error="metadata.error.value || catalog.error.value"
        :empty="rows.length === 0"
        :empty-icon="Database"
        :empty-title="search.trim() ? t('admin.modelMetadataSearchEmptyTitle') : t('admin.modelMetadataEmptyTitle')"
        :empty-description="search.trim() ? t('admin.modelMetadataSearchEmptyBody') : t('admin.modelMetadataEmptyBody')"
        :rows="5"
      >
        <UiCard>
          <div class="mb-4 max-w-sm">
            <UiInput v-model="search" :placeholder="t('admin.modelMetadataSearch')" />
          </div>
          <UiTable>
            <thead>
              <tr>
                <th>{{ t('admin.modelMetadataModel') }}</th>
                <th>{{ t('admin.modelMetadataDescription') }}</th>
                <th>{{ t('admin.modelMetadataModalities') }}</th>
                <th class="num">{{ t('admin.modelMetadataContext') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in rows" :key="item.model">
                <td class="font-medium text-ink">{{ item.model }}</td>
                <td class="max-w-[24rem] truncate text-muted">
                  {{ item.description || t('admin.modelMetadataNotConfigured') }}
                </td>
                <td>
                  <div v-if="item.input_modalities.length || item.output_modalities.length" class="flex flex-wrap gap-1">
                    <UiBadge v-for="modality in [...item.input_modalities, ...item.output_modalities]" :key="modality" tone="outline">{{ modality }}</UiBadge>
                  </div>
                  <span v-else class="text-faint">{{ t('admin.modelMetadataNotConfigured') }}</span>
                </td>
                <td class="num">{{ item.context_window ?? t('admin.modelMetadataNotConfigured') }}</td>
                <td>
                  <div class="flex items-center gap-1">
                    <UiButton variant="ghost" size="sm" @click="openEdit(item)">{{ item.description || item.context_window || item.input_modalities.length || item.output_modalities.length ? t('common.edit') : t('admin.modelMetadataConfigure') }}</UiButton>
                    <UiButton v-if="item.id" variant="ghost" size="sm" @click="removing = item">{{ t('common.delete') }}</UiButton>
                  </div>
                </td>
              </tr>
            </tbody>
          </UiTable>
        </UiCard>
      </ConsoleSystemDataState>
    </div>

    <UiDialog v-model:open="dialogOpen" :title="editing ? t('admin.modelMetadataEdit') : t('admin.modelMetadataNew')" size="md">
      <form class="space-y-4" @submit.prevent="save">
        <UiField :label="t('admin.modelMetadataModel')" required for="metadata-model">
          <UiSelect id="metadata-model" v-model="form.model" :options="modelOptions" :disabled="editing !== null" :placeholder="t('admin.modelMetadataModelPlaceholder')" />
        </UiField>
        <UiField :label="t('admin.modelMetadataDescription')" :hint="t('admin.modelMetadataDescriptionHint')" for="metadata-description">
          <UiTextarea id="metadata-description" v-model="form.description" :rows="4" :maxlength="2000" />
        </UiField>
        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('admin.modelMetadataInputModalities')" :hint="t('admin.modelMetadataModalitiesHint')" for="metadata-input">
            <UiInput id="metadata-input" v-model="inputModalities" :placeholder="t('admin.modelMetadataModalitiesPlaceholder')" />
          </UiField>
          <UiField :label="t('admin.modelMetadataOutputModalities')" :hint="t('admin.modelMetadataModalitiesHint')" for="metadata-output">
            <UiInput id="metadata-output" v-model="outputModalities" :placeholder="t('admin.modelMetadataModalitiesPlaceholder')" />
          </UiField>
        </div>
        <UiField :label="t('admin.modelMetadataContext')" :hint="t('admin.modelMetadataContextHint')" for="metadata-context">
          <UiInput id="metadata-context" v-model="contextWindow" type="number" min="1" step="1" :placeholder="t('admin.modelMetadataContextPlaceholder')" />
        </UiField>
        <div class="flex justify-end gap-2">
          <UiButton type="button" variant="secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</UiButton>
          <UiButton type="submit" :loading="busy">{{ t('common.save') }}</UiButton>
        </div>
      </form>
    </UiDialog>

    <ConsoleSystemConfirmDialog
      :open="removing !== null"
      :body="t('admin.modelMetadataDeleteConfirm', { model: removing?.model ?? '' })"
      :busy="busy"
      @update:open="value => { if (!value) removing = null }"
      @confirm="remove"
    />
  </ConsoleSystemGate>
</template>
