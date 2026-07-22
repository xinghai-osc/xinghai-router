<script setup lang="ts">
import { computed, defineAsyncComponent, h, onMounted, onUnmounted, provide, reactive, ref, watch } from 'vue'
import { Activity, Database, KeyRound, Layers3, LayoutDashboard, RadioTower, ShieldCheck, TerminalSquare, UserRound, Users, WalletCards, ReceiptText, Tags, Settings, Crown } from 'lucide-vue-next'
import { endpoints, clearToken, getToken } from '~/src/api'
import type { MigrateForm, MigrationStatus, Account, ActivityLog, AdminSiteSettings, AdminSubscription, ApiKey, CatalogGroup, CatalogModel, Channel, Group, LedgerEntry, ModelProvider, PaymentMethod, PaymentOrder, PaymentSettings, Pricing, PublicSubscriptionPlan, ReliabilitySettings, SiteSettings, SubscriptionOrder, SubscriptionPlan, UsageRecord, User, UserSubscription } from '~/src/api'
import ModelSquare from '~/components/marketplace/ModelSquare.vue'
import type { View } from '~/src/views'
import { VIEWS } from '~/src/views'
import { CONSOLE_STORE_KEY } from '~/composables/useConsoleStore'
const { locale, t, setLocale, toggleLocale, initializeLocale } = useI18n()
const props = withDefaults(defineProps<{ activeView?: View }>(), { activeView: 'overview' })
const route = useRoute()
const router = useRouter()
const views: View[] = VIEWS
function resolveView(q: string | undefined, p: string | undefined): View {
  const selected = views.includes(q as View) ? q as View : props.activeView && views.includes(props.activeView) ? props.activeView : views.includes(p as View) ? p as View : 'overview'
  return selected === 'logs' || selected === 'audit' ? 'usage' : selected
}
const currentView = ref<View>(resolveView(route.query.view as string | undefined, route.params.view as string | undefined))
const view = computed(() => currentView.value)
router.afterEach((to) => {
  const q = to.query.view as string | undefined
  const p = to.params.view as string | undefined
  currentView.value = resolveView(q, p)
})
const authenticated = ref(Boolean(getToken()))
const error = ref('')
const busy = ref(false)
const sidebarCollapsed = ref(false)
const setupCollapsed = ref(false)
const users = ref<User[]>([])
const groups = ref<Group[]>([])
const ownGroups = ref<string[]>([])
const keys = ref<ApiKey[]>([])
const accountKeys = ref<ApiKey[]>([])
const channels = ref<Channel[]>([])
const providers = ref<ModelProvider[]>([])
const activityLogs = ref<ActivityLog[]>([])
const account = ref<Account | null>(null)
const usageRecords = ref<UsageRecord[]>([])
const ledger = ref<LedgerEntry[]>([])
const payments = ref<PaymentOrder[]>([])
const paymentMethods = ref<PaymentMethod[]>([])
const paymentsEnabled = ref(false)
const paymentMessage = ref('')
const paymentForm = reactive({ amount: 10, type: '' })
const paymentSettings = reactive<PaymentSettings>({ enabled: false, base_url: '', merchant_id: '', has_merchant_key: false, public_base_url: '', methods: [] })
const paymentSettingsForm = reactive({ enabled: false, base_url: '', merchant_id: '', merchant_key: '', public_base_url: '' })
const paymentMethodForm = reactive({ code: '', name: '', enabled: true })
const pricing = ref<Pricing[]>([])
const siteSettings = ref<SiteSettings>({ name: 'Xinghai Router', icon_url: '', auto_disable_failed_channels: false })
const catalog = ref<CatalogModel[]>([])
const catalogGroups = ref<CatalogGroup[]>([])
const catalogLoaded = ref(false)
const catalogGroup = ref('all')
const catalogSearch = ref('')
const activityModels = ref<string[]>([])
const activityFilters = reactive({ user_id: '', model: '', group_id: '', start: '', end: '', type: '' })
const createdKey = ref('')
const showKey = ref(false)
const showAccountKey = ref(false)
const editingAccountKey = ref<ApiKey | null>(null)
const showChannel = ref(false)
const editingChannel = ref<Channel | null>(null)
const showProvider = ref(false)
const selectedUser = ref<User | null>(null)
const originalUser = ref<User | null>(null)
const selectedPermissions = ref<string[]>([])
const selectedGroups = ref<string[]>([])
const userPassword = ref('')
const userBalance = ref<number | null>(null)
const userBalanceNote = ref('')
const keyForm = reactive({ user_id: '', name: '', expires_at: '', group_id: '' })
const accountKeyForm = reactive({ name: '', expires_at: '', group_id: '' })
const channelForm = reactive({ name: '', provider: 'openai', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100, groups: [] as string[] })
const providerForm = reactive({ name: '', slug: '', prefixes: '', priority: 100 })
const editingProviderID = ref('')
const groupForm = reactive({ name: '', multiplier: 1 })
const groupImportText = ref('')

const generalNav = computed(() => [['overview', t('overview'), LayoutDashboard], ['account', t('account'), KeyRound], ['usage-overview', t('usageOverview'), Activity], ['usage', t('usage'), TerminalSquare]] as const)
const billingNav = computed(() => [['wallet', t('wallet'), WalletCards], ['ledger', t('ledger'), ReceiptText], ['subscriptions', t('subscriptions'), Crown]] as const)
const personalNav = computed(() => [['profile', t('profile'), UserRound]] as const)
const managementNavItems = [
  ['users', 'users', Users, 'users.read'], ['groups', 'groups', Layers3, 'system.manage'], ['keys', 'keys', KeyRound, 'keys.manage'], ['channels', 'channels', RadioTower, 'channels.read'], ['providers', 'providers', Tags, 'system.manage'],
] as const
const adminExtraNav = [['pricing', 'pricing', Tags, 'pricing.read'], ['reliability', 'reliability', ShieldCheck, 'system.manage'], ['subscription-plans', 'subscriptionPlans', Crown, 'system.manage'], ['admin-subscriptions', 'adminSubscriptions', Crown, 'users.read'], ['migrate', 'migrate', Database, 'system.manage'], ['site-settings', 'siteSettings', Settings, 'system.manage'], ['payment-settings', 'paymentSettings', WalletCards, 'system.manage']] as const
const localizedManagementNavItems = computed(() => managementNavItems.map(([id, key, icon, permission]) => [id, t(key as Parameters<typeof t>[0]), icon, permission] as const))
const localizedAdminExtraNav = computed(() => adminExtraNav.map(([id, key, icon, permission]) => [id, t(key as Parameters<typeof t>[0]), icon, permission] as const))
const permissions = ['users.read', 'users.manage', 'keys.manage', 'channels.read', 'channels.manage', 'logs.read', 'pricing.read', 'pricing.manage', 'audit.read', 'wallets.manage', 'routes.manage', 'quotas.manage', 'system.manage']
const pricingForm = reactive({ model: '', input_per_million: 0, cached_input_per_million: 0, output_per_million: 0, multiplier: 1 })
const subscriptionPlans = ref<SubscriptionPlan[]>([])
const publicPlans = ref<PublicSubscriptionPlan[]>([])
const userSubscriptions = ref<UserSubscription[]>([])
const subscriptionOrders = ref<SubscriptionOrder[]>([])
const adminSubscriptions = ref<AdminSubscription[]>([])
const subscriptionPlanForm = reactive({ name: '', description: '', price: '0', currency: 'CNY', billing_period: 'month', credit_amount: '0', group_id: '', model_whitelist: '', max_requests_per_period: '' as string | number, max_tokens_per_period: '' as string | number, sort_order: 0, enabled: true })
const editingPlanID = ref('')
const showPlanModal = ref(false)
const subscribingPlan = ref<PublicSubscriptionPlan | null>(null)
const subscribeForm = reactive({ payment_type: '', auto_renew: false })
const subscriptionMessage = ref('')
const extendForm = reactive({ plan_id: '', days: 30 })
const siteSettingsForm = reactive<AdminSiteSettings & { geetest_captcha_key: string; smtp_password: string }>({ name: '', icon_url: '', auto_disable_failed_channels: false, geetest_captcha_id: '', has_geetest_captcha_key: false, geetest_captcha_key: '', smtp_host: '', smtp_port: '465', smtp_username: '', has_smtp_password: false, smtp_password: '', smtp_from: '' })
const reliabilityForm = reactive<ReliabilitySettings>({ retry_count: 3, retry_status_codes: '', health_check_mode: 'off', health_check_interval_minutes: 5, health_check_auto_recover: true, health_check_channel_ids: '', auto_disable_on_test_failure: false, auto_disable_slow_seconds: 0, auto_disable_status_codes: '', auto_disable_keywords: '' })
const newAPIPricingForm = reactive({ base_url: '', api_key: '', price_per_quota_unit: 1 })
const migrateForm = reactive<MigrateForm>({ source_dsn: '', source_driver: 'mysql' })
const migrateStatus = ref<MigrationStatus | null>(null)
const migratePolling = ref(false)
let migratePollTimer: ReturnType<typeof setInterval> | null = null
const leaderboardPrefs = reactive({ opt_in: true, mask_name: true })
async function saveLeaderboardPrefs() { await action(async () => { await endpoints.updateAccountPreferences(leaderboardPrefs.opt_in, leaderboardPrefs.mask_name); if (account.value) { account.value.leaderboard_opt_in = leaderboardPrefs.opt_in; account.value.leaderboard_mask_name = leaderboardPrefs.mask_name } }) }
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarUrlInput = ref('')
const passwordForm = reactive({ current_password: '', new_password: '', confirm_password: '' })
const passwordMessage = ref('')
const isLanding = computed(() => route.path === '/')
const isAuthPage = computed(() => route.path === '/auth')
const isMarketplacePage = computed(() => route.path === '/models')
const isAdmin = computed(() => account.value?.role === 'admin')
const can = (permission: string) => isAdmin.value || Boolean(account.value?.permissions.includes(permission))
const managementNav = computed(() => [...localizedManagementNavItems.value, ...localizedAdminExtraNav.value].filter((item) => can(item[3])))
const personalRequests = computed(() => usageRecords.value.length)
const personalTokens = computed(() => usageRecords.value.reduce((sum, item) => sum + item.prompt_tokens + item.completion_tokens, 0))
const personalCost = computed(() => usageRecords.value.reduce((sum, item) => sum + Number(item.cost), 0))
const setupProgress = computed(() => [accountKeys.value.some((item) => !item.revoked_at), Number(account.value?.balance ?? 0) > 0, personalRequests.value > 0].filter(Boolean).length)
const filteredCatalog = computed(() => {
  const search = catalogSearch.value.trim().toLowerCase()
  return catalog.value.filter((item) => (!search || item.model.toLowerCase().includes(search)) && (catalogGroup.value === 'all' || item.groups.some((group) => group.id === catalogGroup.value)))
})
const apiEndpoint = computed(() => import.meta.client ? `${window.location.origin}/v1/chat/completions` : '/v1/chat/completions')
const usageChart = computed(() => {
  const days = Array.from({ length: 7 }, (_, index) => {
    const date = new Date()
    date.setHours(0, 0, 0, 0)
    date.setDate(date.getDate() - 6 + index)
    return { key: date.toISOString().slice(0, 10), label: new Intl.DateTimeFormat(locale.value === 'en-US' ? 'en-US' : 'zh-CN', { weekday: 'short' }).format(date), tokens: 0, cost: 0 }
  })
  const byDay = new Map(days.map((day) => [day.key, day]))
  for (const item of usageRecords.value) {
    const day = byDay.get(item.created_at.slice(0, 10))
    if (day) { day.tokens += item.prompt_tokens + item.completion_tokens; day.cost += Number(item.cost) }
  }
  const maxTokens = Math.max(...days.map((day) => day.tokens), 1)
  const maxCost = Math.max(...days.map((day) => day.cost), 1)
  return days.map((day) => ({ ...day, tokenHeight: Math.max(day.tokens ? 8 : 2, Math.round(day.tokens / maxTokens * 100)), costHeight: Math.max(day.cost ? 8 : 2, Math.round(day.cost / maxCost * 100)) }))
})
const usageLinePoints = computed(() => usageChart.value.map((day, index) => `${index * 100 / 6},${100 - day.tokenHeight}`).join(' '))
const userName = (id: string | null) => users.value.find((user) => user.id === id)?.name ?? t('deletedUser')
const formatDate = (value: string | null) => value ? new Intl.DateTimeFormat(locale.value === 'en-US' ? 'en-US' : 'zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : t('never')
const short = (value: string | null) => value ? `${value.slice(0, 8)}...` : '---'
const formatPrice = (value: number | null, multiplier = 1) => value == null ? t('pendingConfig') : `¥${(Number(value) * multiplier).toFixed(Number(value) * multiplier < 0.01 ? 4 : 2)}`
const providerIcon = (slug: string) => `https://unpkg.com/@lobehub/icons-static-svg@1.93.0/icons/${slug}.svg`
const modelProvider = (model: string) => catalog.value.find((item) => item.model === model)?.provider ?? t('other')
const selectedCatalogGroup = (item: CatalogModel) => item.groups.find((group) => group.id === catalogGroup.value) ?? item.groups[0]
const actualMultiplier = (item: CatalogModel) => Number(item.multiplier ?? 1) * Number(selectedCatalogGroup(item)?.multiplier ?? 1)
const Empty = (props: { text: string }) => h('div', { class: 'empty' }, props.text)
Empty.props = { text: { type: String, required: true } }

const activityTypeLabel = computed(() => ({ request: 'Request', login: 'Login', register: 'Register', logout: 'Logout', topup: 'Top-up', operation: t('otherOperation') } as Record<ActivityLog['type'], string>))
const actionLabel = (item: ActivityLog) => ({ 'account.logged_in': t('actionAccountLogin'), 'account.registered': t('actionAccountRegister'), 'account.logged_out': t('actionAccountLogout'), 'wallet.adjusted': t('actionWalletAdjusted') }[item.action] ?? item.action)
const activityDetail = (item: ActivityLog) => item.type === 'request' ? `${item.prompt_tokens} / ${item.completion_tokens} tokens · ${Number(item.cost).toFixed(6)}` : JSON.stringify(item.details)

async function loadActivity(filters = false) {
  const query = new URLSearchParams()
  if (filters) Object.entries(activityFilters).forEach(([key, value]) => { if (value) query.set(key, key === 'start' || key === 'end' ? new Date(value).toISOString() : value) })
  const value = await endpoints.getActivityLogs(query.size ? `?${query}` : '')
  activityLogs.value = value.data
  if (!filters) activityModels.value = [...new Set(value.data.map((item) => item.model).filter(Boolean))].sort()
}
async function filterActivity() { await action(() => loadActivity(true)) }
async function resetActivityFilters() { Object.assign(activityFilters, { user_id: '', model: '', group_id: '', start: '', end: '', type: '' }); await action(() => loadActivity()) }

// Core account + site settings. Always loaded on entry; cheap and required by
// the sidebar, header and most views.
async function loadCore() {
  const [settings, me] = await Promise.all([endpoints.getSiteSettings(), endpoints.getAccount()])
  siteSettings.value = settings
  Object.assign(siteSettingsForm, settings)
  account.value = me
  leaderboardPrefs.opt_in = me.leaderboard_opt_in; leaderboardPrefs.mask_name = me.leaderboard_mask_name
  const ownGroupValue = await endpoints.getAccountGroups().catch(() => ({ data: [], groups: [] }))
  ownGroups.value = ownGroupValue.data
  if (!can('users.read')) groups.value = ownGroupValue.groups
}

// Personal data shared across account/wallet/overview/subscriptions views.
// `keys`/`usage`/`ledger`/`payments`/`subscriptions`/`subscription-orders`/`activity`.
async function loadPersonal() {
  const [ownKeys, ownUsage, ownLedger, ownPayments] = await Promise.all([
    endpoints.getAccountKeys().catch(() => ({ data: [] })),
    endpoints.getAccountUsage().catch(() => ({ data: [] })),
    endpoints.getAccountLedger().catch(() => ({ data: [] })),
    endpoints.getAccountPayments().catch(() => ({ enabled: false, payment_methods: [], data: [] })),
  ])
  accountKeys.value = ownKeys.data; usageRecords.value = ownUsage.data; ledger.value = ownLedger.data
  paymentsEnabled.value = ownPayments.enabled; payments.value = ownPayments.data; paymentMethods.value = ownPayments.payment_methods ?? []
  if (!paymentMethods.value.some((method) => method.code === paymentForm.type)) paymentForm.type = paymentMethods.value[0]?.code ?? ''
  const [ownSubs, ownSubOrders] = await Promise.all([
    endpoints.getAccountSubscriptions().catch(() => ({ data: [] as UserSubscription[] })),
    endpoints.getAccountSubscriptionOrders().catch(() => ({ data: [] as SubscriptionOrder[] })),
  ])
  userSubscriptions.value = ownSubs.data
  subscriptionOrders.value = ownSubOrders.data
  await loadActivity()
}

async function loadUsersAndGroups() {
  if (!can('users.read')) return
  const [userValue, groupValue] = await Promise.all([endpoints.getAdminUsers(), endpoints.getAdminGroups()])
  users.value = userValue.data; groups.value = groupValue.data
}
async function loadAdminKeys() { if (can('keys.manage')) keys.value = (await endpoints.getAdminKeys()).data }
async function loadAdminChannels() { if (can('channels.read')) channels.value = (await endpoints.getAdminChannels()).data }
async function loadProviders() { if (can('system.manage')) providers.value = (await endpoints.getAdminProviders()).data }
async function loadPaymentSettings() {
  if (!can('system.manage')) return
  const value = await endpoints.getAdminPaymentSettings()
  Object.assign(paymentSettings, value)
  Object.assign(paymentSettingsForm, { enabled: value.enabled, base_url: value.base_url, merchant_id: value.merchant_id, merchant_key: '', public_base_url: value.public_base_url })
}
async function loadPricing() { if (can('pricing.read')) pricing.value = (await endpoints.getAdminPricing()).data }
async function loadReliability() { if (can('system.manage')) Object.assign(reliabilityForm, await endpoints.getAdminReliabilitySettings()) }
async function loadSubscriptionPlans() { if (can('system.manage')) subscriptionPlans.value = (await endpoints.getAdminSubscriptionPlans()).data }
async function loadAdminSubscriptions() { if (can('users.read')) adminSubscriptions.value = (await endpoints.getAdminSubscriptions()).data }
async function loadAdminSiteSettings() {
  if (!can('system.manage')) return
  const value = await endpoints.getAdminSiteSettings()
  Object.assign(siteSettingsForm, value); siteSettingsForm.geetest_captcha_key = ''; siteSettingsForm.smtp_password = ''
}

// Map each console view to the loaders it requires. Entry into a view only
// triggers the loaders listed here, so a regular user opening /console/account
// no longer fires a dozen admin endpoints.
const VIEW_LOADERS: Partial<Record<View, (() => Promise<void>)[]>> = {
  overview: [loadPersonal],
  account: [loadPersonal],
  profile: [loadPersonal],
  wallet: [loadPersonal],
  ledger: [loadPersonal],
  subscriptions: [loadPersonal],
  usage: [loadPersonal, loadUsersAndGroups],
  users: [loadUsersAndGroups],
  groups: [loadUsersAndGroups],
  keys: [loadAdminKeys, loadUsersAndGroups],
  channels: [loadAdminChannels],
  providers: [loadProviders],
  pricing: [loadPricing],
  reliability: [loadReliability],
  'site-settings': [loadAdminSiteSettings],
  'payment-settings': [loadPaymentSettings],
  'subscription-plans': [loadSubscriptionPlans, loadUsersAndGroups],
  'admin-subscriptions': [loadAdminSubscriptions],
  migrate: [],
}

const loadedViews = ref<Set<View>>(new Set())
async function loadView(targetView: View, force = false) {
  if (!force && loadedViews.value.has(targetView)) return
  loadedViews.value.add(targetView)
  const loaders = VIEW_LOADERS[targetView] ?? []
  await Promise.all(loaders.map((loader) => loader().catch(() => undefined)))
}

// Full reload of the currently visible view plus core/personal data. Used by
// the Refresh button and after any mutation (createKey/savePricing/savePlan...).
async function load() {
  busy.value = true; error.value = ''
  try {
    await loadCore()
    await loadPersonal()
    loadedViews.value.clear()
    await loadView(view.value, true)
    } catch (cause) { error.value = cause instanceof Error ? cause.message : t('loadFailed') } finally { busy.value = false }
}

// Lazy-load data for the view the user just navigated to. Runs only after the
// console is authenticated, so the landing/auth pages don't trigger fetches.
watch(view, async (next) => {
  if (!authenticated.value) return
  busy.value = true; error.value = ''
  try { await loadView(next) } catch (cause) { error.value = cause instanceof Error ? cause.message : t('loadFailed') } finally { busy.value = false }
})
async function loadSiteSettings() { const value = await endpoints.getSiteSettings(); siteSettings.value = value; Object.assign(siteSettingsForm, value); document.title = value.name; const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]') ?? document.head.appendChild(Object.assign(document.createElement('link'), { rel: 'icon' })); if (value.icon_url) link.href = value.icon_url; else link.removeAttribute('href') }
async function saveSiteSettings() { await action(async () => { const value = await endpoints.updateAdminSiteSettings(siteSettingsForm); Object.assign(siteSettingsForm, value); siteSettingsForm.geetest_captcha_key = ''; siteSettingsForm.smtp_password = ''; await loadSiteSettings() }) }
async function saveReliabilitySettings() { await action(async () => { const value = await endpoints.updateReliabilitySettings(reliabilityForm); Object.assign(reliabilityForm, value) }) }
async function loadCatalog() { try { const value = await endpoints.getModelCatalog(); catalog.value = value.data; catalogGroups.value = value.groups } finally { catalogLoaded.value = true } }
async function signOut() { try { await endpoints.logout() } catch { /* Local session removal is sufficient when the server is unreachable. */ } clearToken(); authenticated.value = false; error.value = ''; await router.replace('/') }
async function createKey() { await action(async () => { const response = await endpoints.createKey({ ...keyForm, expires_at: keyForm.expires_at ? new Date(keyForm.expires_at).toISOString() : '' }); createdKey.value = response.key; showKey.value = false; Object.assign(keyForm, { user_id: '', name: '', expires_at: '', group_id: '' }); await load() }) }
async function createAccountKey() { await action(async () => { const response = await endpoints.createAccountKey({ ...accountKeyForm, expires_at: accountKeyForm.expires_at ? new Date(accountKeyForm.expires_at).toISOString() : '' }); createdKey.value = response.key; showAccountKey.value = false; Object.assign(accountKeyForm, { name: '', expires_at: '', group_id: '' }); await load() }) }
function editAccountKey(key: ApiKey) { editingAccountKey.value = key; Object.assign(accountKeyForm, { name: key.name, expires_at: key.expires_at ? new Date(key.expires_at).toISOString().slice(0, 16) : '', group_id: key.group_id }) }
async function updateAccountKey() { if (!editingAccountKey.value) return; await action(async () => { await endpoints.updateAccountKey(editingAccountKey.value.id, { ...accountKeyForm, expires_at: accountKeyForm.expires_at ? new Date(accountKeyForm.expires_at).toISOString() : '' }); editingAccountKey.value = null; Object.assign(accountKeyForm, { name: '', expires_at: '', group_id: '' }); await load() }) }
async function fetchChannelModels() { await action(async () => { const response = await endpoints.fetchChannelModels(channelForm.base_url, channelForm.api_key); channelForm.models = response.models.join(', ');     if (!response.models.length) throw new Error(t('upstreamNoModels')) }) }
async function createChannel() { await action(async () => { await endpoints.createChannel({ ...channelForm, models: channelForm.models.split(',').map((value) => value.trim()).filter(Boolean) }); showChannel.value = false; Object.assign(channelForm, { name: '', provider: 'openai', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100, groups: [] }); await load() }) }
function editChannel(channel: Channel) { editingChannel.value = channel; Object.assign(channelForm, { name: channel.name, provider: channel.provider, base_url: channel.base_url, api_key: '', models: channel.models.join(', '), priority: channel.priority, groups: [...channel.groups] }) }
async function updateChannel() { if (!editingChannel.value) return; await action(async () => { const models = channelForm.models.split(',').map((value) => value.trim()).filter(Boolean); await endpoints.updateChannel(editingChannel.value.id, { ...channelForm, models }); await endpoints.updateChannelGroups(editingChannel.value.id, channelForm.groups); editingChannel.value = null; Object.assign(channelForm, { name: '', provider: 'openai', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100, groups: [] }); await load() }) }
async function saveProvider() { await action(async () => { await endpoints.saveProvider({ ...providerForm, id: editingProviderID.value || undefined, prefixes: providerForm.prefixes.split(',').map((value) => value.trim()).filter(Boolean) }); showProvider.value = false; editingProviderID.value = ''; Object.assign(providerForm, { name: '', slug: '', prefixes: '', priority: 100 }); await load() }) }
function openProvider() { editingProviderID.value = ''; Object.assign(providerForm, { name: '', slug: '', prefixes: '', priority: 100 }); showProvider.value = true }
function editProvider(provider: ModelProvider) { editingProviderID.value = provider.id; Object.assign(providerForm, { name: provider.name, slug: provider.slug, prefixes: provider.prefixes.join(', '), priority: provider.priority }); showProvider.value = true }
async function removeProvider(provider: ModelProvider) { if (!confirm(t('deleteProviderConfirm').replace('{name}', provider.name))) return; await action(async () => { await endpoints.removeProvider(provider.id); await load() }) }
async function createGroup() { const name = groupForm.name.trim(); const multiplier = Number(groupForm.multiplier);     if (!name) { error.value = t('enterGroupName'); return } if (!Number.isFinite(multiplier) || multiplier <= 0) { error.value = t('multiplierMustBePositive'); return } await action(async () => { await endpoints.createGroup(name, multiplier); Object.assign(groupForm, { name: '', multiplier: 1 }); await load() }) }
async function editGroupMultiplier(group: Group, event: Event) { const multiplier = Number(new FormData(event.currentTarget as HTMLFormElement).get('multiplier'));     if (!Number.isFinite(multiplier) || multiplier < 0) { error.value = t('multiplierMustBeNonNegative'); return } await action(async () => { await endpoints.updateGroup(group.id, multiplier); await load() }) }
async function importGroups() { let values: Record<string, unknown>; try { values = JSON.parse(groupImportText.value) } catch { error.value = t('enterValidJSON'); return } if (!values || Array.isArray(values) || typeof values !== 'object') { error.value = t('importMustBeGroupMultiplierJSON'); return } const entries = Object.entries(values); if (!entries.length || entries.some(([name, multiplier]) => !name.trim() || typeof multiplier !== 'number' || !Number.isFinite(multiplier) || multiplier < 0)) { error.value = t('groupNameRequiredMultiplierNonNegative'); return } await action(async () => { await endpoints.importGroups(values as Record<string, number>); groupImportText.value = ''; await load() }) }
async function toggleChannel(channel: Channel) { await action(async () => { await endpoints.toggleChannel(channel.id, !channel.enabled); await load() }) }
async function revokeKey(key: ApiKey) { if (!confirm(t('revokeKeyConfirm').replace('{prefix}', key.key_prefix))) return; await action(async () => { await endpoints.revokeKey(key.id); await load() }) }
async function action(work: () => Promise<void>) { busy.value = true; error.value = ''; try { await work() } catch (cause) { error.value = cause instanceof Error ? cause.message : t('operationFailed') } finally { busy.value = false } }
async function createPayment() {
  const amount = Number(paymentForm.amount)
  if (!Number.isFinite(amount) || amount < 1 || amount > 100000) { error.value = t('paymentAmountRange'); return }
  await action(async () => {
    const result = await endpoints.createAccountPayment(amount.toFixed(2), paymentForm.type)
    window.location.assign(result.pay_url)
  })
}
async function savePaymentSettings() { await action(async () => { const value = await endpoints.updateAdminPaymentSettings(paymentSettingsForm); Object.assign(paymentSettings, value); paymentSettingsForm.merchant_key = ''; await load() }) }
async function createPaymentMethod() { await action(async () => { await endpoints.createPaymentMethod(paymentMethodForm); Object.assign(paymentMethodForm, { code: '', name: '', enabled: true }); await load() }) }
async function updatePaymentMethod(method: PaymentMethod) { await action(async () => { await endpoints.updatePaymentMethod(method.id, { code: method.code, name: method.name, enabled: method.enabled }); await load() }) }
async function deletePaymentMethod(method: PaymentMethod) { if (!confirm(t('deletePaymentMethodConfirm').replace('{name}', method.name))) return; await action(async () => { await endpoints.deletePaymentMethod(method.id); await load() }) }
async function copyKey() { await navigator.clipboard.writeText(createdKey.value) }
async function savePricing() { await action(async () => { await endpoints.savePricing(pricingForm); Object.assign(pricingForm, { model: '', input_per_million: 0, cached_input_per_million: 0, output_per_million: 0, multiplier: 1 }); await load() }) }
async function syncNewAPIPricing() { await action(async () => { const result = await endpoints.syncNewApiPricing(newAPIPricingForm); await load(); error.value = t('syncPricingResult').replace('{count}', String(result.synced)) }) }

async function loadPublicPlans() { const value = await endpoints.getPublicSubscriptionPlans(); publicPlans.value = value.data }
async function openSubscribeModal(plan: PublicSubscriptionPlan) { if (!paymentMethods.value.length) { error.value = t('paymentNotConfigured'); return } subscribingPlan.value = plan; subscribeForm.payment_type = paymentMethods.value[0]?.code ?? ''; subscribeForm.auto_renew = false; if (!publicPlans.value.length) await loadPublicPlans() }
async function confirmSubscribe() {
  if (!subscribingPlan.value || !subscribeForm.payment_type) return
  await action(async () => {
    const result = await endpoints.createAccountSubscription(subscribingPlan.value!.id, subscribeForm.payment_type, subscribeForm.auto_renew)
    subscribingPlan.value = null
    window.location.assign(result.pay_url)
  })
}
async function cancelSubscription(sub: UserSubscription) { if (!confirm(t('cancelSubscriptionConfirm'))) return; await action(async () => { await endpoints.cancelAccountSubscription(sub.id); await load() }) }
async function pollMigrationStatus() {
  if (migratePolling.value) return
  migratePolling.value = true
  try {
    const status = await endpoints.getMigrationStatus()
    migrateStatus.value = status
    if (status.status === 'running') {
      migratePollTimer = setInterval(async () => {
        try {
          const s = await endpoints.getMigrationStatus()
          migrateStatus.value = s
          if (s.status !== 'running') stopMigrationPolling()
        } catch { stopMigrationPolling() }
      }, 2000)
    }
  } catch (cause) { error.value = cause instanceof Error ? cause.message : t('operationFailed') }
  finally { migratePolling.value = false }
}
function stopMigrationPolling() {
  if (migratePollTimer) { clearInterval(migratePollTimer); migratePollTimer = null }
}
async function runMigration() {
  error.value = ''
  try {
    await endpoints.runMigration(migrateForm)
    await pollMigrationStatus()
  } catch (cause) { error.value = cause instanceof Error ? cause.message : t('operationFailed') }
}
async function savePlan() {
  await action(async () => {
    const payload = { name: subscriptionPlanForm.name, description: subscriptionPlanForm.description, price: subscriptionPlanForm.price, currency: subscriptionPlanForm.currency, billing_period: subscriptionPlanForm.billing_period, credit_amount: subscriptionPlanForm.credit_amount, group_id: subscriptionPlanForm.group_id, model_whitelist: subscriptionPlanForm.model_whitelist.split(',').map((v) => v.trim()).filter(Boolean), max_requests_per_period: subscriptionPlanForm.max_requests_per_period === '' ? null : Number(subscriptionPlanForm.max_requests_per_period), max_tokens_per_period: subscriptionPlanForm.max_tokens_per_period === '' ? null : Number(subscriptionPlanForm.max_tokens_per_period), sort_order: subscriptionPlanForm.sort_order, enabled: subscriptionPlanForm.enabled }
    if (editingPlanID.value) await endpoints.updateSubscriptionPlan(editingPlanID.value, payload)
    else await endpoints.createSubscriptionPlan(payload)
    showPlanModal.value = false; editingPlanID.value = ''
    Object.assign(subscriptionPlanForm, { name: '', description: '', price: '0', currency: 'CNY', billing_period: 'month', credit_amount: '0', group_id: '', model_whitelist: '', max_requests_per_period: '', max_tokens_per_period: '', sort_order: 0, enabled: true })
    await load()
  })
}
function openPlanModal() { editingPlanID.value = ''; Object.assign(subscriptionPlanForm, { name: '', description: '', price: '0', currency: 'CNY', billing_period: 'month', credit_amount: '0', group_id: '', model_whitelist: '', max_requests_per_period: '', max_tokens_per_period: '', sort_order: 0, enabled: true }); showPlanModal.value = true }
function editPlan(plan: SubscriptionPlan) { editingPlanID.value = plan.id; Object.assign(subscriptionPlanForm, { name: plan.name, description: plan.description, price: plan.price, currency: plan.currency, billing_period: plan.billing_period, credit_amount: plan.credit_amount, group_id: plan.group_id, model_whitelist: plan.model_whitelist.join(', '), max_requests_per_period: plan.max_requests_per_period ?? '', max_tokens_per_period: plan.max_tokens_per_period ?? '', sort_order: plan.sort_order, enabled: plan.enabled }); showPlanModal.value = true }
async function deletePlan(plan: SubscriptionPlan) { if (!confirm(t('deletePlanConfirm').replace('{name}', plan.name))) return; await action(async () => { await endpoints.deleteSubscriptionPlan(plan.id); await load() }) }
async function extendSubscriptions() {
  if (!extendForm.plan_id || !extendForm.days) return
  await action(async () => {
    const result = await endpoints.batchExtendSubscriptions(extendForm.plan_id, extendForm.days)
    await loadAdminSubscriptions()
    error.value = t('extendSuccess').replace('{count}', String(result.affected)).replace('{days}', String(extendForm.days))
  })
}
function manageUser(user: User) { originalUser.value = user; selectedUser.value = { ...user }; selectedPermissions.value = [...user.permissions]; selectedGroups.value = [...(user.groups ?? [])]; userPassword.value = ''; userBalance.value = Number(user.balance ?? 0); userBalanceNote.value = '' }
async function saveUserAccess() {
  if (!selectedUser.value || !originalUser.value) return
  const current = selectedUser.value
  const original = originalUser.value
  const update: Record<string, unknown> = {}
  if (current.name !== original.name) update.name = current.name
  if (current.email !== original.email) update.email = current.email
  if (current.role !== original.role) update.role = current.role
  if (current.enabled !== original.enabled) update.enabled = current.enabled
  if (userPassword.value) update.password = userPassword.value
  if (Number(userBalance.value) !== Number(original.balance ?? 0)) { update.balance = userBalance.value; update.note = userBalanceNote.value }
  if ([...selectedPermissions.value].sort().join('\n') !== [...original.permissions].sort().join('\n')) update.permissions = selectedPermissions.value
  if ([...selectedGroups.value].sort().join('\n') !== [...(original.groups ?? [])].sort().join('\n')) update.groups = selectedGroups.value
  if (!Object.keys(update).length) { selectedUser.value = null; originalUser.value = null; return }
  await action(async () => { await endpoints.updateUser(current.id, update as Parameters<typeof endpoints.updateUser>[1]); selectedUser.value = null; originalUser.value = null; await load() })
}
async function chooseAvatar(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type) || file.size > 1.5 * 1024 * 1024) { error.value = t('avatarFileRequirements'); return }
  await action(async () => {
    const avatarURL = await new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = () => reject(new Error(t('avatarReadFailed'))); reader.readAsDataURL(file) })
    await endpoints.updateAccountProfile(avatarURL)
    await load()
  })
  if (avatarInput.value) avatarInput.value.value = ''
}
async function removeAvatar() { await action(async () => { await endpoints.updateAccountProfile(''); await load() }) }
async function changePassword() {
  passwordMessage.value = ''
  if (passwordForm.new_password !== passwordForm.confirm_password) {
    error.value = t('passwordMismatch')
    return
  }
  await action(async () => {
    await endpoints.changeAccountPassword(passwordForm.current_password, passwordForm.new_password)
    passwordForm.current_password = ''
    passwordForm.new_password = ''
    passwordForm.confirm_password = ''
    passwordMessage.value = t('passwordChanged')
  })
}
async function saveAvatarUrl() {
  const url = avatarUrlInput.value.trim()
  if (!url) return
  try {
    const parsed = new URL(url)
    if (!['http:', 'https:'].includes(parsed.protocol)) { error.value = t('avatarUrlInvalid'); return }
  } catch { error.value = t('avatarUrlInvalid'); return }
  await action(async () => {
    await endpoints.updateAccountProfile(url)
    avatarUrlInput.value = ''
    await load()
  })
}
function openConsoleOrAuth() { router.push(authenticated.value ? '/console/overview' : '/auth') }
function openConsole(nextView: string) { if (views.includes(nextView as View)) { currentView.value = nextView as View; router.push({ path: '/console', query: { view: nextView } }) } }
onMounted(async () => {
  initializeLocale()
  await loadSiteSettings().catch(() => undefined)
  authenticated.value = Boolean(getToken())
  if (isMarketplacePage.value) { await loadCatalog().catch((cause) => { error.value = cause instanceof Error ? cause.message : t('loadFailed') }); return }
  if (!authenticated.value) return
  await load()
  const returnedOrder = typeof route.query.payment_order === 'string' ? route.query.payment_order : ''
  const returnedSubOrder = typeof route.query.order === 'string' ? route.query.order : ''
  if (returnedOrder) {
    const order = await endpoints.getAccountPayment(returnedOrder).catch(() => null)
    paymentMessage.value = order?.status === 'paid' ? t('paymentPaid') : t('paymentPending')
    if (order?.status === 'paid') await load()
  }
  if (returnedSubOrder) {
    const order = await endpoints.getAccountSubscriptionOrder(returnedSubOrder).catch(() => null)
    subscriptionMessage.value = order?.status === 'paid' ? t('subscriptionPaid') : t('subscriptionPending')
    if (order?.status === 'paid') await load()
  }
  if (route.meta.requiresAuth && error.value) {
    clearToken()
    authenticated.value = false
    await router.replace('/auth')
  }
})

onUnmounted(() => stopMigrationPolling())

// --- View components (lazy-loaded) -----------------------------------------
const viewComponents: Partial<Record<View, ReturnType<typeof defineAsyncComponent>>> = {
  overview: defineAsyncComponent(() => import('~/components/console/Overview.vue')),
  users: defineAsyncComponent(() => import('~/components/console/Users.vue')),
  groups: defineAsyncComponent(() => import('~/components/console/Groups.vue')),
  keys: defineAsyncComponent(() => import('~/components/console/Keys.vue')),
  channels: defineAsyncComponent(() => import('~/components/console/Channels.vue')),
  providers: defineAsyncComponent(() => import('~/components/console/Providers.vue')),
  account: defineAsyncComponent(() => import('~/components/console/Account.vue')),
  profile: defineAsyncComponent(() => import('~/components/console/Profile.vue')),
  wallet: defineAsyncComponent(() => import('~/components/console/Wallet.vue')),
  usage: defineAsyncComponent(() => import('~/components/console/Usage.vue')),
  'usage-overview': defineAsyncComponent(() => import('~/components/console/UsageOverview.vue')),
  ledger: defineAsyncComponent(() => import('~/components/console/Ledger.vue')),
  'site-settings': defineAsyncComponent(() => import('~/components/console/SiteSettings.vue')),
  reliability: defineAsyncComponent(() => import('~/components/console/Reliability.vue')),
  'payment-settings': defineAsyncComponent(() => import('~/components/console/PaymentSettings.vue')),
  pricing: defineAsyncComponent(() => import('~/components/console/Pricing.vue')),
  subscriptions: defineAsyncComponent(() => import('~/components/console/Subscriptions.vue')),
  'subscription-plans': defineAsyncComponent(() => import('~/components/console/SubscriptionPlans.vue')),
  'admin-subscriptions': defineAsyncComponent(() => import('~/components/console/AdminSubscriptions.vue')),
  migrate: defineAsyncComponent(() => import('~/components/console/Migrate.vue')),
}
const currentViewComponent = computed(() => viewComponents[view.value])

// --- Provide the console store for all child view/modal components --------
provide(CONSOLE_STORE_KEY, {
  locale, t, setLocale, toggleLocale, initializeLocale,
  route, router, view, views, openConsole,
  authenticated, busy, error, sidebarCollapsed, setupCollapsed, account, can, isAdmin, signOut,
  generalNav, billingNav, personalNav, managementNav,
  localizedManagementNavItems, localizedAdminExtraNav,
  users, groups, ownGroups, keys, accountKeys, channels, providers,
  activityLogs, usageRecords, ledger, payments, paymentMethods,
  paymentsEnabled, paymentMessage, paymentForm, paymentSettings, paymentSettingsForm, paymentMethodForm,
  pricing, siteSettings, siteSettingsForm, reliabilityForm, newAPIPricingForm,
  catalog, catalogGroups, catalogLoaded, catalogGroup, catalogSearch,
  activityModels, activityFilters, leaderboardPrefs,
  subscriptionPlans, publicPlans, userSubscriptions, subscriptionOrders, adminSubscriptions,
  subscriptionPlanForm, editingPlanID, showPlanModal, subscribingPlan, subscribeForm, subscriptionMessage, extendForm,
  createdKey, showKey, showAccountKey, editingAccountKey, showChannel, editingChannel, showProvider,
  selectedUser, originalUser, selectedPermissions, selectedGroups,
  userPassword, userBalance, userBalanceNote,
  keyForm, accountKeyForm, channelForm, providerForm, editingProviderID, groupForm, groupImportText,
  avatarUrlInput, avatarInput, passwordForm, passwordMessage, changePassword,
  load, action, loadActivity, filterActivity, resetActivityFilters,
  createKey, createAccountKey, editAccountKey, updateAccountKey,
  fetchChannelModels, createChannel, editChannel, updateChannel,
  saveProvider, openProvider, editProvider, removeProvider,
  createGroup, editGroupMultiplier, importGroups,
  toggleChannel, revokeKey, createPayment, savePaymentSettings,
  createPaymentMethod, updatePaymentMethod, deletePaymentMethod,
  copyKey, savePricing, syncNewAPIPricing,
  manageUser, saveUserAccess, chooseAvatar, removeAvatar, saveAvatarUrl,
  saveLeaderboardPrefs, saveSiteSettings, saveReliabilitySettings,
  loadPublicPlans, openSubscribeModal, confirmSubscribe, cancelSubscription,
  savePlan, openPlanModal, editPlan, deletePlan, extendSubscriptions,
  migrateForm, migrateStatus, migratePolling, runMigration, pollMigrationStatus, stopMigrationPolling,
  personalRequests, personalTokens, personalCost, setupProgress,
  filteredCatalog, apiEndpoint, usageChart, usageLinePoints,
  userName, formatDate, short, formatPrice, providerIcon, modelProvider,
  selectedCatalogGroup, actualMultiplier,
  activityTypeLabel, actionLabel, activityDetail,
  Empty, permissions,
})
</script>

<template>
  <ErrorAlert />
  <LandingPage v-if="isLanding" :site-name="siteSettings.name" :authenticated="authenticated" @open-console-or-auth="openConsoleOrAuth" />

  <main v-else-if="isMarketplacePage" class="public-marketplace">
    <PublicTopbar :site-name="siteSettings.name" :authenticated="authenticated" />
    <ModelSquare :catalog="catalog" :groups="catalogGroups" :loaded="catalogLoaded" />
  </main>

  <AuthPage v-else-if="isAuthPage" />

  <main v-else class="flex min-h-screen bg-background text-foreground">
    <ConsoleSidebar />
    <section class="flex flex-1 flex-col overflow-hidden">
      <ConsoleHeader />
      <div class="flex-1 overflow-y-auto p-4">
        <component :is="currentViewComponent" v-if="currentViewComponent" :key="view" />
        <ConsoleModals />
      </div>
    </section>
  </main>
</template>
