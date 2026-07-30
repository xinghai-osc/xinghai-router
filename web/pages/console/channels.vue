<script setup lang="ts">
import { Server, KeyRound } from 'lucide-vue-next'
import { endpoints, type Channel, type ChannelForm, type ChannelKey, type Group } from '~/src/api'
import { formatNumber } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('channels.read'))
const canManage = computed(() => can('channels.manage'))

const PROVIDERS = ['openai', 'ollama', 'kimi', 'opencode_go', 'anthropic']

const channels = useResource(() => endpoints.getAdminChannels(), { data: [] as Channel[] })
const groups = useResource(
  () => (can('users.read') ? endpoints.getAdminGroups() : Promise.resolve({ data: [] as Group[] })),
  { data: [] as Group[] },
)

const search = ref('')

const groupOptions = computed(() => groups.data.value.data)
const groupNames = computed(() => new Map(groupOptions.value.map(group => [group.id, group.name])))
const providerOptions = computed(() => PROVIDERS.map(value => ({ value, label: value })))

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  if (!term) return channels.data.value.data
  return channels.data.value.data.filter(channel =>
    channel.name.toLowerCase().includes(term) || channel.base_url.toLowerCase().includes(term))
})

const dialogOpen = ref(false)
const editingId = ref('')
const formError = ref('')
const fetching = ref(false)
const form = reactive({
  name: '',
  provider: 'openai',
  base_url: '',
  api_key: '',
  api_keys: '',
  models: '',
  priority: '0',
  groups: [] as string[],
})

const keysDialogOpen = ref(false)
const keysChannelId = ref('')
const keysChannelName = ref('')
const channelKeys = useResource(() => Promise.resolve({ data: [] as ChannelKey[] }), { data: [] as ChannelKey[] })
const addKeyDialogOpen = ref(false)
const newKeyName = ref('')
const newKeyValue = ref('')
const migrating = ref(false)

function openCreate() {
  editingId.value = ''
  formError.value = ''
  form.name = ''
  form.provider = 'openai'
  form.base_url = ''
  form.api_key = ''
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
  form.api_key = ''
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
  const apiKey = form.api_key.trim() || parseApiKeys(form.api_keys)[0] || ''
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
  const apiKey = form.api_key.trim()
  if (!editingId.value && !apiKey && !apiKeys.length) { formError.value = t('admin.apiKeyRequired'); return }

  const priority = Number(form.priority)
  if (!Number.isInteger(priority) || priority < -10000 || priority > 10000) {
    formError.value = t('admin.priorityInvalid')
    return
  }

  const payload: ChannelForm = {
    name,
    provider: form.provider,
    base_url: baseUrl,
    api_key: apiKey,
    api_keys: apiKeys.length ? apiKeys : undefined,
    models,
    priority,
    groups: [...form.groups],
  }
  if (!apiKey && !apiKeys.length) {
    delete payload.api_keys
  }
  if (!apiKey) {
    delete payload.api_key
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

async function addKey() {
  const key = newKeyValue.value.trim()
  if (!key) { return }
  const ok = await run(() => endpoints.createChannelKey(keysChannelId.value, { name: newKeyName.value.trim() || 'key', api_key: key }))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.keyCreated'))
  newKeyName.value = ''
  newKeyValue.value = ''
  addKeyDialogOpen.value = false
  const result = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = result.data
}

async function deleteKey(keyId: string) {
  const ok = await run(() => endpoints.deleteChannelKey(keysChannelId.value, keyId))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.keyDeleted'))
  const result = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = result.data
  await channels.refresh()
}

async function toggleKey(key: ChannelKey, enabled: boolean) {
  const ok = await run(() => endpoints.toggleChannelKey(keysChannelId.value, key.id, enabled))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(enabled ? t('admin.keyEnabled') : t('admin.keyDisabled'))
  const result = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = result.data
}

async function migrateKeys() {
  migrating.value = true
  const ok = await run(() => endpoints.migrateChannelKeys(keysChannelId.value))
  migrating.value = false
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.keyMigrated'))
  const result = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = result.data
  await channels.refresh()
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.channelsLead')">
      <template #actions>
        <ConsoleOpsSearch v-model="search" :placeholder="t('admin.channelsSearchPlaceholder')" />
        <UiButton variant="secondary" size="sm" @click="channels.refresh()">{{ t('common.refresh') }}</UiButton>
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
            <th>{{ t('admin.channelName') }}</th>
            <th>{{ t('admin.provider') }}</th>
            <th>{{ t('admin.baseUrl') }}</th>
            <th class="num">{{ t('admin.modelCount') }}</th>
            <th class="num">{{ t('admin.keyCount') }}</th>
            <th class="num">{{ t('admin.priority') }}</th>
            <th>{{ t('admin.groups') }}</th>
            <th>{{ t('common.status') }}</th>
            <th v-if="canManage">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="channel in filtered" :key="channel.id">
            <td class="font-medium text-ink">{{ channel.name }}</td>
            <td><UiBadge tone="outline">{{ channel.provider }}</UiBadge></td>
            <td class="max-w-64 truncate font-mono text-[13px] text-muted">{{ channel.base_url }}</td>
            <td class="num">{{ formatNumber(channel.models.length) }}</td>
            <td class="num">{{ channel.key_count }}</td>
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
              </div>
            </td>
          </tr>
        </tbody>
      </UiTable>
    </ConsoleOpsListState>

    <UiDialog
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

        <UiField
          :label="t('admin.apiKey')"
          :hint="t('admin.apiKeyHintCreate')"
          :required="!editingId"
        >
          <UiInput v-model="form.api_key" type="password" mono autocomplete="off" />
        </UiField>

        <UiField :label="t('admin.apiKeys')" :hint="t('admin.apiKeysHint')">
          <UiTextarea v-model="form.api_keys" mono :rows="3" placeholder="sk-xxx1&#10;sk-xxx2" />
        </UiField>

        <UiField :label="t('admin.models')" :hint="t('admin.modelsHint')" required>
          <div class="space-y-2">
            <UiTextarea v-model="form.models" mono :rows="6" :placeholder="t('admin.modelsPlaceholder')" />
            <UiButton variant="secondary" size="sm" :loading="fetching" @click="fetchModels">
              {{ t('admin.fetchModels') }}
            </UiButton>
          </div>
        </UiField>

        <UiField :label="t('admin.priority')">
          <UiInput v-model="form.priority" type="number" mono />
        </UiField>

        <UiField :label="t('admin.groups')" :hint="t('admin.groupsHint')">
          <ConsoleOpsGroupPicker v-model="form.groups" :options="groupOptions" />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="dialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="save">{{ t('common.save') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog
      v-model:open="keysDialogOpen"
      size="md"
      :title="t('admin.manageKeys')"
    >
      <p class="text-sm text-muted mb-4">{{ keysChannelName }}</p>

      <div v-if="!channelKeys.data.value.data.length" class="py-4 text-center text-muted text-sm">
        <p>{{ t('admin.noKeys') }}</p>
        <p class="mt-1">{{ t('admin.keysEmptyHint') }}</p>
      </div>

      <div v-else class="space-y-2">
        <div
v-for="key in channelKeys.data.value.data" :key="key.id"
          class="flex items-center justify-between rounded-control border border-line px-3 py-2">
          <div class="flex items-center gap-2">
            <KeyRound class="h-4 w-4 text-faint" />
            <span class="text-sm font-medium text-ink">{{ key.name }}</span>
            <UiSwitch
              :model-value="key.enabled"
              size="sm"
              :disabled="busy"
              @update:model-value="toggleKey(key, $event)"
            />
          </div>
          <UiButton variant="danger" size="sm" :disabled="busy" @click="deleteKey(key.id)">
            {{ t('common.delete') }}
          </UiButton>
        </div>
      </div>

      <div class="mt-4 flex flex-wrap gap-2">
        <UiButton size="sm" @click="addKeyDialogOpen = true">{{ t('admin.addKey') }}</UiButton>
        <UiButton variant="secondary" size="sm" :loading="migrating" @click="migrateKeys">{{ t('admin.migrateKeys') }}</UiButton>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="keysDialogOpen = false">{{ t('common.close') }}</UiButton>
      </template>
    </UiDialog>

    <UiDialog
      v-model:open="addKeyDialogOpen"
      size="sm"
      :title="t('admin.addKey')"
    >
      <div class="space-y-4">
        <UiField :label="t('admin.keyName')">
          <UiInput v-model="newKeyName" :placeholder="t('admin.keyNamePlaceholder')" />
        </UiField>
        <UiField :label="t('admin.apiKey')" required>
          <UiInput v-model="newKeyValue" type="password" mono autocomplete="off" />
        </UiField>
      </div>
      <template #footer>
        <UiButton variant="secondary" @click="addKeyDialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="addKey">{{ t('common.save') }}</UiButton>
      </template>
    </UiDialog>
  </div>
</template>
