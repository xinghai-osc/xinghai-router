<script setup lang="ts">
import { computed, defineAsyncComponent, h, onBeforeUnmount, onMounted, provide, reactive, ref, watch } from 'vue'
import { Activity, Bot, ChevronRight, CircleAlert, Copy, KeyRound, Layers3, LayoutDashboard, LogOut, PanelLeftClose, PanelLeftOpen, RadioTower, RefreshCw, ShieldCheck, Sparkles, TerminalSquare, UserRound, Users, WalletCards, ReceiptText, Tags, Settings, Crown } from 'lucide-vue-next'
import { endpoints, clearToken, getToken, setToken } from '~/src/api'
import ModelSquare from '~/components/marketplace/ModelSquare.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Account, ActivityLog, AdminSiteSettings, AdminSubscription, ApiKey, CatalogGroup, CatalogModel, Channel, Group, LedgerEntry, ModelProvider, PaymentMethod, PaymentOrder, PaymentSettings, Pricing, PublicSubscriptionPlan, ReliabilitySettings, SiteSettings, SubscriptionOrder, SubscriptionPlan, UsageRecord, User, UserSubscription } from '~/src/api'
import type { View } from '~/src/views'
import { VIEWS } from '~/src/views'
import { CONSOLE_STORE_KEY } from '~/composables/useConsoleStore'
const { locale, t, setLocale, toggleLocale, initializeLocale } = useI18n()
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
const siteSettingsForm = reactive<AdminSiteSettings & { geetest_captcha_key: string; smtp_password: string }>({ name: '', icon_url: '', auto_disable_failed_channels: false, geetest_captcha_id: '', has_geetest_captcha_key: false, geetest_captcha_key: '', smtp_host: '', smtp_port: '465', smtp_username: '', has_smtp_password: false, smtp_password: '', smtp_from: '' })
const reliabilityForm = reactive<ReliabilitySettings>({ retry_count: 3, retry_status_codes: '', health_check_mode: 'off', health_check_interval_minutes: 5, health_check_auto_recover: true, health_check_channel_ids: '', auto_disable_on_test_failure: false, auto_disable_slow_seconds: 0, auto_disable_status_codes: '', auto_disable_keywords: '' })
const newAPIPricingForm = reactive({ base_url: '', api_key: '', price_per_quota_unit: 1 })
const loginMode = ref<'token' | 'login' | 'register'>('login')
const accountForm = reactive({ name: '', email: '', password: '' })
const leaderboardPrefs = reactive({ opt_in: true, mask_name: true })
async function saveLeaderboardPrefs() { await action(async () => { await endpoints.updateAccountPreferences(leaderboardPrefs.opt_in, leaderboardPrefs.mask_name); if (account.value) { account.value.leaderboard_opt_in = leaderboardPrefs.opt_in; account.value.leaderboard_mask_name = leaderboardPrefs.mask_name } }) }
// Geetest v4 CAPTCHA — loaded lazily the first time sign-in requires it.
declare global { interface Window { initGeetest4?: (options: Record<string, unknown>, callback: (captcha: GeetestCaptcha) => void) => void } }
interface GeetestCaptcha { onReady(fn: () => void): void; onSuccess(fn: () => void): void; onClose(fn: () => void): void; onError(fn: (cause: unknown) => void): void; showCaptcha(): void; getValidate(): Record<string, string> | null }
let geetestScript: Promise<void> | null = null
let geetestInstance: GeetestCaptcha | null = null
let geetestReady: Promise<void> | null = null
function loadGeetestScript() { geetestScript ??= new Promise<void>((resolve, reject) => { const script = document.createElement('script'); script.src = 'https://static.geetest.com/v4/gt4.js'; script.onload = () => resolve(); script.onerror = () => reject(new Error('captcha script failed')); document.head.appendChild(script) }); return geetestScript }
function ensureGeetest() { if (!siteSettings.value.geetest_enabled || !siteSettings.value.geetest_captcha_id) return Promise.resolve(); geetestReady ??= loadGeetestScript().then(() => new Promise<void>((resolve, reject) => { window.initGeetest4?.({ captchaId: siteSettings.value.geetest_captcha_id, product: 'float', language: locale.value === 'en-US' ? 'eng' : 'zho' }, (captcha) => { geetestInstance = captcha; captcha.onReady(() => resolve()); captcha.onError(reject) }) })); return geetestReady }
function runGeetest(): Promise<Record<string, string>> { return new Promise((resolve, reject) => { const captcha = geetestInstance; if (!captcha) return resolve({}); const cleanup = () => { captcha.onSuccess(() => {}); captcha.onClose(() => {}) }; captcha.onSuccess(() => { const result = captcha.getValidate(); cleanup(); if (result) resolve(result); else reject(new Error('captcha failed')) }); captcha.onClose(() => { cleanup(); reject(new Error('captcha closed')) }); captcha.showCaptcha() }) }
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
    await endpoints.sendEmailCode(accountForm.email.trim(), captcha)
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
async function accountSignIn(register: boolean) { let captcha: Record<string, string> = {}; const emailVerify = register && siteSettings.value.email_verification_enabled; if (siteSettings.value.geetest_enabled && !emailVerify) { try { await ensureGeetest(); captcha = await runGeetest() } catch { return } } await action(async () => { const body = register ? { ...accountForm, ...(emailVerify ? { code: emailCode.value.trim() } : {}), ...captcha } : { email: accountForm.email, password: accountForm.password, ...(emailVerify ? { code: emailCode.value.trim() } : {}), ...captcha }; const result = register ? await endpoints.register(body) : await endpoints.login(body); setToken(result.token); authenticated.value = true; await load(); await router.replace({ path: '/console', query: { view: managementNav.value.length ? 'overview' : 'account' } }) }) }
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
    <div v-if="error" ref="errorAlert" class="fixed left-1/2 top-4 z-50 flex max-w-[min(92vw,720px)] -translate-x-1/2 items-start gap-2 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive shadow-md backdrop-blur" role="alert" tabindex="0" :title="t('clickToCopyError')" @mouseenter="lockError" @mouseleave="releaseError" @click="copyError" @keydown.enter.prevent="copyError" @keydown.space.prevent="copyError">
      <CircleAlert :size="17" class="mt-px shrink-0" /><span class="flex-1 overflow-wrap-anywhere">{{ error }}</span><Copy :size="14" aria-hidden="true" class="mt-0.5 shrink-0" />
    </div>
  </Transition>
  <main v-if="isLanding" class="landing-shell">
    <PublicTopbar :site-name="siteSettings.name" :authenticated="authenticated" />
    <section class="hero-section">
      <div class="hero-copy"><p class="eyebrow">OPENAI-COMPATIBLE MODEL GATEWAY</p><h1>{{ t('heroTagline') }}</h1><p class="hero-description">{{ t('heroDescription') }}</p><div class="hero-actions"><button class="button primary hero-button" @click="openConsoleOrAuth">{{ t('openConsole') }} <ChevronRight :size="16" /></button><a class="text-link" href="#quickstart">{{ t('viewRequestExample') }}</a></div></div>
      <div class="hero-visual"><div class="visual-glow"/><div class="route-card"><div class="route-card-top"><span><i class="live-dot"/>ROUTER ONLINE</span><code>POST /v1/chat/completions</code></div><div class="route-model"><Bot :size="18" /><strong>kimi-k3</strong><span>{{ t('routingActive') }}</span></div><div class="route-line"><span class="route-node active"/><div><b>{{ t('openaiMainRoute') }}</b><small>{{ t('priorityLabel') }} P10 &middot; 42ms</small></div><span class="route-check">✓</span></div><div class="route-line muted-route"><span class="route-node"/><div><b>{{ t('backupChannelText') }}</b><small>{{ t('waitingForFailover') }}</small></div></div><div class="route-footer"><span>{{ t('successRate') }}</span><strong>99.98%</strong><span class="route-divider"/><span>{{ t('avgLatency') }}</span><strong>186ms</strong></div></div></div>
    </section>
    <section id="features" class="feature-section"><div class="section-intro"><p class="eyebrow">BUILT FOR CONTROL</p><h2>{{ t('featuresTagline') }}</h2></div><div class="feature-grid"><article><span class="feature-number">01</span><RadioTower :size="21" /><h3>{{ t('smartRouting') }}</h3><p>{{ t('smartRoutingDesc') }}</p></article><article><span class="feature-number">02</span><Activity :size="21" /><h3>{{ t('fullObservability') }}</h3><p>{{ t('fullObservabilityDesc') }}</p></article><article><span class="feature-number">03</span><WalletCards :size="21" /><h3>{{ t('usageAndCost') }}</h3><p>{{ t('usageAndCostDesc') }}</p></article></div></section>
    <section id="quickstart" class="quickstart-section"><div><p class="eyebrow">ONE ENDPOINT</p><h2>{{ t('quickstartTagline') }}</h2><p>{{ t('quickstartDesc') }}</p></div><pre><span class="code-comment">// Using OpenAI SDK</span><span><b>const</b> client = <b>new</b> OpenAI({</span><span>  apiKey: <i>'sk-xh-your-key'</i>,</span><span>  baseURL: <i>'http://localhost:8080/v1'</i></span><span>})</span><span class="code-gap"/><span><b>await</b> client.chat.completions.create({</span><span>  model: <i>'kimi-k3'</i>,</span><span>  messages: [{ role: <i>'user'</i>, content: <i>'你好'</i> }]</span><span>})</span></pre></section>
     <footer class="landing-footer"><span>© 2026 Xinghai Router</span><span>{{ t('footerSlogan') }}</span><span class="legal-links"><a href="/terms">{{ t('termsShort') }}</a><a href="/privacy">{{ t('privacyShort') }}</a></span></footer>
  </main>

  <main v-else-if="isMarketplacePage" class="public-marketplace">
       <PublicTopbar :site-name="siteSettings.name" :authenticated="authenticated" />
    <ModelSquare :catalog="catalog" :groups="catalogGroups" :loaded="catalogLoaded" />
  </main>

      <div v-else-if="isAuthPage" class="public-wrap">
      <PublicTopbar :site-name="siteSettings.name" :authenticated="authenticated" />
      <main class="login-shell">
      <section class="login-card"><aside class="login-aside"><div class="login-aside-glow"/><div class="login-aside-inner"><span class="brand-mark"><Bot :size="28" /></span><h2>{{ t('loginBrandTitle') }}</h2><p>{{ t('loginBrandDesc') }}</p><ul><li><i><RadioTower :size="14" /></i><span>{{ t('brandPoint1') }}</span></li><li><i><Tags :size="14" /></i><span>{{ t('brandPoint2') }}</span></li><li><i><Activity :size="14" /></i><span>{{ t('brandPoint3') }}</span></li><li><i><ShieldCheck :size="14" /></i><span>{{ t('brandPoint4') }}</span></li></ul></div></aside><div class="login-pane"><h1>{{ loginMode === 'register' ? t('createAccountTab') : t('signInTab') }}</h1><p class="login-sub">{{ loginMode === 'register' ? t('registerSub') : t('loginSub') }}</p><div class="flex gap-1 rounded-md bg-muted p-1"><button class="flex-1 rounded px-3 py-1.5 text-sm font-medium transition-colors" :class="loginMode === 'login' ? 'bg-background shadow-sm' : 'text-muted-foreground'" @click="loginMode = 'login'">{{ t('signInTab') }}</button><button class="flex-1 rounded px-3 py-1.5 text-sm font-medium transition-colors" :class="loginMode === 'register' ? 'bg-background shadow-sm' : 'text-muted-foreground'" @click="loginMode = 'register'">{{ t('createAccountTab') }}</button></div><form class="flex flex-col gap-3" @submit.prevent="accountSignIn(loginMode === 'register')"><div v-if="loginMode === 'register'" class="flex flex-col gap-1.5"><Label class="text-xs">{{ t('nameLabel') }}</Label><Input v-model="accountForm.name" autocomplete="name" required maxlength="100" :placeholder="t('namePlaceholder')" /></div><div class="flex flex-col gap-1.5"><Label class="text-xs">{{ t('emailLabel') }}</Label><Input v-model="accountForm.email" type="email" autocomplete="email" required placeholder="name@example.com" /></div><div v-if="loginMode === 'register' && siteSettings.email_verification_enabled" class="flex flex-col gap-1.5"><Label class="text-xs">{{ t('emailCodeLabel') }}</Label><div class="flex gap-2"><Input v-model="emailCode" inputmode="numeric" autocomplete="one-time-code" maxlength="6" required :placeholder="t('emailCodePlaceholder')" class="flex-1" /><Button variant="outline" type="button" :disabled="!emailLooksValid || codeSending || codeCountdown > 0" @click="sendEmailCode">{{ codeCountdown > 0 ? t('codeCountdown').replace('{n}', String(codeCountdown)) : codeSending ? t('codeSending') : t('sendCode') }}</Button></div><small v-if="codeSentHint" class="text-xs text-muted-foreground">{{ codeSentHint }}</small></div><div class="flex flex-col gap-1.5"><Label class="text-xs">{{ t('passwordLabel') }}</Label><Input v-model="accountForm.password" type="password" :autocomplete="loginMode === 'register' ? 'new-password' : 'current-password'" required minlength="8" :placeholder="t('passwordMinLength')" /></div><Button type="submit" class="mt-1 w-full" :disabled="busy">{{ loginMode === 'register' ? t('createAndOpenConsole') : t('signInConsole') }} <ChevronRight :size="16" /></Button></form><p class="auth-legal">{{ t('agreeText') }} <a href="/terms">{{ t('termsShort') }}</a> {{ t('andConnector') }} <a href="/privacy">{{ t('privacyShort') }}</a>.</p></div></section>
  </main>
  </div>

  <main v-else class="flex min-h-screen bg-background text-foreground" :class="{ 'lg:[&>.app-sidebar]:w-16': sidebarCollapsed }">
    <aside class="app-sidebar sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r border-border bg-card/50 transition-[width] lg:w-60">
        <div class="flex h-16 items-center gap-3 px-5"><span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm"><Bot :size="20" /></span><span v-show="!sidebarCollapsed" class="min-w-0 truncate text-[15px] font-bold tracking-tight">{{ siteSettings.name }}</span></div>
        <nav class="flex-1 overflow-y-auto px-3 pt-2 pb-3">
           <div class="mb-4"><p class="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground/70">{{ t('general') }}</p><button v-for="[id, label, Icon] in generalNav" :key="id" class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-[13px] leading-snug transition-all duration-150 hover:bg-accent" :class="(id === 'usage-overview' ? (route.query.view === id || route.params.view === id) : id === 'usage' ? view === id && route.query.view !== 'usage-overview' && route.params.view !== 'usage-overview' : view === id) ? 'bg-accent font-semibold text-accent-foreground shadow-sm' : 'text-muted-foreground'" @click="openConsole(id)"><component :is="Icon" :size="16" class="shrink-0" /><span v-show="!sidebarCollapsed" class="truncate">{{ label }}</span></button></div>
          <div class="mb-4"><p class="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground/70">{{ t('billing') }}</p><button v-for="[id, label, Icon] in billingNav" :key="id" class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-[13px] leading-snug transition-all duration-150 hover:bg-accent" :class="view === id ? 'bg-accent font-semibold text-accent-foreground shadow-sm' : 'text-muted-foreground'" @click="openConsole(id)"><component :is="Icon" :size="16" class="shrink-0" /><span v-show="!sidebarCollapsed" class="truncate">{{ label }}</span></button></div>
          <div class="mb-4"><p class="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground/70">{{ t('personal') }}</p><button v-for="[id, label, Icon] in personalNav" :key="id" class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-[13px] leading-snug transition-all duration-150 hover:bg-accent" :class="view === id ? 'bg-accent font-semibold text-accent-foreground shadow-sm' : 'text-muted-foreground'" @click="openConsole(id)"><component :is="Icon" :size="16" class="shrink-0" /><span v-show="!sidebarCollapsed" class="truncate">{{ label }}</span></button></div>
          <div v-if="managementNav.length" class="mb-4"><p class="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground/70">{{ t('management') }}</p><button v-for="[id, label, Icon] in managementNav" :key="id" class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-[13px] leading-snug transition-all duration-150 hover:bg-accent" :class="view === id ? 'bg-accent font-semibold text-accent-foreground shadow-sm' : 'text-muted-foreground'" @click="openConsole(id)"><component :is="Icon" :size="16" class="shrink-0" /><span v-show="!sidebarCollapsed" class="truncate">{{ label }}</span></button></div>
        </nav>
      <div class="border-t border-border px-4 py-3.5"><div class="mb-3 flex items-center gap-2.5 text-xs text-muted-foreground"><span class="h-2 w-2 shrink-0 animate-pulse rounded-full bg-green-500" /><span class="min-w-0"><b class="block text-foreground/90">{{ t('gatewayOnline') }}</b><small class="text-[11px]">{{ t('serviceRunning') }}</small></span></div><div class="flex items-center gap-2.5"><i class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary text-xs font-bold text-primary-foreground shadow-sm">{{ account?.name?.slice(0, 1) || '?' }}</i><span v-show="!sidebarCollapsed" class="min-w-0 flex-1"><b class="block truncate text-[13px] font-semibold text-foreground">{{ account?.name || t('loadingLabel') }}</b><small class="text-[11px] text-muted-foreground">{{ account?.role || t('accountLabel') }}</small></span><Button v-show="!sidebarCollapsed" variant="ghost" size="icon-sm" :aria-label="t('actionAccountLogout')" :title="t('actionAccountLogout')" @click="signOut"><LogOut :size="15" /></Button></div></div>
    </aside>
      <section class="flex flex-1 flex-col overflow-hidden" :data-usage-page="route.query.view === 'usage-overview' || route.params.view === 'usage-overview' ? 'overview' : 'logs'">
        <header class="sticky top-0 z-10 flex h-14 items-center justify-between gap-3 border-b border-border bg-background/80 px-4 backdrop-blur"><div class="min-w-0"><p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{{ managementNav.some((item) => item[0] === view) ? t('management') : personalNav.some((item) => item[0] === view) ? t('personal') : billingNav.some((item) => item[0] === view) ? t('billing') : t('general') }}</p><h1 class="truncate text-lg font-semibold">{{ [...localizedManagementNavItems, ...generalNav, ...billingNav, ...personalNav, ...localizedAdminExtraNav].find((item) => item[0] === view)?.[1] }}</h1></div><div class="flex items-center gap-2"><Button variant="ghost" size="icon-sm" class="hidden lg:inline-flex" :aria-label="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'" :title="sidebarCollapsed ? '展开侧边栏' : '收起侧边栏'" @click="sidebarCollapsed = !sidebarCollapsed"><PanelLeftOpen v-if="sidebarCollapsed" :size="16" /><PanelLeftClose v-else :size="16" /></Button><Button variant="ghost" size="sm" as-child><a href="/models"><Sparkles :size="15" />{{ t('marketplace') }}</a></Button><span class="hidden items-center gap-2 rounded-full border border-border px-3 py-1 text-sm font-medium sm:inline-flex"><i class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-[10px] font-bold text-primary-foreground">{{ account?.name?.slice(0, 1) || '?' }}</i>{{ account?.name || t('loadingLabel') }}</span><ThemeCustomizer :locale="locale" /><select v-model="locale" class="h-9 rounded-md border border-input bg-transparent px-2 text-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring" :aria-label="t('switchLanguage')"><option value="zh-CN">{{ t('chinese') }}</option><option value="en-US">{{ t('english') }}</option></select><Button variant="ghost" size="sm" :disabled="busy" @click="load"><RefreshCw :size="16" :class="{ 'animate-spin': busy }" />{{ t('refresh') }}</Button></div></header>
        <div class="flex-1 overflow-y-auto p-4">
          <component :is="currentViewComponent" v-if="currentViewComponent" />
          <ConsoleModals />
        </div>
      </section>
  </main>
</template>
