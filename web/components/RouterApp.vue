<script setup lang="ts">
import { computed, defineAsyncComponent, h, onBeforeUnmount, onMounted, provide, reactive, ref, watch } from 'vue'
import { Activity, Bot, Check, ChevronRight, CircleAlert, Copy, KeyRound, Layers3, LayoutDashboard, LogOut, PanelLeftClose, PanelLeftOpen, RadioTower, RefreshCw, ShieldCheck, Sparkles, TerminalSquare, UserRound, Users, WalletCards, ReceiptText, Tags, Settings, Crown } from 'lucide-vue-next'
import { api, clearToken, getToken, setToken } from '~/src/api'
import ModelSquare from '~/components/marketplace/ModelSquare.vue'
import type { Account, ActivityLog, AdminSiteSettings, AdminSubscription, ApiKey, CatalogGroup, CatalogModel, Channel, Group, LedgerEntry, ModelProvider, PaymentMethod, PaymentOrder, PaymentSettings, Pricing, PublicSubscriptionPlan, ReliabilitySettings, SiteSettings, SubscriptionOrder, SubscriptionPlan, UsageRecord, User, UserSubscription } from '~/src/api'
const { locale, t, setLocale, toggleLocale, initializeLocale } = useI18n()

import type { View } from '~/src/views'
import { VIEWS } from '~/src/views'
import { CONSOLE_STORE_KEY } from '~/composables/useConsoleStore'
const props = withDefaults(defineProps<{ activeView?: View }>(), { activeView: 'overview' })
const route = useRoute()
const router = useRouter()
const views: View[] = VIEWS
const view = computed<View>(() => {
  const selected = views.includes(route.query.view as View) ? route.query.view as View : props.activeView && views.includes(props.activeView) ? props.activeView : views.includes(route.params.view as View) ? route.params.view as View : 'overview'
  return selected === 'logs' || selected === 'audit' || selected === 'usage-overview' ? 'usage' : selected
})
const authenticated = ref(false)
const error = ref('')
const errorAlert = ref<HTMLElement | null>(null)
const errorHovered = ref(false)
const errorSelected = ref(false)
let errorTimer: ReturnType<typeof setTimeout> | undefined
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
const adminExtraNav = [['pricing', 'pricing', Tags, 'pricing.read'], ['reliability', 'reliability', ShieldCheck, 'system.manage'], ['subscription-plans', 'subscriptionPlans', Crown, 'system.manage'], ['admin-subscriptions', 'adminSubscriptions', Crown, 'users.read'], ['site-settings', 'siteSettings', Settings, 'system.manage'], ['payment-settings', 'paymentSettings', WalletCards, 'system.manage']] as const
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
const selectedSubscriptionOrder = ref('')
const siteSettingsForm = reactive<AdminSiteSettings & { geetest_captcha_key: string; smtp_password: string }>({ name: '', icon_url: '', auto_disable_failed_channels: false, geetest_captcha_id: '', has_geetest_captcha_key: false, geetest_captcha_key: '', smtp_host: '', smtp_port: '465', smtp_username: '', has_smtp_password: false, smtp_password: '', smtp_from: '' })
const reliabilityForm = reactive<ReliabilitySettings>({ retry_count: 3, retry_status_codes: '', health_check_mode: 'off', health_check_interval_minutes: 5, health_check_auto_recover: true, health_check_channel_ids: '', auto_disable_on_test_failure: false, auto_disable_slow_seconds: 0, auto_disable_status_codes: '', auto_disable_keywords: '' })
const newAPIPricingForm = reactive({ base_url: '', api_key: '', price_per_quota_unit: 1 })
const loginMode = ref<'token' | 'login' | 'register'>('login')
const accountForm = reactive({ name: '', email: '', password: '' })
const leaderboardPrefs = reactive({ opt_in: true, mask_name: true })
async function saveLeaderboardPrefs() { await action(async () => { await api('/account/preferences', { method: 'PUT', body: JSON.stringify({ leaderboard_opt_in: leaderboardPrefs.opt_in, leaderboard_mask_name: leaderboardPrefs.mask_name }) }); if (account.value) { account.value.leaderboard_opt_in = leaderboardPrefs.opt_in; account.value.leaderboard_mask_name = leaderboardPrefs.mask_name } }) }
// Geetest v4 CAPTCHA — loaded lazily the first time sign-in requires it.
declare global { interface Window { initGeetest4?: (options: Record<string, unknown>, callback: (captcha: GeetestCaptcha) => void) => void } }
interface GeetestCaptcha { onReady(fn: () => void): void; onSuccess(fn: () => void): void; onClose(fn: () => void): void; onError(fn: (cause: unknown) => void): void; showCaptcha(): void; getValidate(): Record<string, string> | null }
let geetestScript: Promise<void> | null = null
let geetestInstance: GeetestCaptcha | null = null
let geetestReady: Promise<void> | null = null
function loadGeetestScript() { geetestScript ??= new Promise<void>((resolve, reject) => { const script = document.createElement('script'); script.src = 'https://static.geetest.com/v4/gt4.js'; script.onload = () => resolve(); script.onerror = () => reject(new Error('captcha script failed')); document.head.appendChild(script) }); return geetestScript }
function ensureGeetest() { if (!siteSettings.value.geetest_enabled || !siteSettings.value.geetest_captcha_id) return Promise.resolve(); geetestReady ??= loadGeetestScript().then(() => new Promise<void>((resolve, reject) => { window.initGeetest4?.({ captchaId: siteSettings.value.geetest_captcha_id, product: 'float', language: locale.value === 'en-US' ? 'eng' : 'zho' }, (captcha) => { geetestInstance = captcha; captcha.onReady(() => resolve()); captcha.onError(reject) }) })); return geetestReady }
function runGeetest(): Promise<Record<string, string>> { return new Promise((resolve, reject) => { const captcha = geetestInstance; if (!captcha) return resolve({}); const cleanup = () => { captcha.onSuccess(() => {}); captcha.onClose(() => {}) }; captcha.onSuccess(() => { const result = captcha.getValidate(); cleanup(); result ? resolve(result) : reject(new Error('captcha failed')) }); captcha.onClose(() => { cleanup(); reject(new Error('captcha closed')) }); captcha.showCaptcha() }) }
// Registration email verification code.
const emailCode = ref('')
const codeSending = ref(false)
const codeCountdown = ref(0)
const codeSentHint = ref('')
let codeTimer = 0
const emailLooksValid = computed(() => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(accountForm.email.trim()))
async function sendEmailCode() {
  if (!emailLooksValid.value || codeSending.value || codeCountdown.value > 0) return
  let captcha: Record<string, string> = {}
  if (siteSettings.value.geetest_enabled) { try { await ensureGeetest(); captcha = await runGeetest() } catch { return } }
  codeSending.value = true
  try {
    await api('/auth/email-code', { method: 'POST', body: JSON.stringify({ email: accountForm.email.trim(), ...captcha }) })
    codeSentHint.value = t('codeSent')
    codeCountdown.value = 60
    window.clearInterval(codeTimer)
    codeTimer = window.setInterval(() => { codeCountdown.value -= 1; if (codeCountdown.value <= 0) window.clearInterval(codeTimer) }, 1000)
  } catch (cause) {
    codeSentHint.value = cause instanceof Error ? cause.message : t('operationFailed')
  } finally {
    codeSending.value = false
  }
}
const avatarInput = ref<HTMLInputElement | null>(null)
const avatarUrlInput = ref('')
const isLanding = computed(() => route.path === '/')
const isAuthPage = computed(() => route.path === '/auth')
const isMarketplacePage = computed(() => route.path === '/models')
const activeChannels = computed(() => channels.value.filter((channel) => channel.enabled).length)
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

function clearErrorTimer() {
  if (errorTimer) window.clearTimeout(errorTimer)
  errorTimer = undefined
}

function scheduleErrorDismissal() {
  clearErrorTimer()
  if (!error.value || errorHovered.value || errorSelected.value) return
  errorTimer = window.setTimeout(() => { error.value = '' }, 5000)
}

function updateErrorSelection() {
  const selection = window.getSelection()
  const anchor = selection?.anchorNode
  const focus = selection?.focusNode
  errorSelected.value = Boolean(selection?.toString() && anchor && focus && errorAlert.value?.contains(anchor) && errorAlert.value?.contains(focus))
  scheduleErrorDismissal()
}

function lockError() {
  errorHovered.value = true
  clearErrorTimer()
}

function releaseError() {
  errorHovered.value = false
  updateErrorSelection()
  if (!errorSelected.value) error.value = ''
}

async function copyError() {
  if (!error.value) return
  if (navigator.clipboard) {
    await navigator.clipboard.writeText(error.value)
    return
  }
  const textarea = document.createElement('textarea')
  textarea.value = error.value
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.append(textarea)
  textarea.select()
  document.execCommand('copy')
  textarea.remove()
}

watch(error, () => {
  errorSelected.value = false
  scheduleErrorDismissal()
})

const activityTypeLabel = computed(() => ({ request: 'Request', login: 'Login', register: 'Register', logout: 'Logout', topup: 'Top-up', operation: t('otherOperation') } as Record<ActivityLog['type'], string>))
const actionLabel = (item: ActivityLog) => ({ 'account.logged_in': t('actionAccountLogin'), 'account.registered': t('actionAccountRegister'), 'account.logged_out': t('actionAccountLogout'), 'wallet.adjusted': t('actionWalletAdjusted') }[item.action] ?? item.action)
const activityDetail = (item: ActivityLog) => item.type === 'request' ? `${item.prompt_tokens} / ${item.completion_tokens} tokens · ${Number(item.cost).toFixed(6)}` : JSON.stringify(item.details)

async function loadActivity(filters = false) {
  const query = new URLSearchParams()
  if (filters) Object.entries(activityFilters).forEach(([key, value]) => { if (value) query.set(key, key === 'start' || key === 'end' ? new Date(value).toISOString() : value) })
  const value = await api<{ data: ActivityLog[] }>(`/activity-logs${query.size ? `?${query}` : ''}`)
  activityLogs.value = value.data
  if (!filters) activityModels.value = [...new Set(value.data.map((item) => item.model).filter(Boolean))].sort()
}
async function filterActivity() { await action(() => loadActivity(true)) }
async function resetActivityFilters() { Object.assign(activityFilters, { user_id: '', model: '', group_id: '', start: '', end: '', type: '' }); await action(() => loadActivity()) }

async function load() {
  busy.value = true; error.value = ''
  try {
    const [settings, me] = await Promise.all([api<SiteSettings>('/site-settings'), api<Account>('/account/me')])
    siteSettings.value = settings
    Object.assign(siteSettingsForm, settings)
    account.value = me
    leaderboardPrefs.opt_in = me.leaderboard_opt_in; leaderboardPrefs.mask_name = me.leaderboard_mask_name
    const [ownKeys, ownUsage, ownLedger, ownGroupValue, ownPayments] = await Promise.all([
      api<{ data: ApiKey[] }>('/account/keys').catch(() => ({ data: [] })),
      api<{ data: UsageRecord[] }>('/account/usage').catch(() => ({ data: [] })),
      api<{ data: LedgerEntry[] }>('/account/ledger').catch(() => ({ data: [] })),
      api<{ data: string[]; groups: Group[] }>('/account/groups').catch(() => ({ data: [], groups: [] })),
      api<{ enabled: boolean; payment_methods: PaymentMethod[]; data: PaymentOrder[] }>('/account/payments').catch(() => ({ enabled: false, payment_methods: [], data: [] })),
    ])
    accountKeys.value = ownKeys.data; usageRecords.value = ownUsage.data; ledger.value = ownLedger.data; ownGroups.value = ownGroupValue.data
    paymentsEnabled.value = ownPayments.enabled; payments.value = ownPayments.data; paymentMethods.value = ownPayments.payment_methods ?? []
    if (!paymentMethods.value.some((method) => method.code === paymentForm.type)) paymentForm.type = paymentMethods.value[0]?.code ?? ''
    const ownSubs = await api<{ data: UserSubscription[] }>('/account/subscriptions').catch(() => ({ data: [] as UserSubscription[] }))
    userSubscriptions.value = ownSubs.data
    const ownSubOrders = await api<{ data: SubscriptionOrder[] }>('/account/subscription-orders').catch(() => ({ data: [] as SubscriptionOrder[] }))
    subscriptionOrders.value = ownSubOrders.data
    await loadActivity()
    if (!can('users.read')) groups.value = ownGroupValue.groups
    const requests: Promise<void>[] = []
    if (can('users.read')) requests.push(Promise.all([api<{ data: User[] }>('/admin/users'), api<{ data: Group[] }>('/admin/groups')]).then(([userValue, groupValue]) => { users.value = userValue.data; groups.value = groupValue.data }))
    if (can('keys.manage')) requests.push(api<{ data: ApiKey[] }>('/admin/keys').then((value) => { keys.value = value.data }))
    if (can('channels.read')) requests.push(api<{ data: Channel[] }>('/admin/channels').then((value) => { channels.value = value.data }))
    if (can('system.manage')) requests.push(api<{ data: ModelProvider[] }>('/admin/providers').then((value) => { providers.value = value.data }))
    if (can('system.manage')) requests.push(api<PaymentSettings>('/admin/payment-settings').then((value) => { Object.assign(paymentSettings, value); Object.assign(paymentSettingsForm, { enabled: value.enabled, base_url: value.base_url, merchant_id: value.merchant_id, merchant_key: '', public_base_url: value.public_base_url }) }))
    if (can('pricing.read')) requests.push(api<{ data: Pricing[] }>('/admin/pricing').then((value) => { pricing.value = value.data }))
    if (can('system.manage')) requests.push(api<ReliabilitySettings>('/admin/reliability-settings').then((value) => { Object.assign(reliabilityForm, value) }))
    if (can('system.manage')) requests.push(api<{ data: SubscriptionPlan[] }>('/admin/subscription-plans').then((value) => { subscriptionPlans.value = value.data }))
    if (can('users.read')) requests.push(api<{ data: AdminSubscription[] }>('/admin/subscriptions').then((value) => { adminSubscriptions.value = value.data }))
    if (can('system.manage')) requests.push(api<AdminSiteSettings>('/admin/site-settings').then((value) => { Object.assign(siteSettingsForm, value); siteSettingsForm.geetest_captcha_key = ''; siteSettingsForm.smtp_password = '' }))
    await Promise.all(requests)
    } catch (cause) { error.value = cause instanceof Error ? cause.message : t('loadFailed') } finally { busy.value = false }
}
async function loadSiteSettings() { const value = await api<SiteSettings>('/site-settings'); siteSettings.value = value; Object.assign(siteSettingsForm, value); document.title = value.name; const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]') ?? document.head.appendChild(Object.assign(document.createElement('link'), { rel: 'icon' })); if (value.icon_url) link.href = value.icon_url; else link.removeAttribute('href') }
async function saveSiteSettings() { await action(async () => { const value = await api<AdminSiteSettings>('/admin/site-settings', { method: 'PUT', body: JSON.stringify(siteSettingsForm) }); Object.assign(siteSettingsForm, value); siteSettingsForm.geetest_captcha_key = ''; siteSettingsForm.smtp_password = ''; await loadSiteSettings() }) }
async function saveReliabilitySettings() { await action(async () => { const value = await api<ReliabilitySettings>('/admin/reliability-settings', { method: 'PUT', body: JSON.stringify(reliabilityForm) }); Object.assign(reliabilityForm, value) }) }
async function loadCatalog() { try { const value = await api<{ data: CatalogModel[]; groups: CatalogGroup[] }>('/model-catalog'); catalog.value = value.data; catalogGroups.value = value.groups } finally { catalogLoaded.value = true } }
async function accountSignIn(register: boolean) { let captcha: Record<string, string> = {}; const emailVerify = register && siteSettings.value.email_verification_enabled; if (siteSettings.value.geetest_enabled && !emailVerify) { try { await ensureGeetest(); captcha = await runGeetest() } catch { return } } await action(async () => { const result = await api<{ token: string }>(register ? '/auth/register' : '/auth/login', { method: 'POST', body: JSON.stringify({ ...(register ? accountForm : { email: accountForm.email, password: accountForm.password }), ...(emailVerify ? { code: emailCode.value.trim() } : {}), ...captcha }) }); setToken(result.token); authenticated.value = true; await load(); await router.replace({ path: '/console', query: { view: managementNav.value.length ? 'overview' : 'account' } }) }) }
async function signOut() { try { await api('/auth/logout', { method: 'POST' }) } catch { /* Local session removal is sufficient when the server is unreachable. */ } clearToken(); authenticated.value = false; error.value = ''; await router.replace('/') }
async function createKey() { await action(async () => { const response = await api<{ key: string }>('/admin/keys', { method: 'POST', body: JSON.stringify({ ...keyForm, expires_at: keyForm.expires_at ? new Date(keyForm.expires_at).toISOString() : '' }) }); createdKey.value = response.key; showKey.value = false; Object.assign(keyForm, { user_id: '', name: '', expires_at: '', group_id: '' }); await load() }) }
async function createAccountKey() { await action(async () => { const response = await api<{ key: string }>('/account/keys', { method: 'POST', body: JSON.stringify({ ...accountKeyForm, expires_at: accountKeyForm.expires_at ? new Date(accountKeyForm.expires_at).toISOString() : '' }) }); createdKey.value = response.key; showAccountKey.value = false; Object.assign(accountKeyForm, { name: '', expires_at: '', group_id: '' }); await load() }) }
function editAccountKey(key: ApiKey) { editingAccountKey.value = key; Object.assign(accountKeyForm, { name: key.name, expires_at: key.expires_at ? new Date(key.expires_at).toISOString().slice(0, 16) : '', group_id: key.group_id }) }
async function updateAccountKey() { if (!editingAccountKey.value) return; await action(async () => { await api(`/account/keys/${editingAccountKey.value.id}`, { method: 'PUT', body: JSON.stringify({ ...accountKeyForm, expires_at: accountKeyForm.expires_at ? new Date(accountKeyForm.expires_at).toISOString() : '' }) }); editingAccountKey.value = null; Object.assign(accountKeyForm, { name: '', expires_at: '', group_id: '' }); await load() }) }
async function fetchChannelModels() { await action(async () => { const response = await api<{ models: string[] }>('/admin/channels/models', { method: 'POST', body: JSON.stringify({ base_url: channelForm.base_url, api_key: channelForm.api_key }) }); channelForm.models = response.models.join(', ');     if (!response.models.length) throw new Error(t('upstreamNoModels')) }) }
async function createChannel() { await action(async () => { await api('/admin/channels', { method: 'POST', body: JSON.stringify({ ...channelForm, models: channelForm.models.split(',').map((value) => value.trim()).filter(Boolean) }) }); showChannel.value = false; Object.assign(channelForm, { name: '', provider: 'openai', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100, groups: [] }); await load() }) }
function editChannel(channel: Channel) { editingChannel.value = channel; Object.assign(channelForm, { name: channel.name, provider: channel.provider, base_url: channel.base_url, api_key: '', models: channel.models.join(', '), priority: channel.priority, groups: [...channel.groups] }) }
async function updateChannel() { if (!editingChannel.value) return; await action(async () => { const models = channelForm.models.split(',').map((value) => value.trim()).filter(Boolean); await api(`/admin/channels/${editingChannel.value.id}`, { method: 'PUT', body: JSON.stringify({ ...channelForm, models }) }); await api(`/admin/channels/${editingChannel.value.id}/groups`, { method: 'PUT', body: JSON.stringify({ groups: channelForm.groups }) }); editingChannel.value = null; Object.assign(channelForm, { name: '', provider: 'openai', base_url: 'https://api.openai.com', api_key: '', models: '', priority: 100, groups: [] }); await load() }) }
async function saveProvider() { await action(async () => { await api('/admin/providers', { method: 'POST', body: JSON.stringify({ ...providerForm, id: editingProviderID.value || undefined, prefixes: providerForm.prefixes.split(',').map((value) => value.trim()).filter(Boolean) }) }); showProvider.value = false; editingProviderID.value = ''; Object.assign(providerForm, { name: '', slug: '', prefixes: '', priority: 100 }); await load() }) }
function openProvider() { editingProviderID.value = ''; Object.assign(providerForm, { name: '', slug: '', prefixes: '', priority: 100 }); showProvider.value = true }
function editProvider(provider: ModelProvider) { editingProviderID.value = provider.id; Object.assign(providerForm, { name: provider.name, slug: provider.slug, prefixes: provider.prefixes.join(', '), priority: provider.priority }); showProvider.value = true }
async function removeProvider(provider: ModelProvider) { if (!confirm(t('deleteProviderConfirm').replace('{name}', provider.name))) return; await action(async () => { await api(`/admin/providers/${provider.id}`, { method: 'DELETE' }); await load() }) }
async function createGroup() { const name = groupForm.name.trim(); const multiplier = Number(groupForm.multiplier);     if (!name) { error.value = t('enterGroupName'); return } if (!Number.isFinite(multiplier) || multiplier <= 0) { error.value = t('multiplierMustBePositive'); return } await action(async () => { await api('/admin/groups', { method: 'POST', body: JSON.stringify({ name, multiplier }) }); Object.assign(groupForm, { name: '', multiplier: 1 }); await load() }) }
async function editGroupMultiplier(group: Group, event: Event) { const multiplier = Number(new FormData(event.currentTarget as HTMLFormElement).get('multiplier'));     if (!Number.isFinite(multiplier) || multiplier < 0) { error.value = t('multiplierMustBeNonNegative'); return } await action(async () => { await api(`/admin/groups/${group.id}`, { method: 'PUT', body: JSON.stringify({ multiplier }) }); await load() }) }
async function importGroups() { let values: Record<string, unknown>; try { values = JSON.parse(groupImportText.value) } catch { error.value = t('enterValidJSON'); return } if (!values || Array.isArray(values) || typeof values !== 'object') { error.value = t('importMustBeGroupMultiplierJSON'); return } const entries = Object.entries(values); if (!entries.length || entries.some(([name, multiplier]) => !name.trim() || typeof multiplier !== 'number' || !Number.isFinite(multiplier) || multiplier < 0)) { error.value = t('groupNameRequiredMultiplierNonNegative'); return } await action(async () => { await api('/admin/groups/import', { method: 'POST', body: JSON.stringify(values) }); groupImportText.value = ''; await load() }) }
async function toggleChannel(channel: Channel) { await action(async () => { await api(`/admin/channels/${channel.id}/status`, { method: 'POST', body: JSON.stringify({ enabled: !channel.enabled }) }); await load() }) }
async function revokeKey(key: ApiKey) { if (!confirm(t('revokeKeyConfirm').replace('{prefix}', key.key_prefix))) return; await action(async () => { await api(`/admin/keys/${key.id}/revoke`, { method: 'POST' }); await load() }) }
async function action(work: () => Promise<void>) { busy.value = true; error.value = ''; try { await work() } catch (cause) { error.value = cause instanceof Error ? cause.message : t('operationFailed') } finally { busy.value = false } }
async function createPayment() {
  const amount = Number(paymentForm.amount)
  if (!Number.isFinite(amount) || amount < 1 || amount > 100000) { error.value = t('paymentAmountRange'); return }
  await action(async () => {
    const result = await api<{ pay_url: string }>('/account/payments', { method: 'POST', body: JSON.stringify({ amount: amount.toFixed(2), type: paymentForm.type }) })
    window.location.assign(result.pay_url)
  })
}
async function savePaymentSettings() { await action(async () => { const value = await api<PaymentSettings>('/admin/payment-settings', { method: 'PUT', body: JSON.stringify(paymentSettingsForm) }); Object.assign(paymentSettings, value); paymentSettingsForm.merchant_key = ''; await load() }) }
async function createPaymentMethod() { await action(async () => { await api('/admin/payment-methods', { method: 'POST', body: JSON.stringify(paymentMethodForm) }); Object.assign(paymentMethodForm, { code: '', name: '', enabled: true }); await load() }) }
async function updatePaymentMethod(method: PaymentMethod) { await action(async () => { await api(`/admin/payment-methods/${method.id}`, { method: 'PUT', body: JSON.stringify({ code: method.code, name: method.name, enabled: method.enabled }) }); await load() }) }
async function deletePaymentMethod(method: PaymentMethod) { if (!confirm(t('deletePaymentMethodConfirm').replace('{name}', method.name))) return; await action(async () => { await api(`/admin/payment-methods/${method.id}`, { method: 'DELETE' }); await load() }) }
async function copyKey() { await navigator.clipboard.writeText(createdKey.value) }
async function savePricing() { await action(async () => { await api('/admin/pricing', { method: 'POST', body: JSON.stringify(pricingForm) }); Object.assign(pricingForm, { model: '', input_per_million: 0, cached_input_per_million: 0, output_per_million: 0, multiplier: 1 }); await load() }) }
async function syncNewAPIPricing() { await action(async () => { const result = await api<{ synced: number }>('/admin/pricing/newapi/sync', { method: 'POST', body: JSON.stringify(newAPIPricingForm) }); await load(); error.value = t('syncPricingResult').replace('{count}', String(result.synced)) }) }

async function loadPublicPlans() { const value = await api<{ data: PublicSubscriptionPlan[] }>('/subscription-plans'); publicPlans.value = value.data }
async function openSubscribeModal(plan: PublicSubscriptionPlan) { if (!paymentMethods.value.length) { error.value = t('paymentNotConfigured'); return } subscribingPlan.value = plan; subscribeForm.payment_type = paymentMethods.value[0]?.code ?? ''; subscribeForm.auto_renew = false; if (!publicPlans.value.length) await loadPublicPlans() }
async function confirmSubscribe() {
  if (!subscribingPlan.value || !subscribeForm.payment_type) return
  await action(async () => {
    const result = await api<{ pay_url: string }>('/account/subscriptions', { method: 'POST', body: JSON.stringify({ plan_id: subscribingPlan.value!.id, payment_type: subscribeForm.payment_type, auto_renew: subscribeForm.auto_renew }) })
    subscribingPlan.value = null
    window.location.assign(result.pay_url)
  })
}
async function cancelSubscription(sub: UserSubscription) { if (!confirm(t('cancelSubscriptionConfirm'))) return; await action(async () => { await api(`/account/subscriptions/${sub.id}/cancel`, { method: 'POST' }); await load() }) }
async function savePlan() {
  await action(async () => {
    const payload = { name: subscriptionPlanForm.name, description: subscriptionPlanForm.description, price: subscriptionPlanForm.price, currency: subscriptionPlanForm.currency, billing_period: subscriptionPlanForm.billing_period, credit_amount: subscriptionPlanForm.credit_amount, group_id: subscriptionPlanForm.group_id, model_whitelist: subscriptionPlanForm.model_whitelist.split(',').map((v) => v.trim()).filter(Boolean), max_requests_per_period: subscriptionPlanForm.max_requests_per_period === '' ? null : Number(subscriptionPlanForm.max_requests_per_period), max_tokens_per_period: subscriptionPlanForm.max_tokens_per_period === '' ? null : Number(subscriptionPlanForm.max_tokens_per_period), sort_order: subscriptionPlanForm.sort_order, enabled: subscriptionPlanForm.enabled }
    if (editingPlanID.value) await api(`/admin/subscription-plans/${editingPlanID.value}`, { method: 'PUT', body: JSON.stringify(payload) })
    else await api('/admin/subscription-plans', { method: 'POST', body: JSON.stringify(payload) })
    showPlanModal.value = false; editingPlanID.value = ''
    Object.assign(subscriptionPlanForm, { name: '', description: '', price: '0', currency: 'CNY', billing_period: 'month', credit_amount: '0', group_id: '', model_whitelist: '', max_requests_per_period: '', max_tokens_per_period: '', sort_order: 0, enabled: true })
    await load()
  })
}
function openPlanModal() { editingPlanID.value = ''; Object.assign(subscriptionPlanForm, { name: '', description: '', price: '0', currency: 'CNY', billing_period: 'month', credit_amount: '0', group_id: '', model_whitelist: '', max_requests_per_period: '', max_tokens_per_period: '', sort_order: 0, enabled: true }); showPlanModal.value = true }
function editPlan(plan: SubscriptionPlan) { editingPlanID.value = plan.id; Object.assign(subscriptionPlanForm, { name: plan.name, description: plan.description, price: plan.price, currency: plan.currency, billing_period: plan.billing_period, credit_amount: plan.credit_amount, group_id: plan.group_id, model_whitelist: plan.model_whitelist.join(', '), max_requests_per_period: plan.max_requests_per_period ?? '', max_tokens_per_period: plan.max_tokens_per_period ?? '', sort_order: plan.sort_order, enabled: plan.enabled }); showPlanModal.value = true }
async function deletePlan(plan: SubscriptionPlan) { if (!confirm(t('deletePlanConfirm').replace('{name}', plan.name))) return; await action(async () => { await api(`/admin/subscription-plans/${plan.id}`, { method: 'DELETE' }); await load() }) }
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
  await action(async () => { await api(`/admin/users/${current.id}`, { method: 'PUT', body: JSON.stringify(update) }); selectedUser.value = null; originalUser.value = null; await load() })
}
async function chooseAvatar(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (!['image/png', 'image/jpeg', 'image/gif', 'image/webp'].includes(file.type) || file.size > 1.5 * 1024 * 1024) { error.value = t('avatarFileRequirements'); return }
  await action(async () => {
    const avatarURL = await new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = () => reject(new Error(t('avatarReadFailed'))); reader.readAsDataURL(file) })
    await api('/account/profile', { method: 'PUT', body: JSON.stringify({ avatar_url: avatarURL }) })
    await load()
  })
  if (avatarInput.value) avatarInput.value.value = ''
}
async function removeAvatar() { await action(async () => { await api('/account/profile', { method: 'PUT', body: JSON.stringify({ avatar_url: '' }) }); await load() }) }
async function saveAvatarUrl() {
  const url = avatarUrlInput.value.trim()
  if (!url) return
  try {
    const parsed = new URL(url)
    if (!['http:', 'https:'].includes(parsed.protocol)) { error.value = t('avatarUrlInvalid'); return }
  } catch { error.value = t('avatarUrlInvalid'); return }
  await action(async () => {
    await api('/account/profile', { method: 'PUT', body: JSON.stringify({ avatar_url: url }) })
    avatarUrlInput.value = ''
    await load()
  })
}
function openAuth() { router.push('/auth') }
function openConsoleOrAuth() { router.push(authenticated.value ? '/console/overview' : '/auth') }
function closeAuth() { router.push('/') }
function openConsole(nextView: string) { if (views.includes(nextView as View)) router.push({ path: '/console', query: { view: nextView } }) }
onMounted(async () => {
  initializeLocale()
  document.addEventListener('selectionchange', updateErrorSelection)
  await loadSiteSettings().catch(() => undefined)
  authenticated.value = Boolean(getToken())
  if (isMarketplacePage.value) { await loadCatalog().catch((cause) => { error.value = cause instanceof Error ? cause.message : t('loadFailed') }); return }
  if (!authenticated.value) return
  await load()
  const returnedOrder = typeof route.query.payment_order === 'string' ? route.query.payment_order : ''
  const returnedSubOrder = typeof route.query.order === 'string' ? route.query.order : ''
  if (returnedOrder) {
    const order = await api<PaymentOrder>(`/account/payments/${encodeURIComponent(returnedOrder)}`).catch(() => null)
    paymentMessage.value = order?.status === 'paid' ? t('paymentPaid') : t('paymentPending')
    if (order?.status === 'paid') await load()
  }
  if (returnedSubOrder) {
    const order = await api<SubscriptionOrder>(`/account/subscription-orders/${encodeURIComponent(returnedSubOrder)}`).catch(() => null)
    subscriptionMessage.value = order?.status === 'paid' ? t('subscriptionPaid') : t('subscriptionPending')
    if (order?.status === 'paid') await load()
  }
  if (route.meta.requiresAuth && error.value) {
    clearToken()
    authenticated.value = false
    await router.replace('/auth')
  }
})

onBeforeUnmount(() => {
  clearErrorTimer()
  document.removeEventListener('selectionchange', updateErrorSelection)
})

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
  ledger: defineAsyncComponent(() => import('~/components/console/Ledger.vue')),
  'site-settings': defineAsyncComponent(() => import('~/components/console/SiteSettings.vue')),
  reliability: defineAsyncComponent(() => import('~/components/console/Reliability.vue')),
  'payment-settings': defineAsyncComponent(() => import('~/components/console/PaymentSettings.vue')),
  pricing: defineAsyncComponent(() => import('~/components/console/Pricing.vue')),
  subscriptions: defineAsyncComponent(() => import('~/components/console/Subscriptions.vue')),
  'subscription-plans': defineAsyncComponent(() => import('~/components/console/SubscriptionPlans.vue')),
  'admin-subscriptions': defineAsyncComponent(() => import('~/components/console/AdminSubscriptions.vue')),
}
const currentViewComponent = computed(() => viewComponents[view.value])

// --- Provide the console store for all child view/modal components --------
provide(CONSOLE_STORE_KEY, {
  locale, t, setLocale, toggleLocale, initializeLocale,
  route, router, view, views, openConsole,
  authenticated, busy, error, sidebarCollapsed, setupCollapsed, account, can, isAdmin,
  users, groups, ownGroups, keys, accountKeys, channels, providers,
  activityLogs, usageRecords, ledger, payments, paymentMethods,
  paymentsEnabled, paymentMessage, paymentForm, paymentSettings, paymentSettingsForm, paymentMethodForm,
  pricing, siteSettings, siteSettingsForm, reliabilityForm, newAPIPricingForm,
  catalog, catalogGroups, catalogLoaded, catalogGroup, catalogSearch,
  activityModels, activityFilters, leaderboardPrefs,
  subscriptionPlans, publicPlans, userSubscriptions, subscriptionOrders, adminSubscriptions,
  subscriptionPlanForm, editingPlanID, showPlanModal, subscribingPlan, subscribeForm, subscriptionMessage,
  createdKey, showKey, showAccountKey, editingAccountKey, showChannel, editingChannel, showProvider,
  selectedUser, originalUser, selectedPermissions, selectedGroups,
  userPassword, userBalance, userBalanceNote,
  keyForm, accountKeyForm, channelForm, providerForm, editingProviderID, groupForm, groupImportText,
  avatarUrlInput, avatarInput,
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
  savePlan, openPlanModal, editPlan, deletePlan,
  personalRequests, personalTokens, personalCost, setupProgress,
  filteredCatalog, apiEndpoint, usageChart, usageLinePoints,
  userName, formatDate, short, formatPrice, providerIcon, modelProvider,
  selectedCatalogGroup, actualMultiplier,
  activityTypeLabel, actionLabel, activityDetail,
  Empty, permissions,
})
</script>

<template>
  <Transition name="error-alert">
    <div v-if="error" ref="errorAlert" class="error-alert" role="alert" tabindex="0" :title="t('clickToCopyError')" @mouseenter="lockError" @mouseleave="releaseError" @click="copyError" @keydown.enter.prevent="copyError" @keydown.space.prevent="copyError">
      <CircleAlert :size="17" /><span>{{ error }}</span><Copy :size="14" aria-hidden="true" />
    </div>
  </Transition>
  <main v-if="isLanding" class="landing-shell">
    <PublicTopbar :site-name="siteSettings.name" :authenticated="authenticated" />
    <section class="hero-section">
      <div class="hero-copy"><p class="eyebrow">OPENAI-COMPATIBLE MODEL GATEWAY</p><h1>{{ t('heroTagline') }}</h1><p class="hero-description">{{ t('heroDescription') }}</p><div class="hero-actions"><button class="button primary hero-button" @click="openConsoleOrAuth">{{ t('openConsole') }} <ChevronRight :size="16" /></button><a class="text-link" href="#quickstart">{{ t('viewRequestExample') }}</a></div></div>
      <div class="hero-visual"><div class="visual-glow"></div><div class="route-card"><div class="route-card-top"><span><i class="live-dot"></i>ROUTER ONLINE</span><code>POST /v1/chat/completions</code></div><div class="route-model"><Bot :size="18" /><strong>kimi-k3</strong><span>{{ t('routingActive') }}</span></div><div class="route-line"><span class="route-node active"></span><div><b>{{ t('openaiMainRoute') }}</b><small>{{ t('priorityLabel') }} P10 &middot; 42ms</small></div><span class="route-check">✓</span></div><div class="route-line muted-route"><span class="route-node"></span><div><b>{{ t('backupChannelText') }}</b><small>{{ t('waitingForFailover') }}</small></div></div><div class="route-footer"><span>{{ t('successRate') }}</span><strong>99.98%</strong><span class="route-divider"></span><span>{{ t('avgLatency') }}</span><strong>186ms</strong></div></div></div>
    </section>
    <section id="features" class="feature-section"><div class="section-intro"><p class="eyebrow">BUILT FOR CONTROL</p><h2>{{ t('featuresTagline') }}</h2></div><div class="feature-grid"><article><span class="feature-number">01</span><RadioTower :size="21" /><h3>{{ t('smartRouting') }}</h3><p>{{ t('smartRoutingDesc') }}</p></article><article><span class="feature-number">02</span><Activity :size="21" /><h3>{{ t('fullObservability') }}</h3><p>{{ t('fullObservabilityDesc') }}</p></article><article><span class="feature-number">03</span><WalletCards :size="21" /><h3>{{ t('usageAndCost') }}</h3><p>{{ t('usageAndCostDesc') }}</p></article></div></section>
    <section id="quickstart" class="quickstart-section"><div><p class="eyebrow">ONE ENDPOINT</p><h2>{{ t('quickstartTagline') }}</h2><p>{{ t('quickstartDesc') }}</p></div><pre><span class="code-comment">// Using OpenAI SDK</span><span><b>const</b> client = <b>new</b> OpenAI({</span><span>  apiKey: <i>'sk-xh-your-key'</i>,</span><span>  baseURL: <i>'http://localhost:8080/v1'</i></span><span>})</span><span class="code-gap"></span><span><b>await</b> client.chat.completions.create({</span><span>  model: <i>'kimi-k3'</i>,</span><span>  messages: [{ role: <i>'user'</i>, content: <i>'你好'</i> }]</span><span>})</span></pre></section>
     <footer class="landing-footer"><span>© 2026 Xinghai Router</span><span>{{ t('footerSlogan') }}</span><span class="legal-links"><a href="/terms">{{ t('termsShort') }}</a><a href="/privacy">{{ t('privacyShort') }}</a></span></footer>
  </main>

  <main v-else-if="isMarketplacePage" class="public-marketplace">
       <PublicTopbar :site-name="siteSettings.name" :authenticated="authenticated" />
    <ModelSquare :catalog="catalog" :groups="catalogGroups" :loaded="catalogLoaded" />
  </main>

      <div v-else-if="isAuthPage" class="public-wrap">
      <PublicTopbar :site-name="siteSettings.name" :authenticated="authenticated" />
      <main class="login-shell">
      <section class="login-card"><aside class="login-aside"><div class="login-aside-glow"></div><div class="login-aside-inner"><span class="brand-mark"><Bot :size="28" /></span><h2>{{ t('loginBrandTitle') }}</h2><p>{{ t('loginBrandDesc') }}</p><ul><li><i><RadioTower :size="14" /></i><span>{{ t('brandPoint1') }}</span></li><li><i><Tags :size="14" /></i><span>{{ t('brandPoint2') }}</span></li><li><i><Activity :size="14" /></i><span>{{ t('brandPoint3') }}</span></li><li><i><ShieldCheck :size="14" /></i><span>{{ t('brandPoint4') }}</span></li></ul></div></aside><div class="login-pane"><h1>{{ loginMode === 'register' ? t('createAccountTab') : t('signInTab') }}</h1><p class="login-sub">{{ loginMode === 'register' ? t('registerSub') : t('loginSub') }}</p><div class="auth-tabs"><button :class="{ active: loginMode === 'login' }" @click="loginMode = 'login'">{{ t('signInTab') }}</button><button :class="{ active: loginMode === 'register' }" @click="loginMode = 'register'">{{ t('createAccountTab') }}</button></div><form @submit.prevent="accountSignIn(loginMode === 'register')"><label v-if="loginMode === 'register'">{{ t('nameLabel') }}<input v-model="accountForm.name" autocomplete="name" required maxlength="100" :placeholder="t('namePlaceholder')" /></label><label>{{ t('emailLabel') }}<input v-model="accountForm.email" type="email" autocomplete="email" required placeholder="name@example.com" /></label><label v-if="loginMode === 'register' && siteSettings.email_verification_enabled">{{ t('emailCodeLabel') }}<span class="code-row"><input v-model="emailCode" inputmode="numeric" autocomplete="one-time-code" maxlength="6" required :placeholder="t('emailCodePlaceholder')" /><button class="button ghost code-send" type="button" :disabled="!emailLooksValid || codeSending || codeCountdown > 0" @click="sendEmailCode">{{ codeCountdown > 0 ? t('codeCountdown').replace('{n}', String(codeCountdown)) : codeSending ? t('codeSending') : t('sendCode') }}</button></span><small v-if="codeSentHint" class="code-hint">{{ codeSentHint }}</small></label><label>{{ t('passwordLabel') }}<input v-model="accountForm.password" type="password" :autocomplete="loginMode === 'register' ? 'new-password' : 'current-password'" required minlength="8" :placeholder="t('passwordMinLength')" /></label><button class="button primary full" :disabled="busy">{{ loginMode === 'register' ? t('createAndOpenConsole') : t('signInConsole') }} <ChevronRight :size="16" /></button></form><p class="auth-legal">{{ t('agreeText') }} <a href="/terms">{{ t('termsShort') }}</a> {{ t('andConnector') }} <a href="/privacy">{{ t('privacyShort') }}</a>.</p></div></section>
  </main>
  </div>

  <main v-else class="app-shell" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <aside class="sidebar">
        <div class="logo"><span class="brand-mark small"><Bot :size="19" /></span><span>{{ siteSettings.name }}</span></div>
        <nav>
           <div class="nav-group"><p class="nav-label">{{ t('general') }}</p><button v-for="[id, label, Icon] in generalNav" :key="id" :class="{ active: id === 'usage-overview' ? (route.query.view === id || route.params.view === id) : id === 'usage' ? view === id && route.query.view !== 'usage-overview' && route.params.view !== 'usage-overview' : view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
          <div class="nav-group"><p class="nav-label">{{ t('billing') }}</p><button v-for="[id, label, Icon] in billingNav" :key="id" :class="{ active: view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
          <div class="nav-group"><p class="nav-label">{{ t('personal') }}</p><button v-for="[id, label, Icon] in personalNav" :key="id" :class="{ active: view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
          <div v-if="managementNav.length" class="nav-group management-group"><p class="nav-label">{{ t('management') }}</p><button v-for="[id, label, Icon] in managementNav" :key="id" :class="{ active: view === id }" @click="openConsole(id)"><component :is="Icon" :size="17" /><span>{{ label }}</span></button></div>
        </nav>
      <div class="sidebar-footer"><div class="gateway-status"><span class="live-dot"></span><span><b>{{ t('gatewayOnline') }}</b><small>{{ t('serviceRunning') }}</small></span></div><div class="sidebar-account"><i>{{ account?.name?.slice(0, 1) || '?' }}</i><span><b>{{ account?.name || t('loadingLabel') }}</b><small>{{ account?.role || t('accountLabel') }}</small></span><button :aria-label="t('actionAccountLogout')" :title="t('actionAccountLogout')" @click="signOut"><LogOut :size="16" /></button></div></div>
    </aside>
      <section class="content" :data-usage-page="route.query.view === 'usage-overview' || route.params.view === 'usage-overview' ? 'overview' : 'logs'">
        <header class="console-header"><div><p class="eyebrow">{{ managementNav.some((item) => item[0] === view) ? t('management') : personalNav.some((item) => item[0] === view) ? t('personal') : billingNav.some((item) => item[0] === view) ? t('billing') : t('general') }}</p><h1>{{ [...localizedManagementNavItems, ...generalNav, ...billingNav, ...personalNav, ...localizedAdminExtraNav].find((item) => item[0] === view)?.[1] }}</h1></div><div class="header-actions"><button class="theme-toggle sidebar-toggle" :aria-label="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'" :title="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'" @click="sidebarCollapsed = !sidebarCollapsed"><PanelLeftOpen v-if="sidebarCollapsed" :size="16" /><PanelLeftClose v-else :size="16" /></button><a class="button ghost marketplace-link" href="/models"><Sparkles :size="15" />{{ t('marketplace') }}</a><span class="account-chip"><i>{{ account?.name?.slice(0, 1) || '?' }}</i>{{ account?.name || t('loadingLabel') }}</span><ThemeCustomizer :locale="locale" /><select v-model="locale" class="language-select" :aria-label="t('switchLanguage')"><option value="zh-CN">{{ t('chinese') }}</option><option value="en-US">{{ t('english') }}</option></select><button class="button ghost" @click="load" :disabled="busy"><RefreshCw :size="16" :class="{ spinning: busy }" />{{ t('refresh') }}</button></div></header>
        <component :is="currentViewComponent" v-if="currentViewComponent" />
        <ConsoleModals />
      </section>
  </main>
</template>
