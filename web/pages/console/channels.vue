<script setup lang="ts">
import type { Component } from 'vue'
import { Server, KeyRound, Play, ArrowRightLeft, BarChart3, X, ListChecks, SortAsc, Eye, EyeOff, Sparkles, Cpu, Moon, SquareTerminal, Hexagon, Brain, Plug } from 'lucide-vue-next'
import { endpoints, type Channel, type ChannelForm, type ChannelKey, type ChannelKeyTestResult, type ChannelQuota, type ChannelQuotaForm, type ChannelTestResult, type ChannelUsageStats, type Group, type ModelRoute, type ModelRouteForm } from '~/src/api'
import { formatCompact, formatNumber, formatMoney, formatDateTime } from '~/src/format'

definePageMeta({ layout: 'console', middleware: 'console-auth' })

const { t } = useI18n()
const { can } = useAccount()
const { toast } = useToast()
const { busy, run } = useAction()

const allowed = computed(() => can('channels.read'))
const canManage = computed(() => can('channels.manage'))

const PROVIDERS = ['openai', 'ollama', 'kimi', 'opencode_go', 'anthropic', 'deepseek', 'custom']
const KEY_TYPES = [
  { value: 'single', label: t('admin.singleKey') },
  { value: 'multi', label: t('admin.multiKey') },
]

const PROVIDER_ICONS: Record<string, Component> = {
  openai: Sparkles,
  ollama: Cpu,
  kimi: Moon,
  opencode_go: SquareTerminal,
  anthropic: Hexagon,
  deepseek: Brain,
  custom: Plug,
}

const page = ref(1)
const pageSize = ref('50')

function pageQuery(): string {
  const params = new URLSearchParams()
  params.set('page', String(page.value))
  params.set('page_size', pageSize.value)
  return `?${params.toString()}`
}

watch(pageSize, async () => { page.value = 1; await channels.refresh() })

const channels = useResource(() => endpoints.getAdminChannels(pageQuery()), { data: [] as Channel[], total: 0, page: 1, page_size: 50 })
const groups = useResource(
  () => (can('users.read') ? endpoints.getAdminGroups('?page_size=100') : Promise.resolve({ data: [] as Group[], total: 0, page: 1, page_size: 100 })),
  { data: [] as Group[], total: 0, page: 1, page_size: 100 },
)

const search = ref('')
const modelFilter = ref('')
const statusFilter = ref('all')
const typeFilter = ref('all')
const groupFilter = ref('all')
const selected = ref<Set<string>>(new Set())
const batchMode = ref(false)
const idSort = ref(false)
const sensitiveVisible = ref(true)

const STATUS_OPTIONS = computed(() => [
  { value: 'all', label: t('common.all') },
  { value: 'enabled', label: t('common.enabled') },
  { value: 'disabled', label: t('common.disabled') },
  { value: 'auto_disabled', label: t('admin.autoDisabled') },
])
const typeOptions = computed(() => [
  { value: 'all', label: t('common.all') },
  ...PROVIDERS.map(value => ({ value, label: t(`admin.provider_${value}`) })),
])
const providerOptions = computed(() => PROVIDERS.map(value => ({ value, label: t(`admin.provider_${value}`) })))
const formatOptions = computed(() => [
  { value: '', label: t('admin.formatAuto') },
  { value: 'openai', label: t('admin.formatOpenAI') },
  { value: 'anthropic', label: t('admin.formatAnthropic') },
])

function formatRelativeTime(value: string | null): string {
  if (!value) return t('admin.neverTested')
  const time = new Date(value).getTime()
  if (Number.isNaN(time)) return t('admin.neverTested')
  const diff = Math.max(0, Date.now() - time)
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return t('admin.timeJustNow')
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return t('admin.timeMinutesAgo', { count: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t('admin.timeHoursAgo', { count: hours })
  const days = Math.floor(hours / 24)
  return t('admin.timeDaysAgo', { count: days })
}

function formatDuration(ms: number): string {
  if (!ms) return '—'
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.round(ms)}ms`
}

function responseTone(ms: number): 'success' | 'warn' | 'danger' | 'neutral' {
  if (!ms) return 'neutral'
  if (ms < 500) return 'success'
  if (ms < 1500) return 'warn'
  return 'danger'
}

function toggleSelected(id: string) {
  const next = new Set(selected.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selected.value = next
}

const groupOptions = computed(() => groups.data.value.data)
const groupNames = computed(() => new Map(groupOptions.value.map(group => [group.id, group.name])))
const groupFilterOptions = computed(() => [
  { value: 'all', label: t('common.all') },
  ...groupOptions.value.map(group => ({ value: group.id, label: group.name })),
])
const keyTypeOptions = computed(() => KEY_TYPES)

const filtered = computed(() => {
  const term = search.value.trim().toLowerCase()
  const model = modelFilter.value.trim().toLowerCase()
  let list = channels.data.value.data.filter(channel => {
    if (term && !(channel.name.toLowerCase().includes(term) || channel.id.includes(term) || channel.base_url.toLowerCase().includes(term))) return false
    if (model && !channel.models.some(m => m.toLowerCase().includes(model))) return false
    if (statusFilter.value === 'enabled' && !channel.enabled) return false
    if (statusFilter.value === 'disabled' && channel.enabled) return false
    if (statusFilter.value === 'auto_disabled' && !channel.auto_disabled) return false
    if (typeFilter.value !== 'all' && channel.provider !== typeFilter.value) return false
    if (groupFilter.value !== 'all' && !channel.groups.includes(groupFilter.value)) return false
    return true
  })
  if (idSort.value) {
    list = [...list].sort((a, b) => Number(a.id) - Number(b.id))
  }
  return list
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
  test_model: '',
  priority: '0',
  groups: [] as string[],
  auto_disable: true,
  upstream_path: '',
  upstream_format: '',
  delete_fields: '',
  set_fields: '',
})

const keyDraft = ref('')
const keyInputError = ref('')
const keyChips = computed(() => parseApiKeys(form.api_keys))

function addKey() {
  keyInputError.value = ''
  const key = keyDraft.value.trim()
  if (!key) return
  if (key.length > 4096) {
    keyInputError.value = t('admin.apiKeyInvalid')
    return
  }
  const keys = keyChips.value
  if (keys.includes(key)) {
    keyInputError.value = t('admin.duplicateApiKey')
    return
  }
  if (form.key_type === 'single' && keys.length > 0) {
    keyInputError.value = t('admin.singleKeyOnlyOne')
    return
  }
  form.api_keys = [...keys, key].join('\n')
  keyDraft.value = ''
}

function removeKey(index: number) {
  form.api_keys = keyChips.value.filter((_, i) => i !== index).join('\n')
  keyInputError.value = ''
}

const keysDialogOpen = ref(false)
const keysChannelId = ref('')
const keysChannelName = ref('')
const channelKeys = useResource(() => Promise.resolve({ data: [] as ChannelKey[] }), { data: [] as ChannelKey[] })

const keyDialogOpen = ref(false)
const editingKeyId = ref('')
const keyFormError = ref('')
const keyForm = reactive({
  name: '',
  api_key: '',
  priority: '100',
})

const keyRevealOpen = ref(false)
const revealingKey = ref<ChannelKey | null>(null)
const revealedSecret = ref('')
const revealError = ref('')

function openRevealKey(key: ChannelKey) {
  revealingKey.value = key
  revealedSecret.value = ''
  revealError.value = ''
  keyRevealOpen.value = true
  loadRevealedSecret()
}

async function loadRevealedSecret() {
  const target = revealingKey.value
  if (!target) return
  revealError.value = ''
  const ok = await run(async () => {
    revealedSecret.value = (await endpoints.revealChannelKey(keysChannelId.value, target.id)).key
  })
  if (!ok) revealError.value = t('console.keyRevealFailed')
}

function openCreateKey() {
  editingKeyId.value = ''
  keyFormError.value = ''
  keyForm.name = ''
  keyForm.api_key = ''
  keyForm.priority = '100'
  keyDialogOpen.value = true
}

function openEditKey(key: ChannelKey) {
  editingKeyId.value = key.id
  keyFormError.value = ''
  keyForm.name = key.name
  keyForm.api_key = ''
  keyForm.priority = String(key.priority)
  keyDialogOpen.value = true
}

async function saveKey() {
  keyFormError.value = ''
  const name = keyForm.name.trim()
  if (name.length > 100) {
    keyFormError.value = t('admin.nameRequired')
    return
  }
  const priority = Number(keyForm.priority)
  if (!Number.isInteger(priority) || priority < -10000 || priority > 10000) {
    keyFormError.value = t('admin.priorityInvalid')
    return
  }
  if (!editingKeyId.value) {
    const apiKey = keyForm.api_key.trim()
    if (!apiKey || apiKey.length > 4096) {
      keyFormError.value = t('admin.apiKeyRequired')
      return
    }
    const ok = await run(() => endpoints.createChannelKey(keysChannelId.value, { name, api_key: apiKey, priority }))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    toast.success(t('admin.keyCreated'))
  } else {
    if (!name) {
      keyFormError.value = t('admin.nameRequired')
      return
    }
    const ok = await run(() => endpoints.updateChannelKey(keysChannelId.value, editingKeyId.value, { name, priority }))
    if (!ok) { toast.error(t('common.actionFailed')); return }
    toast.success(t('admin.keySaved'))
  }
  keyDialogOpen.value = false
  const refreshed = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = refreshed.data
  await channels.refresh()
}

async function toggleKey(key: ChannelKey) {
  const ok = await run(() => endpoints.toggleChannelKey(keysChannelId.value, key.id, !key.enabled))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(key.enabled ? t('admin.keyDisabled') : t('admin.keyEnabled'))
  const refreshed = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = refreshed.data
  await channels.refresh()
}

async function deleteKey(key: ChannelKey) {
  if (!confirm(t('admin.confirmDeleteKey'))) return
  const ok = await run(() => endpoints.deleteChannelKey(keysChannelId.value, key.id))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.keyDeleted'))
  const refreshed = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = refreshed.data
  await channels.refresh()
}

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
  keyDraft.value = ''
  keyInputError.value = ''
  form.models = ''
  form.test_model = ''
  form.priority = '0'
  form.groups = []
  form.auto_disable = true
  form.upstream_path = ''
  form.upstream_format = ''
  form.delete_fields = ''
  form.set_fields = ''
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
  keyDraft.value = ''
  keyInputError.value = ''
  form.models = channel.models.join('\n')
  form.priority = String(channel.priority)
  form.groups = [...channel.groups]
  form.auto_disable = channel.auto_disable
  form.upstream_path = channel.upstream_path ?? ''
  form.upstream_format = channel.upstream_format ?? ''
  form.delete_fields = (channel.request_overrides?.delete ?? []).join('\n')
  const setFields = channel.request_overrides?.set ?? {}
  form.set_fields = Object.keys(setFields).length ? JSON.stringify(setFields, null, 2) : ''
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

function parseOverrideFields(value: string): string[] {
  const seen = new Set<string>()
  for (const entry of value.split(/[\n,]/)) {
    const field = entry.trim()
    if (field) seen.add(field)
  }
  return [...seen]
}

/** Returns null when the text is non-empty but not a JSON object. */
function parseOverrideSet(value: string): Record<string, unknown> | null {
  const trimmed = value.trim()
  if (!trimmed) return {}
  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return null
  }
  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) return null
  return parsed as Record<string, unknown>
}

/** Mirrors the server rule: HTTP or HTTPS to any host, never a trailing /v1. */
function validateBaseUrl(value: string): string {
  if (!value) return t('admin.baseUrlRequired')
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return t('admin.baseUrlInvalid')
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
    return t('admin.baseUrlInvalid')
  }
  if (/\/v1\/?$/.test(parsed.pathname)) return t('admin.baseUrlTrailingV1')
  return ''
}

async function fetchModels() {
  if (keyDraft.value.trim()) addKey()
  if (keyInputError.value) { formError.value = keyInputError.value; return }
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
  if (keyDraft.value.trim()) addKey()
  if (keyInputError.value) { formError.value = keyInputError.value; return }
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

  const overrideFields = parseOverrideFields(form.delete_fields)
  const overrideSet = parseOverrideSet(form.set_fields)
  if (overrideSet === null) {
    formError.value = t('admin.requestOverridesInvalid')
    return
  }
  const overrideSetKeys = Object.keys(overrideSet)
  if (overrideFields.length > 50 || overrideSetKeys.length > 50
    || overrideFields.some(field => field.length > 100)
    || overrideSetKeys.some(key => !key.trim() || key.length > 100)) {
    formError.value = t('admin.requestOverridesInvalid')
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
    auto_disable: form.auto_disable,
    request_overrides: { delete: overrideFields, set: overrideSet },
  }
  if (form.provider === 'custom') {
    payload.upstream_path = form.upstream_path.trim()
    payload.upstream_format = form.upstream_format
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
    if (result.channel_disabled) {
      toast.error(t('admin.keyTestFailedAndChannelDisabled', { reason: result.reason ?? '' }))
    } else {
      toast.error(t('admin.keyTestFailedAndDisabled', { reason: result.reason ?? '' }))
    }
  } else {
    toast.error(t('admin.keyTestFailed', { status_code: result.status_code, reason: result.reason ?? '' }))
  }
  const refreshed = await endpoints.getChannelKeys(keysChannelId.value)
  channelKeys.data.value.data = refreshed.data
  await channels.refresh()
}

const testingChannelId = ref<string | null>(null)
async function testChannelRow(channel: Channel) {
  let result: ChannelTestResult | undefined
  testingChannelId.value = channel.id
  const ok = await run(async () => {
    result = await endpoints.testChannel(channel.id)
  })
  testingChannelId.value = null
  if (!ok || !result) { toast.error(t('common.actionFailed')); return }
  const failedKeys = result.keys.filter(k => !k.success)
  if (result.success) {
    if (failedKeys.length > 0) {
      toast.warn(t('admin.channelTestPartial', { passed: result.keys.length - failedKeys.length, disabled: failedKeys.length }))
    } else {
      toast.success(t('admin.channelTestSuccess', { status_code: result.status_code, latency_ms: result.latency_ms }))
    }
  } else if (result.channel_disabled) {
    toast.error(t('admin.channelTestFailedAndDisabled', { reason: result.reason ?? '', disabled: failedKeys.length }))
  } else {
    toast.error(t('admin.channelTestFailed', { status_code: result.status_code, reason: result.reason ?? '' }))
  }
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

async function toggleRouteEnabled(route: ModelRoute) {
  const ok = await run(() => endpoints.updateChannelRoute(routesChannelId.value, route.id, { enabled: !route.enabled }))
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

const usageDialogOpen = ref(false)
const usageChannelId = ref('')
const usageChannelName = ref('')
const usageTab = ref('stats')
const usageTabs = computed(() => [
  { value: 'stats', label: t('admin.channelUsage') },
  { value: 'quota', label: t('admin.channelQuota') },
])
const EMPTY_USAGE_STATS: ChannelUsageStats = {
  total_requests: 0, success_count: 0, error_count: 0,
  prompt_tokens: 0, completion_tokens: 0, total_tokens: 0,
  total_cost: '0', avg_duration_ms: 0,
}
const channelUsageStats = useResource(
  () => usageChannelId.value ? endpoints.getChannelUsageStats(usageChannelId.value) : Promise.resolve(EMPTY_USAGE_STATS),
  { data: EMPTY_USAGE_STATS },
)
const channelQuota = useResource(
  () => usageChannelId.value ? endpoints.getChannelQuota(usageChannelId.value) : Promise.resolve({ limits: [], usage: [] } as ChannelQuota),
  { data: { limits: [], usage: [] } as ChannelQuota },
)

const usageStatTiles = computed(() => [
  { key: 'statRequests', value: formatNumber(channelUsageStats.data.value.total_requests) },
  { key: 'statSuccess', value: formatNumber(channelUsageStats.data.value.success_count) },
  { key: 'statErrors', value: formatNumber(channelUsageStats.data.value.error_count) },
  { key: 'statPromptTokens', value: formatCompact(channelUsageStats.data.value.prompt_tokens) },
  { key: 'statCompletionTokens', value: formatCompact(channelUsageStats.data.value.completion_tokens) },
  { key: 'statTotalTokens', value: formatCompact(channelUsageStats.data.value.total_tokens) },
  { key: 'statCost', value: formatMoney(channelUsageStats.data.value.total_cost, 4) },
  { key: 'statAvgDuration', value: t('admin.durationMs', { value: Math.round(channelUsageStats.data.value.avg_duration_ms) }) },
])

const QUOTA_WINDOWS = [
  { value: 'minute', label: t('admin.quotaWindowMinute') },
  { value: 'day', label: t('admin.quotaWindowDay') },
  { value: 'month', label: t('admin.quotaWindowMonth') },
]

const quotaDialogOpen = ref(false)
const editingQuotaWindow = ref('')
const quotaFormError = ref('')
const quotaForm = reactive({
  window: 'day' as 'minute' | 'day' | 'month',
  max_requests: '',
  max_tokens: '',
})

async function openUsage(channel: Channel) {
  usageChannelId.value = channel.id
  usageChannelName.value = channel.name
  usageTab.value = 'stats'
  usageDialogOpen.value = true
  await Promise.all([channelUsageStats.refresh(), channelQuota.refresh()])
}

function openCreateQuota() {
  editingQuotaWindow.value = ''
  quotaFormError.value = ''
  quotaForm.window = 'day'
  quotaForm.max_requests = ''
  quotaForm.max_tokens = ''
  quotaDialogOpen.value = true
}

function openEditQuota(limit: { window: string; max_requests: number | null; max_tokens: number | null }) {
  editingQuotaWindow.value = limit.window
  quotaFormError.value = ''
  quotaForm.window = limit.window as 'minute' | 'day' | 'month'
  quotaForm.max_requests = limit.max_requests != null ? String(limit.max_requests) : ''
  quotaForm.max_tokens = limit.max_tokens != null ? String(limit.max_tokens) : ''
  quotaDialogOpen.value = true
}

async function saveQuota() {
  quotaFormError.value = ''
  const maxRequests = quotaForm.max_requests.trim()
  const maxTokens = quotaForm.max_tokens.trim()
  if (!maxRequests && !maxTokens) {
    quotaFormError.value = t('admin.quotaLimitInvalid')
    return
  }
  const form: ChannelQuotaForm = { window: quotaForm.window }
  if (maxRequests) {
    const val = Number(maxRequests)
    if (!Number.isInteger(val) || val < 0 || val > 1e12) {
      quotaFormError.value = t('admin.quotaLimitInvalid')
      return
    }
    form.max_requests = val
  }
  if (maxTokens) {
    const val = Number(maxTokens)
    if (!Number.isInteger(val) || val < 0 || val > 1e12) {
      quotaFormError.value = t('admin.quotaLimitInvalid')
      return
    }
    form.max_tokens = val
  }
  const ok = await run(() => endpoints.upsertChannelQuota(usageChannelId.value, form))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.quotaSaved'))
  quotaDialogOpen.value = false
  await channelQuota.refresh()
}

async function deleteQuota(window: string) {
  if (!confirm(t('admin.quotaConfirmDelete'))) return
  const ok = await run(() => endpoints.deleteChannelQuota(usageChannelId.value, window))
  if (!ok) { toast.error(t('common.actionFailed')); return }
  toast.success(t('admin.quotaDeleted'))
  await channelQuota.refresh()
}

function quotaUsageForWindow(window: string) {
  return channelQuota.data.value.usage.find(u => u.window === window)
}
</script>

<template>
  <ConsoleOpsDenied v-if="!allowed" />

  <div v-else class="space-y-4">
    <ConsoleOpsPageHeader :lead="t('admin.channelsLead')">
      <template #actions>
        <div class="hidden items-center gap-2 rounded-control border border-line px-3 py-1.5 sm:flex">
          <ListChecks class="size-4 text-muted" />
          <label for="channel-batch-mode" class="cursor-pointer text-sm text-ink">{{ t('admin.batchOps') }}</label>
          <UiSwitch id="channel-batch-mode" v-model="batchMode" />
        </div>

        <div class="hidden items-center gap-2 rounded-control border border-line px-3 py-1.5 sm:flex">
          <SortAsc class="size-4 text-muted" />
          <label for="channel-id-sort" class="cursor-pointer text-sm text-ink">{{ t('admin.idSort') }}</label>
          <UiSwitch id="channel-id-sort" v-model="idSort" />
        </div>

        <UiButton variant="secondary" size="sm" @click="channels.refresh()">{{ t('common.refresh') }}</UiButton>
        <UiButton v-if="canManage" size="sm" @click="openCreate">{{ t('admin.createChannel') }}</UiButton>
      </template>
    </ConsoleOpsPageHeader>

    <div class="flex flex-wrap items-center gap-2">
      <ConsoleOpsSearch v-model="search" :placeholder="t('admin.channelsSearchPlaceholder')" />
      <UiInput v-model="modelFilter" :placeholder="t('admin.channelsModelSearchPlaceholder')" class="w-full sm:w-40" />
      <div class="w-32"><UiSelect v-model="statusFilter" size="sm" :options="STATUS_OPTIONS" /></div>
      <div class="w-32"><UiSelect v-model="typeFilter" size="sm" :options="typeOptions" /></div>
      <div class="w-36"><UiSelect v-model="groupFilter" size="sm" :options="groupFilterOptions" /></div>
      <UiTooltip :content="sensitiveVisible ? t('admin.hideSensitive') : t('admin.showSensitive')">
        <UiButton
          variant="secondary"
          size="sm"
          :aria-label="sensitiveVisible ? t('admin.hideSensitive') : t('admin.showSensitive')"
          @click="sensitiveVisible = !sensitiveVisible"
        >
          <EyeOff v-if="sensitiveVisible" class="size-4" />
          <Eye v-else class="size-4" />
        </UiButton>
      </UiTooltip>
    </div>

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

      <template v-else>
        <div v-if="batchMode && selected.size > 0" class="flex flex-wrap items-center gap-2">
          <UiBadge tone="outline" class="text-xs">{{ t('admin.selectedCount', { count: selected.size }) }}</UiBadge>
          <UiButton variant="secondary" size="sm" @click="openBatchToggle(true)">{{ t('admin.batchEnable') }}</UiButton>
          <UiButton variant="secondary" size="sm" @click="openBatchToggle(false)">{{ t('admin.batchDisable') }}</UiButton>
        </div>

        <UiTable>
          <thead>
            <tr>
              <th v-if="batchMode" class="w-10">
                <UiCheckbox :model-value="allSelected" @update:model-value="toggleAll" />
              </th>
              <th class="num">{{ t('admin.channelId') }}</th>
              <th>{{ t('admin.channelName') }}</th>
              <th>{{ t('admin.provider') }}</th>
              <th>{{ t('common.status') }}</th>
              <th>{{ t('admin.groups') }}</th>
              <th class="num">{{ t('admin.priority') }}</th>
              <th class="num">{{ t('admin.weight') }}</th>
              <th>{{ t('admin.usedTokens') }}</th>
              <th>{{ t('admin.responseTime') }}</th>
              <th>{{ t('admin.lastTested') }}</th>
              <th v-if="canManage">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="channel in filtered" :key="channel.id">
              <td v-if="batchMode">
                <UiCheckbox :model-value="selected.has(channel.id)" @update:model-value="toggleSelected(channel.id)" />
              </td>
              <td class="num font-mono text-[13px] text-faint">{{ sensitiveVisible ? channel.id : '••••' }}</td>
              <td class="font-medium text-ink">
                <div class="flex items-center gap-2">
                  {{ sensitiveVisible ? channel.name : '••••' }}
                  <UiTooltip v-if="channel.auto_disabled" :content="channel.disabled_reason || t('admin.autoDisabled')">
                    <UiBadge tone="warn" dot>{{ t('admin.autoDisabled') }}</UiBadge>
                  </UiTooltip>
                </div>
              </td>
              <td>
                <span class="inline-flex items-center gap-1.5">
                  <component :is="PROVIDER_ICONS[channel.provider]" class="size-4 text-muted" />
                  <UiBadge tone="outline">{{ t(`admin.provider_${channel.provider}`) }}</UiBadge>
                </span>
              </td>
              <td>
                <UiSwitch
                  :model-value="channel.enabled"
                  :disabled="!canManage || busy"
                  :label="channel.enabled ? t('common.disable') : t('common.enable')"
                  @update:model-value="toggle(channel, $event)"
                />
              </td>
              <td>
                <div v-if="channel.groups.length" class="flex flex-wrap gap-1">
                  <UiBadge v-for="id in channel.groups" :key="id" tone="outline">{{ sensitiveVisible ? (groupNames.get(id) ?? id) : '••••' }}</UiBadge>
                </div>
                <span v-else class="text-faint">{{ t('common.all') }}</span>
              </td>
              <td class="num">{{ channel.priority }}</td>
              <td class="num">{{ channel.weight }}</td>
              <td class="num">
                <UiTooltip :content="t('admin.usedTokensDetail', { requests: formatNumber(channel.used_requests), tokens: formatCompact(channel.used_tokens) })">
                  <span class="text-sm text-ink">{{ sensitiveVisible ? formatCompact(channel.used_tokens) : '••••' }}</span>
                </UiTooltip>
              </td>
              <td>
                <UiBadge :tone="responseTone(channel.response_time_ms)">
                  {{ formatDuration(channel.response_time_ms) }}
                </UiBadge>
              </td>
              <td class="text-muted">
                <UiTooltip :content="formatDateTime(channel.last_test_time)">
                  <span class="text-[13px]">{{ sensitiveVisible ? formatRelativeTime(channel.last_test_time) : '••••' }}</span>
                </UiTooltip>
              </td>
              <td v-if="canManage">
                <div class="flex items-center gap-1">
                  <UiButton variant="ghost" size="sm" @click="openEdit(channel)">{{ t('common.edit') }}</UiButton>
                  <UiButton variant="ghost" size="sm" :loading="testingChannelId === channel.id" :disabled="busy" @click="testChannelRow(channel)">
                    <Play class="h-4 w-4" />
                  </UiButton>
                  <UiButton variant="ghost" size="sm" @click="openKeys(channel)">
                    <KeyRound class="h-4 w-4" />
                  </UiButton>
                  <UiButton variant="ghost" size="sm" @click="openRoutes(channel)">
                    <ArrowRightLeft class="h-4 w-4" />
                  </UiButton>
                  <UiButton variant="ghost" size="sm" @click="openUsage(channel)">
                    <BarChart3 class="h-4 w-4" />
                  </UiButton>
                </div>
              </td>
            </tr>
          </tbody>
        </UiTable>

        <ConsoleOpsPagination
          v-model:page="page"
          v-model:pageSize="pageSize"
          :total="channels.data.value.total"
        />
      </template>
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

        <div v-if="form.provider === 'custom'" class="grid gap-4 sm:grid-cols-2">
          <UiField :label="t('admin.upstreamFormat')" :hint="t('admin.upstreamFormatHint')">
            <UiSelect v-model="form.upstream_format" :options="formatOptions" />
          </UiField>
          <UiField :label="t('admin.upstreamPath')" :hint="t('admin.upstreamPathHint')">
            <UiInput v-model="form.upstream_path" mono :placeholder="t('admin.upstreamPathPlaceholder')" />
          </UiField>
        </div>

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
          :hint="editingId ? t('admin.apiKeyHintChannelEdit') : t('admin.apiKeyHintChannelCreate')"
          :required="!editingId"
        >
          <div class="rounded-control border border-line-strong bg-surface p-2">
            <div v-if="keyChips.length" class="flex flex-wrap gap-1.5">
              <span
                v-for="(key, index) in keyChips" :key="`${key}-${index}`"
                class="inline-flex items-center gap-1 rounded-full border border-line bg-sunken py-0.5 pl-2.5 pr-1 font-mono text-[13px] text-ink"
              >
                <span class="max-w-48 truncate">{{ key }}</span>
                <button
                  type="button"
                  class="flex size-4 items-center justify-center rounded-full text-faint transition-colors hover:bg-danger-soft hover:text-danger"
                  :aria-label="t('admin.removeKey')"
                  @click="removeKey(index)"
                >
                  <X class="size-3" />
                </button>
              </span>
            </div>
            <p v-else class="px-1 pb-1 text-2xs text-faint">{{ t('admin.keyListEmpty') }}</p>
            <UiInput
              v-model="keyDraft"
              mono
              :placeholder="t('admin.apiKeyInputPlaceholder')"
              @keydown.enter.prevent="addKey"
            />
          </div>
          <p v-if="keyInputError" class="mt-1 text-xs text-danger">{{ keyInputError }}</p>
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

        <UiField :label="t('admin.autoDisable')" :hint="t('admin.autoDisableHint')">
          <UiSwitch v-model="form.auto_disable" />
        </UiField>

        <UiField :label="t('admin.requestOverrideDelete')" :hint="t('admin.requestOverrideDeleteHint')">
          <UiTextarea v-model="form.delete_fields" mono :rows="2" :placeholder="t('admin.requestOverrideDeletePlaceholder')" />
        </UiField>

        <UiField :label="t('admin.requestOverrideSet')" :hint="t('admin.requestOverrideSetHint')">
          <UiTextarea v-model="form.set_fields" mono :rows="3" :placeholder="t('admin.requestOverrideSetPlaceholder')" />
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
        <p class="mt-1">{{ t('admin.keysEmptyHint') }}</p>
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="key in channelKeys.data.value.data" :key="key.id"
          class="flex items-center justify-between rounded-control border border-line px-3 py-2"
        >
          <div class="flex items-center gap-2 min-w-0">
            <KeyRound class="h-4 w-4 text-faint shrink-0" />
            <span class="text-sm font-medium text-ink truncate">{{ key.name }}</span>
            <UiBadge tone="outline" class="shrink-0">{{ t('admin.keyPriorityShort', { value: key.priority }) }}</UiBadge>
            <UiBadge v-if="!key.enabled" tone="danger" class="shrink-0">{{ t('common.disabled') }}</UiBadge>
            <UiTooltip v-else-if="key.last_error" :content="key.last_error">
              <UiBadge tone="warn" class="shrink-0">{{ t('admin.lastTestFailed') }}</UiBadge>
            </UiTooltip>
          </div>
          <div class="flex items-center gap-1 shrink-0">
            <UiSwitch
              :model-value="key.enabled"
              size="sm"
              :disabled="busy"
              @update:model-value="toggleKey(key)"
            />
            <UiButton v-if="canManage" variant="ghost" size="sm" :disabled="busy" :title="t('console.revealKey')" @click="openRevealKey(key)">
              <Eye class="h-4 w-4" />
            </UiButton>
            <UiButton variant="ghost" size="sm" :disabled="busy" @click="openEditKey(key)">
              {{ t('common.edit') }}
            </UiButton>
            <UiButton variant="secondary" size="sm" :loading="busy" :disabled="busy" @click="testKey(key)">
              <Play class="h-4 w-4" />
              <span class="ml-1">{{ t('admin.testKey') }}</span>
            </UiButton>
            <UiButton variant="ghost" size="sm" :disabled="busy" @click="deleteKey(key)">
              {{ t('common.delete') }}
            </UiButton>
          </div>
        </div>
      </div>

      <div class="mt-4">
        <UiButton size="sm" @click="openCreateKey">{{ t('admin.addKey') }}</UiButton>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="keysDialogOpen = false">{{ t('common.close') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel
      v-model:open="keyDialogOpen"
      size="sm"
      :title="editingKeyId ? t('admin.editKey') : t('admin.addKey')"
    >
      <div class="space-y-4">
        <UiAlert v-if="keyFormError" tone="danger">{{ keyFormError }}</UiAlert>

        <UiField v-if="!editingKeyId" :label="t('admin.apiKey')" required>
          <UiInput v-model="keyForm.api_key" mono :placeholder="t('admin.apiKeysPlaceholder')" />
        </UiField>

        <UiField :label="t('admin.keyName')" :required="!!editingKeyId">
          <UiInput v-model="keyForm.name" :placeholder="t('admin.keyNamePlaceholder')" />
        </UiField>

        <UiField :label="t('admin.keyPriority')" :hint="t('admin.keyPriorityHint')">
          <UiInput v-model="keyForm.priority" type="number" mono />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="keyDialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="saveKey">{{ t('common.save') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiDialog v-model:open="keyRevealOpen" size="sm" :title="t('console.revealKeyTitle')">
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
        <UiButton variant="secondary" @click="keyRevealOpen = false">{{ t('common.close') }}</UiButton>
        <UiButton v-if="revealError" :loading="busy" @click="loadRevealedSecret">
          {{ t('common.retry') }}
        </UiButton>
      </template>
    </UiDialog>

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
              :model-value="route.enabled"
              size="sm"
              :label="route.enabled ? t('common.enabled') : t('common.disabled')"
              :disabled="busy"
              @update:model-value="toggleRouteEnabled(route)"
            />
            <UiSwitch
              :model-value="route.hidden"
              size="sm"
              :label="t('admin.hidden')"
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

    <UiSlidePanel
      v-model:open="usageDialogOpen"
      size="lg"
      :title="usageChannelName"
    >
      <UiTabs v-model="usageTab" :items="usageTabs" class="mb-4" />

      <div v-if="usageTab === 'stats'">
        <p class="text-sm text-muted mb-4">{{ t('admin.channelUsageLead') }}</p>

        <UiSkeleton v-if="channelUsageStats.pending.value" :rows="4" />
        <div v-else class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div v-for="tile in usageStatTiles" :key="tile.key" class="rounded-card border border-line bg-surface px-4 py-3">
            <p class="text-2xs text-muted">{{ t(`admin.${tile.key}`) }}</p>
            <p class="numeric mt-1 text-lg text-ink">{{ tile.value }}</p>
          </div>
        </div>
      </div>

      <div v-else-if="usageTab === 'quota'">
        <p class="text-sm text-muted mb-4">{{ t('admin.channelQuotaLead') }}</p>

        <UiSkeleton v-if="channelQuota.pending.value" :rows="3" />

        <div v-else-if="!channelQuota.data.value.limits.length" class="py-4 text-center text-muted text-sm">
          <p>{{ t('admin.quotaNoLimits') }}</p>
          <p class="mt-1">{{ t('admin.quotaNoLimitsHint') }}</p>
        </div>

        <div v-else class="space-y-2">
          <div
            v-for="limit in channelQuota.data.value.limits" :key="limit.id"
            class="rounded-control border border-line px-3 py-2"
          >
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <UiBadge tone="outline">{{ t(`admin.quotaWindow${limit.window.charAt(0).toUpperCase() + limit.window.slice(1)}`) }}</UiBadge>
              </div>
              <div v-if="canManage" class="flex items-center gap-1">
                <UiButton variant="ghost" size="sm" @click="openEditQuota(limit)">{{ t('common.edit') }}</UiButton>
                <UiButton variant="danger" size="sm" @click="deleteQuota(limit.window)">{{ t('common.delete') }}</UiButton>
              </div>
            </div>
            <div class="mt-2 grid gap-2 sm:grid-cols-2 text-sm">
              <div>
                <span class="text-muted">{{ t('admin.quotaMaxRequests') }}:</span>
                <span class="numeric text-ink ml-1">{{ limit.max_requests != null ? formatNumber(limit.max_requests) : '—' }}</span>
                <template v-if="quotaUsageForWindow(limit.window)">
                  <span class="text-faint ml-1">/ {{ formatNumber(quotaUsageForWindow(limit.window)!.requests) }}</span>
                </template>
              </div>
              <div>
                <span class="text-muted">{{ t('admin.quotaMaxTokens') }}:</span>
                <span class="numeric text-ink ml-1">{{ limit.max_tokens != null ? formatNumber(limit.max_tokens) : '—' }}</span>
                <template v-if="quotaUsageForWindow(limit.window)">
                  <span class="text-faint ml-1">/ {{ formatCompact(quotaUsageForWindow(limit.window)!.tokens) }}</span>
                </template>
              </div>
            </div>
          </div>
        </div>

        <div v-if="canManage" class="mt-4">
          <UiButton size="sm" @click="openCreateQuota">{{ t('admin.quotaAddLimit') }}</UiButton>
        </div>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="usageDialogOpen = false">{{ t('common.close') }}</UiButton>
      </template>
    </UiSlidePanel>

    <UiSlidePanel
      v-model:open="quotaDialogOpen"
      size="sm"
      :title="editingQuotaWindow ? t('admin.quotaEditLimit') : t('admin.quotaAddLimit')"
    >
      <div class="space-y-4">
        <UiAlert v-if="quotaFormError" tone="danger">{{ quotaFormError }}</UiAlert>

        <UiField :label="t('admin.quotaWindow')" required>
          <UiSelect
            v-model="quotaForm.window"
            :options="QUOTA_WINDOWS"
            :disabled="!!editingQuotaWindow"
          />
        </UiField>

        <UiField :label="t('admin.quotaMaxRequests')" :hint="t('admin.quotaMaxRequestsHint')">
          <UiInput v-model="quotaForm.max_requests" type="number" mono />
        </UiField>

        <UiField :label="t('admin.quotaMaxTokens')" :hint="t('admin.quotaMaxTokensHint')">
          <UiInput v-model="quotaForm.max_tokens" type="number" mono />
        </UiField>
      </div>

      <template #footer>
        <UiButton variant="secondary" @click="quotaDialogOpen = false">{{ t('common.cancel') }}</UiButton>
        <UiButton :loading="busy" @click="saveQuota">{{ t('common.save') }}</UiButton>
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
