<script setup lang="ts">
import { ArrowRightLeft } from 'lucide-vue-next'
import { endpoints, type Channel, type ModelRoute } from '~/src/api'
import { formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('routes.manage'))

const routes = useResource(() => endpoints.getModelRoutes(), { data: [] as ModelRoute[] })
const channels = useResource(() => endpoints.getAdminChannels(), { data: [] as Channel[] })

const channelNames = computed(() => new Map(channels.data.value.data.map(c => [c.id, c.name])))

const dialogOpen = ref(false)
const editingId = ref('')
const formError = ref('')
const form = reactive({
  public_model: '',
  upstream_model: '',
  channel_id: '',
  priority: '0',
  weight: '100',
  hidden: false,
})

const search = ref('')

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) return routes.data.value.data
  return routes.data.value.data.filter(r =>
    r.public_model.toLowerCase().includes(term) ||
    r.upstream_model.toLowerCase().includes(term) ||
    r.channel_id.toLowerCase().includes(term))
})

const channelOptions = computed(() =>
  channels.data.value.data.map(c => ({ value: c.id, label: `${c.name} (${c.id.slice(0, 8)}...)` })))

function resetForm() {
  formError.value = ''
  form.public_model = ''
  form.upstream_model = ''
  form.channel_id = ''
  form.priority = '0'
  form.weight = '100'
  form.hidden = false
}

function openCreate() {
  editingId.value = ''
  resetForm()
  dialogOpen.value = true
}

function openEdit(route: ModelRoute) {
  editingId.value = route.id
  formError.value = ''
  form.public_model = route.public_model
  form.upstream_model = route.upstream_model
  form.channel_id = route.channel_id
  form.priority = String(route.priority)
  form.weight = String(route.weight)
  form.hidden = route.hidden
  dialogOpen.value = true
}

async function save() {
  formError.value = ''
  const publicModel = form.public_model.trim()
  const upstreamModel = form.upstream_model.trim()
  const channelId = form.channel_id.trim()
  if (!publicModel || !upstreamModel || !channelId) {
    formError.value = t('common.required')
    return
  }
  const priority = Number(form.priority)
  const weight = Number(form.weight)

  const payload: Record<string, unknown> = {
    public_model: publicModel,
    upstream_model: upstreamModel,
    channel_id: channelId,
    priority,
    weight,
    hidden: form.hidden,
  }

  const ok = await run(async () => {
    if (!editingId.value) {
      await endpoints.createModelRoute(payload)
    } else {
      await endpoints.updateModelRoute(editingId.value, payload)
    }
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.modelRouteSaved'))
  dialogOpen.value = false
  await routes.refresh()
}

async function toggleHidden(route: ModelRoute) {
  const ok = await run(() => endpoints.updateModelRoute(route.id, { hidden: !route.hidden }))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  await routes.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.modelRoutesLead')">
      <template #actions>
        <ConsoleOpsSearch v-model="search" :placeholder="t('admin.modelRoutes')" />
        <UiButton variant="secondary" size="sm" @click="routes.refresh()">{{ t('common.refresh') }}</UiButton>
        <UiButton size="sm" @click="openCreate">{{ t('admin.createModelRoute') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <ConsoleOpsListState
      :pending="routes.pending.value"
      :error="routes.error.value"
      :empty="!routes.data.value.data.length"
      :empty-icon="ArrowRightLeft"
      :empty-title="t('admin.modelRoutesEmptyTitle')"
      :empty-description="t('admin.modelRoutesEmptyBody')"
    >
      <div v-if="!filtered.length" class="rounded-card border border-line bg-surface">
        <UiEmptyState :title="t('admin.noResultsTitle')" :description="t('admin.noResultsBody')" />
      </div>

      <UiTable v-else>
        <thead>
          <tr>
            <th>{{ t('admin.publicModel') }}</th>
            <th>{{ t('admin.upstreamModel') }}</th>
            <th>{{ t('admin.channel') }}</th>
            <th class="num">{{ t('admin.priority') }}</th>
            <th class="num">{{ t('admin.weight') }}</th>
            <th>{{ t('admin.hidden') }}</th>
            <th>{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="route in filtered" :key="route.id">
            <td class="font-medium text-ink">{{ route.public_model }}</td>
            <td class="font-mono text-[13px] text-muted">{{ route.upstream_model }}</td>
            <td class="text-sm text-muted">{{ channelNames.get(route.channel_id) ?? route.channel_id.slice(0, 8) }}</td>
            <td class="num">{{ formatNumber(route.priority) }}</td>
            <td class="num">{{ formatNumber(route.weight) }}</td>
            <td>
              <UiSwitch
                :model-value="route.hidden"
                :disabled="busy"
                @update:model-value="toggleHidden(route)"
              />
            </td>
            <td>
              <UiButton variant="ghost" size="sm" @click="openEdit(route)">{{ t('common.edit') }}</UiButton>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiDialog
      v-model:open="dialogOpen"
      :title="editingId ? t('admin.editModelRoute') : t('admin.createModelRoute')"
    >
      <div class="space-y-4">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>

        <UiField :label="t('admin.publicModel')" required>
          <UiInput v-model="form.public_model" mono />
        </UiField>

        <UiField :label="t('admin.upstreamModel')" required>
          <UiInput v-model="form.upstream_model" mono />
        </UiField>

        <UiField :label="t('admin.channel')" required>
          <UiSelect v-model="form.channel_id" :options="channelOptions" :placeholder="t('common.selectPlaceholder')" />
        </UiField>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('admin.priority')">
            <UiInput v-model="form.priority" type="number" mono />
          </UiField>
          <UiField :label="t('admin.weight')">
            <UiInput v-model="form.weight" type="number" mono />
          </UiField>
        </div>

        <UiField :label="t('admin.hidden')" :hint="t('admin.hiddenHint')">
          <UiSwitch v-model="form.hidden" />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('common.save') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
