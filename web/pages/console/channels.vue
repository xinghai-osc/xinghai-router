<script setup lang="ts">
import { Server, KeyRound, Play, ArrowRightLeft } from 'lucide-vue-next'
import { endpoints, type Channel, type ChannelForm, type ChannelKey, type ChannelKeyTestResult, type Group, type ModelRoute, type ModelRouteForm } from '~/src/api'
import { formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('channels.read'))
const canManage = computed(() => can('channels.manage'))

const PROVIDERS = ['openai', 'ollama', 'kimi', 'opencode_go', 'anthropic']
const KEY_TYPES = [
  { value: 'single', label: t('admin.singleKey') },
  { value: 'multi', label: t('admin.multiKey') },
]

const channels = useResource(() => endpoints.getAdminChannels(), { data: [] as Channel[] })
const groups = useResource(
  () => (can('users.read') ? endpoints.getAdminGroups() : Promise.resolve({ data: [] as Group[] })),
  { data: [] as Group[] },
)

const search = ref('')
const selected = ref<Set<string>>(new Set())

function toggleSelected(id: string) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

const groupOptions = computed(() => groups.data.value.data)
const groupNames = computed(() => new Map(groupOptions.value.map(group => [group.id, group.name])))
const providerOptions = computed(() => PROVIDERS.map(value => ({ value, label: value })))
const keyTypeOptions = computed(() => KEY_TYPES)

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) return channels.data.value.data
  return channels.data.value.data.filter(channel =>
    channel.name.toLowerCase().includes(term) ||
    channel.base_url.toLowerCase().includes(term) ||
    channel.models.some(m => m.toLowerCase().includes(term)))
})

const allSelected = computed(() => filtered.value.length > 0 && filtered.value.every(ch => selected.value.has(ch.id)))

function toggleAll() {
  if (allSelected.value) selected.value.clear()
  else selected.value = new Set(filtered.value.map(ch => ch.id))
}

const batchConfirmOpen = ref(false)
const batchEnabled = ref(false)

function openBatchToggle(enabled: boolean) {
  batchEnabled.value = enabled
  batchConfirmOpen.value = true
}

async function confirmBatchToggle() {
  const ids = [...selected.value]
  const enabled = batchEnabled.value
  const ok = await run(() => endpoints.batchToggleChannels(ids, enabled))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.batchToggleDone', { action: enabled ? t('common.enable') : t('common.disable'), count: ids.length }))
  batchConfirmOpen.value = false
  selected.value.clear()
  await channels.refresh()
}

const dialogOpen = ref(false)
const editingId = ref('')
const formError = ref('')
const fetching = ref(false)
const form = reactive({
  name: '',
  provider: 'openai',
  base_url: '',
  key_type: 'single' as 'single' | 'multi',
  api_keys: '',
  models: '',
  priority: '0',
  groups: [] as string[],
})

const keysDialogOpen = ref(false)
const keysChannelId = ref('')
const keysChannelName = ref('')
const channelKeys = useResource(() => Promise.resolve({ data: [] as ChannelKey[] }), { data: [] as ChannelKey[] })

const routesDialogOpen = ref(false)
const routesChannelId = ref('')
const routesChannelName = ref('')
const channelRoutes = useResource(() => Promise.resolve({ data: [] as ModelRoute[] }), { data: [] as ModelRoute[] })
const routeDialogOpen = ref(false)
const editingRouteId = ref('')
const routeFormError = ref('')
const routeForm = reactive({
  public_model: '',
  upstream_model: '',
  priority: '0',
  weight: '100',
  hidden: false,
})

function openCreate() {
  editingId.value = ''
  formError.value = ''
  form.name = ''
  form.provider = 'openai'
  form.base_url = ''
  form.key_type = 'single'
  form.api_keys = ''
  form.models = ''
  form.priority = '0'
  form.groups = []
  dialogOpen.value = true
}

function openEdit(channel: Channel) {
  editingId.value = channel.id
  formError.value = ''
  form.name = channel.name
  form.provider = channel.provider
  form.base_url = channel.base_url
  form.key_type = channel.key_type
  form.api_keys = ''
  form.models = channel.models.join('\n')
  form.priority = String(channel.priority)
  form.groups = [...channel.groups]
  dialogOpen.value = true
}

function parseModels(value: string): string[] {
  const seen = new Set<string>()
  for (const entry of value.split(/[\n,]/)) {
    const model = entry.trim()
    if (model) seen.add(model)
  }
  return [...seen]
}

function parseApiKeys(value: string): string[] {
  const seen = new Set<string>()
  for (const entry of value.split('\n')) {
    const key = entry.trim()
    if (key) seen.add(key)
  }
  return [...seen]
}

/** Mirrors the server rule: HTTPS to a public host, or HTTP to loopback, never a trailing /v1. */
function validateBaseUrl(value: string): string {
  if (!value) return t('admin.baseUrlRequired')
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return t('admin.baseUrlInvalid')
  }
  const host = parsed.hostname.toLowerCase().replace(/^\[|\]$/g, '')
  const loopback = host === 'localhost' || host === '127.0.0.1' || host === '::1'
  if (parsed.protocol === 'http:') {
    if (!loopback) return t('admin.baseUrlInvalid')
  } else if (parsed.protocol === 'https:') {
    if (loopback) return t('admin.baseUrlInvalid')
  } else {
    return t('admin.baseUrlInvalid')
  }
  if (/\/v1\/?$/.test(parsed.pathname)) return t('admin.baseUrlTrailingV1')
  return ''
}

async function fetchModels() {
  const baseUrl = form.base_url.trim()
  const apiKey = parseApiKeys(form.api_keys)[0] || ''
  formError.value = ''
  if (!baseUrl || !apiKey) { formError.value = t('admin.fetchModelsNeedInput'); return }
  const urlError = validateBaseUrl(baseUrl)
  if (urlError) { formError.value = urlError; return }

  fetching.value = true
  try {
    const result = await endpoints.fetchChannelModels(baseUrl, apiKey)
    form.models = result.models.join('\n')
    toast.success(t('admin.fetchModelsDone', { count: result.models.length }))
  } catch (cause) {
    formError.value = cause instanceof Error ? cause.message : t('common.actionFailed')
  } finally {
    fetching.value = false
  }
}

async function save() {
  formError.value = ''
  const name = form.name.trim()
  if (!name) { formError.value = t('admin.nameRequired'); return }

  const baseUrl = form.base_url.trim().replace(/\/+$/, '')
  const urlError = validateBaseUrl(baseUrl)
  if (urlError) { formError.value = urlError; return }

  const models = parseModels(form.models)
  if (!models.length) { formError.value = t('admin.modelsRequired'); return }

  const apiKeys = parseApiKeys(form.api_keys)
  if (!editingId.value && !apiKeys.length) { formError.value = t('admin.apiKeyRequired'); return }
  if (apiKeys.length) {
    if (form.key_type === 'single' && apiKeys.length > 1) {
      formError.value = t('admin.singleKeyOnlyOne')
      return
    }
  }

  const priority = Number(form.priority)
  if (!Number.isInteger(priority) || priority < -10000 || priority > 10000) {
    formError.value = t('admin.priorityInvalid')
    return
  }

  const payload: ChannelForm = {
    name,
    provider: form.provider,
    base_url: baseUrl,
    key_type: form.key_type,
    api_keys: apiKeys.join('\n'),
    models,
    priority,
    groups: [...form.groups],
  }
  if (!apiKeys.length) {
    delete (payload as Partial<ChannelForm>).api_keys
  }

  const id = editingId.value

  const ok = await run(async () => {
    if (!id) return endpoints.createChannel(payload)
    await endpoints.updateChannel(id, payload)
    await endpoints.updateChannelGroups(id, payload.groups)
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }

  toast.success(t('admin.channelSaved'))
  dialogOpen.value = false
  await channels.refresh()
}

async function toggle(channel: Channel, enabled: boolean) {
  const ok = await run(() => endpoints.toggleChannel(channel.id, enabled))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(enabled ? t('admin.channelEnabled') : t('admin.channelDisabled'))
  await channels.refresh()
}

async function openKeys(channel: Channel) {
  keysChannelId.value = channel.id
  keysChannelName.value = channel.name
  channelKeys.data.value.data = []
  const ok = await run(async () => {
    const result = await endpoints.getChannelKeys(channel.id)
    channelKeys.data.value.data = result.data
  })
  if (ok) keysDialogOpen.value = true
}

async function testKey(key: ChannelKey) {
  let result: ChannelKeyTestResult | undefined
  const ok = await run(async () => {
    result = await endpoints.testChannelKey(keysChannelId.value, key.id)
  })
  if (!ok || !result) { toast.error(t('common.actionFailed')); return }
  if (result.success) {
    toast.success(t('admin.keyTestSuccess', { status_code: result.status_code, latency_ms: result.latency_ms }))
  } else if (result.auto_disabled) {
    toast.error(t('admin.keyTestFailedAndDisabled', { reason: result.reason ?? '' }))
  } else {
    toast.error(t('admin.keyTestFailed', { status_code: result.status_code, reason: result.reason ?? '' }))
  }
  const refreshed = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = refreshed.data
  await channels.refresh()
}

function resetRouteForm() {
  routeFormError.value = ''
  routeForm.public_model = ''
  routeForm.upstream_model = ''
  routeForm.priority = '0'
  routeForm.weight = '100'
  routeForm.hidden = false
}

function openRoutes(channel: Channel) {
  routesChannelId.value = channel.id
  routesChannelName.value = channel.name
  channelRoutes.data.value.data = channel.model_routes ?? []
  routesDialogOpen.value = true
}

async function refreshRoutes() {
  if (!routesChannelId.value) return
  const result = await endpoints.getChannelRoutes(routesChannelId.value)
  channelRoutes.data.value.data = result.data
  await channels.refresh()
}

function openCreateRoute() {
  editingRouteId.value = ''
  resetRouteForm()
  routeDialogOpen.value = true
}

function openEditRoute(route: ModelRoute) {
  editingRouteId.value = route.id
  routeFormError.value = ''
  routeForm.public_model = route.public_model
  routeForm.upstream_model = route.upstream_model
  routeForm.priority = String(route.priority)
  routeForm.weight = String(route.weight)
  routeForm.hidden = route.hidden
  routeDialogOpen.value = true
}

async function saveRoute() {
  routeFormError.value = ''
  const publicModel = routeForm.public_model.trim()
  const upstreamModel = routeForm.upstream_model.trim()
  if (!publicModel || !upstreamModel) {
    routeFormError.value = t('admin.modelRequired')
    return
  }
  const priority = Number(routeForm.priority)
  const weight = Number(routeForm.weight)
  if (!Number.isInteger(priority) || priority < -10000 || priority > 10000) {
    routeFormError.value = t('admin.priorityInvalid')
    return
  }
  if (!Number.isInteger(weight) || weight < 0 || weight > 10000) {
    routeFormError.value = t('admin.weightInvalid')
    return
  }

  const payload: ModelRouteForm = {
    public_model: publicModel,
    upstream_model: upstreamModel,
    priority,
    weight,
    hidden: routeForm.hidden,
  }

  const ok = await run(async () => {
    if (!editingRouteId.value) {
      await endpoints.createChannelRoute(routesChannelId.value, payload)
    } else {
      await endpoints.updateChannelRoute(routesChannelId.value, editingRouteId.value, payload)
    }
  })
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.modelRouteSaved'))
  routeDialogOpen.value = false
  await refreshRoutes()
}

async function toggleRouteHidden(route: ModelRoute) {
  const ok = await run(() => endpoints.updateChannelRoute(routesChannelId.value, route.id, { hidden: !route.hidden }))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  await refreshRoutes()
}

async function deleteRoute(route: ModelRoute) {
  if (!confirm(t('admin.confirmDeleteModelRoute'))) return
  const ok = await run(() => endpoints.deleteChannelRoute(routesChannelId.value, route.id))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.modelRouteDeleted'))
  await refreshRoutes()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.channelsLead')">
      <template #actions>
        <ConsoleOpsSearch v-model="search" :placeholder="t('admin.channelsSearchPlaceholder')" />
        <UiButton variant="secondary" size="sm" @click="channels.refresh()">{{ t('common.refresh') }}</UiButton>
        <template v-if="canManage && selected.size > 0">
          <UiBadge tone="outline" class="text-xs">{{ selected.size }}</UiBadge>
          <UiButton variant="secondary" size="sm" @click="openBatchToggle(true)">{{ t('admin.batchEnable') }}</UiButton>
          <UiButton variant="secondary" size="sm" @click="openBatchToggle(false)">{{ t('admin.batchDisable') }}</UiButton>
        </template>
        <UiButton v-if="canManage" size="sm" @click="openCreate">{{ t('admin.createChannel') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <UiAlert v-if="!canManage" tone="info">{{ t('admin.readOnlyNotice') }}</UiAlert>

    <ConsoleOpsListState
      :pending="channels.pending.value"
      :error="channels.error.value"
      :empty="!channels.data.value.data.length"
      :empty-icon="Server"
      :empty-title="t('admin.channelsEmptyTitle')"
      :empty-description="t('admin.channelsEmptyBody')"
    >
      <div v-if="!filtered.length" class="rounded-card border border-line bg-surface">
        <UiEmptyState :title="t('admin.noResultsTitle')" :description="t('admin.noResultsBody')" />
      </div>

      <UiTable v-else>
        <thead>
          <tr>
            <th class="w-10">
              <UiCheckbox v-if="canManage" :model-value="allSelected" @change="toggleAll" />
            </th>
            <th>{{ t('admin.channelId') }}</th>
            <th>{{ t('admin.channelName') }}</th>
            <th>{{ t('admin.provider') }}</th>
            <th>{{ t('admin.baseUrl') }}</th>
            <th class="num">{{ t('admin.modelCount') }}</th>
            <th class="num">{{ t('admin.keyCount') }}</th>
            <th>{{ t('admin.keyType') }}</th>
            <th class="num">{{ t('admin.routeCount') }}</th>
            <th class="num">{{ t('admin.priority') }}</th>
            <th>{{ t('admin.groups') }}</th>
            <th>{{ t('common.status') }}</th>
            <th v-if="canManage">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="channel in filtered" :key="channel.id">
            <td>
              <UiCheckbox v-if="canManage" :model-value="selected.has(channel.id)" @change="toggleSelected(channel.id)" />
            </td>
            <td class="font-mono text-[13px] text-faint">{{ channel.id }}</td>
            <td class="font-medium text-ink">
              <div class="flex items-center gap-2">
                {{ channel.name }}
              </div>
            </td>
            <td><UiBadge tone="outline">{{ channel.provider }}</UiBadge></td>
            <td class="max-w-64 truncate font-mono text-[13px] text-muted">{{ channel.base_url }}</td>
            <td class="num">{{ formatNumber(channel.models.length) }}</td>
            <td class="num">{{ channel.key_count }}</td>
            <td>
              <UiBadge tone="outline" :class="channel.key_type === 'multi' ? 'text-warn' : ''">
                {{ channel.key_type === 'multi' ? t('admin.multiKey') : t('admin.singleKey') }}
              </UiBadge>
            </td>
            <td class="num">{{ formatNumber((channel.model_routes ?? []).length) }}</td>
            <td class="num">{{ channel.priority }}</td>
            <td>
              <div v-if="channel.groups.length" class="flex flex-wrap gap-1">
                <UiBadge v-for="id in channel.groups" :key="id" tone="outline">{{ groupNames.get(id) ?? id }}</UiBadge>
              </div>
              <span v-else class="text-faint">{{ t('common.all') }}</span>
            </td>
            <td>
              <div class="flex items-center gap-2">
                <UiSwitch
                  :model-value="channel.enabled"
                  :disabled="!canManage || busy"
                  :label="channel.enabled ? t('common.disable') : t('common.enable')"
                  @update:model-value="toggle(channel, $event)"
                />
                <UiTooltip v-if="channel.auto_disabled" :content="channel.disabled_reason || t('admin.autoDisabled')">
                  <UiBadge tone="warn" dot>{{ t('admin.autoDisabled') }}</UiBadge>
                </UiTooltip>
              </div>
            </td>
            <td v-if="canManage">
              <div class="flex items-center gap-1">
                <UiButton variant="ghost" size="sm" @click="openEdit(channel)">{{ t('common.edit') }}</UiButton>
                <UiButton variant="ghost" size="sm" @click="openKeys(channel)">
                  <KeyRound class="h-4 w-4" />
                </UiButton>
                <UiButton variant="ghost" size="sm" @click="openRoutes(channel)">
                  <ArrowRightLeft class="h-4 w-4" />
                </UiButton>
              </div>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiSlidePanel
      v-model:open="dialogOpen"
      size="lg"
      :title="editingId ? t('admin.editChannel') : t('admin.createChannel')"
    >
      <div class="space-y-4">
        <UiAlert v-if="formError" tone="danger">{{ formError }}</UiAlert>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('admin.channelName')" required>
            <UiInput v-model="form.name" />
          </UiField>
          <UiField :label="t('admin.provider')" required>
            <UiSelect v-model="form.provider" :options="providerOptions" :placeholder="t('common.selectPlaceholder')" />
          </UiField>
        </div>

        <UiField :label="t('admin.baseUrl')" :hint="t('admin.baseUrlHint')" required>
          <UiInput v-model="form.base_url" mono :placeholder="t('admin.baseUrlPlaceholder')" />
        </UiField>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('admin.keyType')" required>
            <UiSelect v-model="form.key_type" :options="keyTypeOptions" />
          </UiField>
          <UiField :label="t('admin.priority')">
            <UiInput v-model="form.priority" type="number" mono />
          </UiField>
        </div>

        <UiField
          :label="t('admin.apiKey')"
          :hint="editingId ? t('admin.apiKeyHintEdit') : t('admin.apiKeyHintCreate')"
          :required="!editingId"
        >
          <UiTextarea v-model="form.api_keys" mono :rows="3" :placeholder="t('admin.apiKeysPlaceholder')" />
        </UiField>

        <UiField :label="t('admin.models')" :hint="t('admin.modelsHint')" required>
          <div class="space-y-2">
            <UiTextarea v-model="form.models" mono :rows="6" :placeholder="t('admin.modelsPlaceholder')" />
            <UiButton variant="secondary" size="sm" :loading="fetching" @click="fetchModels">
              {{ t('admin.fetchModels') }}
            </UiButton>
          </div>
        </UiField>

        <UiField :label="t('admin.groups')" :hint="t('admin.groupsHint')">
          <ConsoleOpsGroupPicker v-model="form.groups" :options="groupOptions" />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel
      v-model:open="keysDialogOpen"
      size="md"
      :title="t('admin.manageKeys')"
    >
      <p class="text-sm text-muted mb-4">{{ keysChannelName }}</p>

      <div v-if="!channelKeys.data.value.data.length" class="py-4 text-center text-muted text-sm">
        <p>{{ t('admin.noKeys') }}</p>
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="key in channelKeys.data.value.data" :key="key.id"
          class="flex items-center justify-between rounded-control border border-line px-3 py-2"
        >
          <div class="flex items-center gap-2 min-w-0">
            <KeyRound class="h-4 w-4 text-faint shrink-0" />
            <span class="text-sm font-medium text-ink truncate">{{ key.name }}</span>
            <UiBadge v-if="!key.enabled" tone="danger" class="shrink-0">{{ t('common.disabled') }}</UiBadge>
            <UiTooltip v-else-if="key.last_error" :content="key.last_error">
              <UiBadge tone="warn" class="shrink-0">{{ t('admin.lastTestFailed') }}</UiBadge>
            </UiTooltip>
          </div>
          <UiButton variant="secondary" size="sm" :loading="busy" :disabled="busy" @click="testKey(key)">
            <Play class="h-4 w-4" />
            <span class="ml-1">{{ t('admin.testKey') }}</span>
          </UiButton>
        </div>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="keysDialogOpen = false">{{ t('common.close') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel
      v-model:open="routesDialogOpen"
      size="md"
      :title="t('admin.modelRoutes')"
    >
      <p class="text-sm text-muted mb-4">{{ routesChannelName }}</p>

      <div v-if="!channelRoutes.data.value.data.length" class="py-4 text-center text-muted text-sm">
        <p>{{ t('admin.noRoutes') }}</p>
        <p class="mt-1">{{ t('admin.routesEmptyHint') }}</p>
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="route in channelRoutes.data.value.data" :key="route.id"
          class="flex items-center justify-between rounded-control border border-line px-3 py-2"
        >
          <div class="flex items-center gap-2 min-w-0">
            <ArrowRightLeft class="h-4 w-4 text-faint shrink-0" />
            <div class="flex flex-col min-w-0">
              <span class="text-sm font-medium text-ink truncate">{{ route.public_model }}</span>
              <span class="text-xs text-muted truncate">{{ route.upstream_model }}</span>
            </div>
            <UiBadge v-if="!route.enabled" tone="danger" class="shrink-0">{{ t('common.disabled') }}</UiBadge>
            <UiBadge v-if="route.hidden" tone="outline" class="shrink-0">{{ t('admin.hidden') }}</UiBadge>
          </div>
          <div class="flex items-center gap-1 shrink-0">
            <UiSwitch
              :model-value="route.hidden"
              size="sm"
              :disabled="busy"
              @update:model-value="toggleRouteHidden(route)"
            />
            <UiButton variant="ghost" size="sm" :disabled="busy" @click="openEditRoute(route)">
              {{ t('common.edit') }}
            </UiButton>
            <UiButton variant="danger" size="sm" :disabled="busy" @click="deleteRoute(route)">
              {{ t('common.delete') }}
            </UiButton>
          </div>
        </div>
      </div>

      <div class="mt-4">
        <UiButton size="sm" @click="openCreateRoute">{{ t('admin.createModelRoute') }}</UiButton>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="routesDialogOpen = false">{{ t('common.close') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel
      v-model:open="routeDialogOpen"
      size="sm"
      :title="editingRouteId ? t('admin.editModelRoute') : t('admin.createModelRoute')"
    >
      <div class="space-y-4">
        <UiAlert v-if="routeFormError" tone="danger">{{ routeFormError }}</UiAlert>

        <UiField :label="t('admin.publicModel')" required>
          <UiInput v-model="routeForm.public_model" mono />
        </UiField>

        <UiField :label="t('admin.upstreamModel')" required>
          <UiInput v-model="routeForm.upstream_model" mono />
        </UiField>

        <div class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('admin.priority')">
            <UiInput v-model="routeForm.priority" type="number" mono />
          </UiField>
          <UiField :label="t('admin.weight')">
            <UiInput v-model="routeForm.weight" type="number" mono />
          </UiField>
        </div>

        <UiField :label="t('admin.hidden')" :hint="t('admin.hiddenHint')">
          <UiSwitch v-model="routeForm.hidden" />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="routeDialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="saveRoute">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiDialog
      v-model:open="batchConfirmOpen"
      size="sm"
      :title="batchEnabled ? t('admin.batchEnable') : t('admin.batchDisable')"
    >
      <p class="text-sm text-muted">
        {{ t('admin.confirmBatchToggle', { toggle: batchEnabled ? t('common.enable') : t('common.disable'), count: selected.size }) }}
      </p>
      <template #footer>
        <UiButton variant="secondary" @click="batchConfirmOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="confirmBatchToggle">{{ t('common.confirm') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
