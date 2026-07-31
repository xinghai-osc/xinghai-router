export interface User { id: string; email: string; name: string; role: string; enabled: boolean; balance: number; reserved: number; permissions: string[]; groups: string[]; created_at: string }
export interface ApiKey { id: string; user_id: string; name: string; key_prefix: string; group_id: string; group_name: string; expires_at: string | null; revoked_at: string | null; last_used_at: string | null; created_at: string }
/** `groups` holds group ids, not names — resolve them through /admin/groups. */
export interface Channel { id: string; name: string; base_url: string; provider: 'openai' | 'ollama' | 'kimi' | 'opencode_go' | 'anthropic'; models: string[]; enabled: boolean; auto_disabled: boolean; disabled_reason: string; priority: number; groups: string[]; key_type: 'single' | 'multi'; key_count: number; created_at: string; updated_at: string; model_routes: ModelRoute[] }

export interface ChannelKey { id: string; name: string; enabled: boolean; last_checked_at: string | null; last_error: string | null; created_at: string }
export interface ChannelKeyForm { name?: string; api_key: string }
export interface ChannelKeyTestResult { success: boolean; status_code: number; latency_ms: number; reason?: string; auto_disabled: boolean }

export interface ModelRoute { id: string; public_model: string; upstream_model: string; channel_id?: string; priority: number; weight: number; enabled: boolean; hidden: boolean; created_at: string }
export interface ModelRouteForm { public_model: string; upstream_model: string; priority?: number; weight?: number; hidden?: boolean }
export interface Group { id: string; name: string; multiplier: number; max_concurrency: number | null; public: boolean; created_at: string }
export interface RequestLog { request_id: string; user_id: string | null; api_key_id: string | null; channel_id: string | null; model: string; status_code: number; prompt_tokens: number | null; completion_tokens: number | null; total_tokens: number | null; duration_ms: number; error_code: string | null; created_at: string }
export interface Account { id: string; email: string; name: string; role: string; avatar_url: string; permissions: string[]; balance: number; reserved: number; leaderboard_opt_in: boolean; leaderboard_mask_name: boolean; must_change_password?: boolean }
export interface Pricing { id: string; model: string; input_per_million: number; cached_input_per_million: number; output_per_million: number; multiplier: number; enabled: boolean; updated_at: string }
export interface CatalogGroup { id: string; name: string; multiplier: number; public: boolean }
export interface CatalogModel { id: string; model: string; provider: string; provider_slug: string; input_per_million: number | null; cached_input_per_million: number | null; output_per_million: number | null; multiplier: number | null; groups: CatalogGroup[] }
export interface ModelProvider { id: string; name: string; slug: string; prefixes: string[]; priority: number }
export interface UsageRecord { request_id: string; model: string; prompt_tokens: number; cached_prompt_tokens: number; completion_tokens: number; cost: string; status: string; created_at: string }
export interface ActivityLog { id: string; type: 'request' | 'login' | 'register' | 'logout' | 'topup' | 'operation'; action: string; user_id: string; user_name: string; model: string; group_id: string; group_name: string; status_code: number | null; duration_ms: number | null; prompt_tokens: number; completion_tokens: number; total_tokens: number; cost: number; details: Record<string, unknown>; created_at: string }
export interface LedgerEntry { id: string; amount: string; balance_after: string; kind: string; request_id: string | null; note: string | null; created_at: string }
export interface PaymentOrder { order_no: string; payment_type: string; amount: string; status: 'pending' | 'paid' | 'failed' | 'expired'; provider_trade_no?: string; paid_at: string | null; created_at: string }
export interface PaymentMethod { id: string; code: string; name: string; enabled: boolean; created_at: string | null }
export interface PaymentSettings { enabled: boolean; base_url: string; merchant_id: string; has_merchant_key: boolean; public_base_url: string; methods: PaymentMethod[] }
export interface ModelRanking { rank: number; previous_rank?: number; model_name: string; vendor: string; total_tokens: number; share: number; growth_pct: number }
export interface VendorRanking { rank: number; vendor: string; total_tokens: number; share: number; growth_pct: number; models_count: number; top_model: string }
export interface RankingMover { model_name: string; vendor: string; rank_delta: number; current_rank: number; growth_pct: number }
export interface UserRanking { rank: number; name: string; total_tokens: number; total_cost: number; share: number; growth_pct: number; requests: number; top_model: string }
export interface Rankings { period: string; models: ModelRanking[]; vendors: VendorRanking[]; top_movers: RankingMover[]; top_droppers: RankingMover[]; users: UserRanking[]; total_tokens: number; updated_at: string }
export interface SiteSettings { name: string; icon_url: string; auto_disable_failed_channels: boolean; geetest_enabled?: boolean; geetest_captcha_id?: string; email_verification_enabled?: boolean; oauth_providers?: string[] }
export interface AdminSiteSettings { name: string; icon_url: string; auto_disable_failed_channels: boolean; geetest_captcha_id: string; has_geetest_captcha_key: boolean; smtp_host: string; smtp_port: string; smtp_username: string; has_smtp_password: boolean; smtp_from: string }
export interface ReliabilitySettings { retry_count: number; retry_status_codes: string; health_check_mode: 'off' | 'scheduled_all' | 'passive_recovery'; health_check_interval_minutes: number; health_check_auto_recover: boolean; health_check_channel_ids: string; auto_disable_on_test_failure: boolean; auto_disable_slow_seconds: number; auto_disable_status_codes: string; auto_disable_keywords: string }

export interface PaymentRedirect { order_no: string; amount: string; status: 'pending'; pay_url: string }

export interface AccountGroups { data: string[]; groups: Group[]; user_groups: string[]; user_group: string }

export interface UsageLog {
  request_id: string
  user_id: string | null
  user_name: string
  api_key_id: string | null
  channel_id: string | null
  channel_name: string
  group_id: string
  group_name: string
  model: string
  status_code: number
  prompt_tokens: number | null
  completion_tokens: number | null
  total_tokens: number | null
  duration_ms: number
  error_code: string | null
  cost: string
  created_at: string
}

export interface UsageStats {
  total_requests: number
  success_count: number
  error_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost: string
  avg_duration_ms: number
  breakdown?: UsageStatBreakdown[]
}

export interface UsageStatBreakdown {
  period: string
  requests: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cost: string
}

export interface SubscriptionPlan {
  id: string
  name: string
  description: string
  price: string
  currency: string
  billing_period: 'month' | 'year'
  credit_amount: string
  group_id: string
  group_name: string
  model_whitelist: string[]
  max_requests_per_period: number | null
  max_tokens_per_period: number | null
  sort_order: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface PublicSubscriptionPlan {
  id: string
  name: string
  description: string
  price: string
  currency: string
  billing_period: 'month' | 'year'
  credit_amount: string
  group_name: string
  model_whitelist: string[]
  sort_order: number
}

export interface UserSubscription {
  id: string
  user_id: string
  plan_id: string
  plan_name: string
  status: 'pending' | 'active' | 'expired' | 'cancelled'
  current_period_start: string | null
  current_period_end: string | null
  auto_renew: boolean
  cancelled_at: string | null
  created_at: string
  updated_at: string
  price: string
  billing_period: 'month' | 'year'
  credit_amount: string
  group_id: string
  group_name: string
  model_whitelist: string[]
  max_requests_per_period: number | null
  max_tokens_per_period: number | null
}

export interface SubscriptionOrder {
  id: string
  order_no: string
  subscription_id: string
  plan_id: string
  plan_name: string
  provider: string
  payment_type: string
  amount: string
  status: 'pending' | 'paid' | 'failed' | 'expired'
  provider_trade_no?: string
  period_kind: 'new' | 'renewal'
  paid_at: string | null
  created_at: string
}

export interface AdminSubscription {
  id: string
  user_id: string
  email: string
  user_name: string
  plan_id: string
  plan_name: string
  status: 'pending' | 'active' | 'expired' | 'cancelled'
  current_period_start: string | null
  current_period_end: string | null
  auto_renew: boolean
  cancelled_at: string | null
  created_at: string
  updated_at: string
}

export interface PublicActivityItem {
  model: string
  status_code: number
  duration_ms: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  created_at: string
}

export interface AuditLog {
  id: string
  action: string
  actor: string
  entity_type: string
  entity_id: string
  details: Record<string, unknown> | null
  client_ip: string
  forwarded_for: string
  user_agent: string
  browser: string
  browser_version: string
  operating_system: string
  operating_system_version: string
  device_type: string
  is_bot: boolean
  request_method: string
  request_path: string
  request_id: string
  created_at: string
}

export interface OAuthProvider {
  id: string
  client_id: string
  enabled: boolean
  has_client_secret: boolean
  created_at: string
  updated_at: string
}

export interface OAuthProviderForm {
  client_id: string
  client_secret: string
  enabled: boolean
}

export interface OAuthConnection {
  provider: string
  provider_username: string
  provider_avatar_url: string
  created_at: string
}

export interface MigrationRequest {
  id: string
  status: 'idle' | 'running' | 'completed' | 'failed'
  source_driver: string
  step: string
  current: number
  total: number
  detail: string
  error: string
  started_at: string | null
  finished_at: string | null
  created_at: string
}

const TOKEN_COOKIE = 'xinghai.admin-token'
const TOKEN_MAX_AGE = 60 * 60 * 24 * 7

function readCookie(name: string): string {
  if (typeof document === 'undefined') return ''
  const prefix = `${name}=`
  const match = document.cookie.split('; ').find((entry) => entry.startsWith(prefix))
  return match ? decodeURIComponent(match.slice(prefix.length)) : ''
}

function writeCookie(name: string, value: string, maxAge: number): void {
  if (typeof document === 'undefined') return
  const encoded = encodeURIComponent(value.trim())
  document.cookie = `${name}=${encoded}; path=/; max-age=${maxAge}; samesite=strict`
}

function deleteCookie(name: string): void {
  if (typeof document === 'undefined') return
  document.cookie = `${name}=; path=/; max-age=0; samesite=strict`
}

let token = import.meta.client ? readCookie(TOKEN_COOKIE) : ''
export const getToken = () => token
export const setToken = (value: string) => { token = value.trim(); writeCookie(TOKEN_COOKIE, token, TOKEN_MAX_AGE) }
export const clearToken = () => { token = ''; deleteCookie(TOKEN_COOKIE) }

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api${path}`, { ...init, headers: { Authorization: `Bearer ${token}`, ...(init.body ? { 'Content-Type': 'application/json' } : {}), ...init.headers } })
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new Error(body?.error?.message ?? `请求失败 (${response.status})`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

async function get<T>(path: string): Promise<T> { return api<T>(path) }
async function post<T>(path: string, body?: unknown): Promise<T> { return api<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) }) }
async function put<T>(path: string, body?: unknown): Promise<T> { return api<T>(path, { method: 'PUT', body: body === undefined ? undefined : JSON.stringify(body) }) }
async function send(path: string, method: 'POST' | 'PUT' | 'DELETE', body?: unknown): Promise<void> { await api<unknown>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) }) }

export interface LoginBody { email: string; password: string; code?: string; lot_number?: string; captcha_output?: string; pass_token?: string; gen_time?: string }
export interface RegisterBody { name: string; email: string; password: string; code?: string; lot_number?: string; captcha_output?: string; pass_token?: string; gen_time?: string }
export interface KeyForm { user_id?: string; name: string; expires_at: string; group_id: string }
export interface AccountKeyForm { name: string; expires_at: string; group_id: string }
export interface ChannelForm { name: string; provider: string; base_url: string; key_type: 'single' | 'multi'; api_keys: string; models: string[]; priority: number; groups: string[]; model_routes?: ModelRouteForm[] }
export interface ProviderForm { name: string; slug: string; prefixes: string[]; priority: number; id?: string }
export interface PaymentSettingsForm { enabled: boolean; base_url: string; merchant_id: string; merchant_key: string; public_base_url: string }
export interface PaymentMethodForm { code: string; name: string; enabled: boolean }
export interface PricingForm { model: string; input_per_million: number; cached_input_per_million: number; output_per_million: number; multiplier: number }
export interface NewApiPricingForm { base_url: string; api_key: string; price_per_quota_unit: number }
export interface SubscriptionPlanForm { name: string; description: string; price: string; currency: string; billing_period: string; credit_amount: string; group_id: string; model_whitelist: string[]; max_requests_per_period: number | null; max_tokens_per_period: number | null; sort_order: number; enabled: boolean }
export interface UserUpdate { name?: string; email?: string; role?: string; enabled?: boolean; password?: string; balance?: number | null; note?: string; permissions?: string[]; groups?: string[] }
export interface MigrateForm { source_dsn: string; source_driver: string }
export interface MigrateResult { message: string }
/**
 * `status` is the Go zero value ('') until the first migration runs, and the
 * timestamps are always present as the Go zero time ('0001-01-01T00:00:00Z')
 * rather than omitted — `omitempty` has no effect on time.Time.
 */
export interface MigrationStatus {
  id: string
  status: '' | 'idle' | 'running' | 'completed' | 'failed'
  step: string
  current: number
  total: number
  detail?: string
  error?: string
  started_at: string
  finished_at: string
}

/**
 * PUT /admin/site-settings rejects unknown fields, so the read-only `has_*`
 * indicators must not be echoed back in the update body.
 */
export type SiteSettingsForm =
  Omit<AdminSiteSettings, 'has_geetest_captcha_key' | 'has_smtp_password'>
  & { geetest_captcha_key: string; smtp_password: string }

export const endpoints = {
  getSiteSettings: () => get<SiteSettings>('/site-settings'),
  getAccount: () => get<Account>('/account/me'),
  getAccountKeys: () => get<{ data: ApiKey[] }>('/account/keys'),
  getAccountUsage: () => get<{ data: UsageRecord[] }>('/account/usage'),
  getAccountLedger: () => get<{ data: LedgerEntry[] }>('/account/ledger'),
  getAccountGroups: () => get<AccountGroups>('/account/groups'),
  getAccountPayments: () => get<{ enabled: boolean; payment_methods: PaymentMethod[]; data: PaymentOrder[] }>('/account/payments'),
  getAccountPayment: (orderNo: string) => get<PaymentOrder>(`/account/payments/${encodeURIComponent(orderNo)}`),
  createAccountPayment: (amount: string, type: string) => post<PaymentRedirect>('/account/payments', { amount, type }),
  getAccountSubscriptions: () => get<{ data: UserSubscription[] }>('/account/subscriptions'),
  createAccountSubscription: (planId: string, paymentType: string, autoRenew: boolean) => post<PaymentRedirect>('/account/subscriptions', { plan_id: planId, payment_type: paymentType, auto_renew: autoRenew }),
  cancelAccountSubscription: (id: string) => send(`/account/subscriptions/${encodeURIComponent(id)}/cancel`, 'POST'),
  getAccountSubscriptionOrders: () => get<{ data: SubscriptionOrder[] }>('/account/subscription-orders'),
  getAccountSubscriptionOrder: (orderNo: string) => get<SubscriptionOrder>(`/account/subscription-orders/${encodeURIComponent(orderNo)}`),
  updateAccountProfile: (avatarUrl: string) => send('/account/profile', 'PUT', { avatar_url: avatarUrl }),
  changeAccountPassword: (currentPassword: string, newPassword: string) => send('/account/password', 'PUT', { current_password: currentPassword, new_password: newPassword }),
  revokeAccountKey: (id: string) => send(`/account/keys/${encodeURIComponent(id)}/revoke`, 'POST'),
  updateAccountPreferences: (leaderboardOptIn: boolean, leaderboardMaskName: boolean) => send('/account/preferences', 'PUT', { leaderboard_opt_in: leaderboardOptIn, leaderboard_mask_name: leaderboardMaskName }),
  createAccountKey: (form: AccountKeyForm) => post<{ key: string }>('/account/keys', form),
  updateAccountKey: (id: string, form: AccountKeyForm) => send(`/account/keys/${encodeURIComponent(id)}`, 'PUT', form),

  getActivityLogs: (query = '') => get<{ data: ActivityLog[] }>(`/activity-logs${query}`),
  getModelCatalog: () => get<{ data: CatalogModel[]; groups: CatalogGroup[] }>('/model-catalog'),
  getPublicSubscriptionPlans: () => get<{ data: PublicSubscriptionPlan[] }>('/subscription-plans'),

  login: (body: LoginBody) => post<{ token: string }>('/auth/login', body),
  register: (body: RegisterBody) => post<{ token: string }>('/auth/register', body),
  logout: () => send('/auth/logout', 'POST'),
  sendEmailCode: (email: string, captcha?: Record<string, string>) => send('/auth/email-code', 'POST', { email, ...captcha }),

  getAdminUsers: () => get<{ data: User[] }>('/admin/users'),
  updateUser: (id: string, update: UserUpdate) => send(`/admin/users/${encodeURIComponent(id)}`, 'PUT', update),
  getAdminGroups: () => get<{ data: Group[] }>('/admin/groups'),
  createGroup: (name: string, multiplier: number, maxConcurrency: number | null, publicGroup: boolean) => send('/admin/groups', 'POST', { name, multiplier, max_concurrency: maxConcurrency, public: publicGroup }),
  updateGroup: (id: string, multiplier: number, maxConcurrency: number | null, publicGroup: boolean) => send(`/admin/groups/${encodeURIComponent(id)}`, 'PUT', { multiplier, max_concurrency: maxConcurrency, public: publicGroup }),
  importGroups: (entries: Record<string, number>) => send('/admin/groups/import', 'POST', entries),
  getAdminKeys: () => get<{ data: ApiKey[] }>('/admin/keys'),
  createKey: (form: KeyForm) => post<{ key: string }>('/admin/keys', form),
  revokeKey: (id: string) => send(`/admin/keys/${encodeURIComponent(id)}/revoke`, 'POST'),
  getAdminChannels: () => get<{ data: Channel[] }>('/admin/channels'),
  batchToggleChannels: (ids: string[], enabled: boolean) => post<{ affected: number }>('/admin/channels/batch-status', { ids, enabled }),
  createChannel: (form: ChannelForm) => send('/admin/channels', 'POST', form),
  fetchChannelModels: (baseUrl: string, apiKey: string) => post<{ models: string[] }>('/admin/channels/models', { base_url: baseUrl, api_key: apiKey }),
  updateChannel: (id: string, form: ChannelForm) => send(`/admin/channels/${encodeURIComponent(id)}`, 'PUT', form),
  updateChannelGroups: (id: string, groups: string[]) => send(`/admin/channels/${encodeURIComponent(id)}/groups`, 'PUT', { groups }),
  toggleChannel: (id: string, enabled: boolean) => send(`/admin/channels/${encodeURIComponent(id)}/status`, 'POST', { enabled }),
  getChannelKeys: (id: string) => get<{ data: ChannelKey[] }>(`/admin/channels/${encodeURIComponent(id)}/keys`),
  createChannelKey: (id: string, form: ChannelKeyForm) => post<{ id: string }>(`/admin/channels/${encodeURIComponent(id)}/keys`, form),
  deleteChannelKey: (channelId: string, keyId: string) => send(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}`, 'DELETE'),
  toggleChannelKey: (channelId: string, keyId: string, enabled: boolean) => send(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}/status`, 'POST', { enabled }),
  testChannelKey: (channelId: string, keyId: string) => post<ChannelKeyTestResult>(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}/test`),
  migrateChannelKeys: (id: string) => post<{ migrated: boolean }>(`/admin/channels/${encodeURIComponent(id)}/keys/migrate`),
  getAdminProviders: () => get<{ data: ModelProvider[] }>('/admin/providers'),
  saveProvider: (form: ProviderForm) => send('/admin/providers', 'POST', form),
  removeProvider: (id: string) => send(`/admin/providers/${encodeURIComponent(id)}`, 'DELETE'),
  getAdminPricing: () => get<{ data: Pricing[] }>('/admin/pricing'),
  savePricing: (form: PricingForm) => send('/admin/pricing', 'POST', form),
  syncNewApiPricing: (form: NewApiPricingForm) => post<{ synced: number; skipped: number }>('/admin/pricing/newapi/sync', form),
  getAdminReliabilitySettings: () => get<ReliabilitySettings>('/admin/reliability-settings'),
  updateReliabilitySettings: (form: ReliabilitySettings) => put<ReliabilitySettings>('/admin/reliability-settings', form),
  getAdminSiteSettings: () => get<AdminSiteSettings>('/admin/site-settings'),
  updateAdminSiteSettings: (form: SiteSettingsForm) => put<AdminSiteSettings>('/admin/site-settings', form),
  getAdminPaymentSettings: () => get<PaymentSettings>('/admin/payment-settings'),
  updateAdminPaymentSettings: (form: PaymentSettingsForm) => put<PaymentSettings>('/admin/payment-settings', form),
  createPaymentMethod: (form: PaymentMethodForm) => send('/admin/payment-methods', 'POST', form),
  updatePaymentMethod: (id: string, form: PaymentMethodForm) => send(`/admin/payment-methods/${encodeURIComponent(id)}`, 'PUT', form),
  deletePaymentMethod: (id: string) => send(`/admin/payment-methods/${encodeURIComponent(id)}`, 'DELETE'),
  getAdminSubscriptionPlans: () => get<{ data: SubscriptionPlan[] }>('/admin/subscription-plans'),
  createSubscriptionPlan: (form: SubscriptionPlanForm) => send('/admin/subscription-plans', 'POST', form),
  updateSubscriptionPlan: (id: string, form: SubscriptionPlanForm) => send(`/admin/subscription-plans/${encodeURIComponent(id)}`, 'PUT', form),
  deleteSubscriptionPlan: (id: string) => send(`/admin/subscription-plans/${encodeURIComponent(id)}`, 'DELETE'),
  getAdminSubscriptions: () => get<{ data: AdminSubscription[] }>('/admin/subscriptions'),
  batchExtendSubscriptions: (planId: string, days: number, status: 'active' | 'inactive' | 'all') => post<{ affected: number }>('/admin/subscriptions/extend', { plan_id: planId, days, status }),
  runMigration: (form: MigrateForm) => post<MigrateResult>('/admin/migrate', form),
  getMigrationStatus: () => get<MigrationStatus>('/admin/migrate'),

  getUsageLogs: (query = '') => get<{ data: UsageLog[]; total: number; page: number; page_size: number }>(`/admin/usage-logs${query}`),
  getUsageStats: (query = '') => get<UsageStats>(`/admin/usage-stats${query}`),

  getRequestLogs: () => get<{ data: RequestLog[] }>('/admin/request-logs'),
  getAuditLogs: () => get<{ data: AuditLog[] }>('/admin/audit-logs'),
  getModelRoutes: () => get<{ data: ModelRoute[] }>('/admin/model-routes'),
  createModelRoute: (form: Record<string, unknown>) => send('/admin/model-routes', 'POST', form),
  updateModelRoute: (id: string, form: Record<string, unknown>) => send(`/admin/model-routes/${encodeURIComponent(id)}`, 'PUT', form),
  getChannelRoutes: (channelId: string) => get<{ data: ModelRoute[] }>(`/admin/channels/${encodeURIComponent(channelId)}/routes`),
  createChannelRoute: (channelId: string, form: ModelRouteForm) => post<{ id: string }>(`/admin/channels/${encodeURIComponent(channelId)}/routes`, form),
  updateChannelRoute: (channelId: string, routeId: string, form: Record<string, unknown>) => send(`/admin/channels/${encodeURIComponent(channelId)}/routes/${encodeURIComponent(routeId)}`, 'PUT', form),
  deleteChannelRoute: (channelId: string, routeId: string) => send(`/admin/channels/${encodeURIComponent(channelId)}/routes/${encodeURIComponent(routeId)}`, 'DELETE'),
  getGroupNames: () => get<{ data: string[] }>('/group'),
  setAccountKeyGroup: (id: string, groupId: string) => send(`/account/keys/${encodeURIComponent(id)}/group`, 'PUT', { group_id: groupId }),
  setKeyGroup: (id: string, groupId: string) => send(`/admin/keys/${encodeURIComponent(id)}/group`, 'PUT', { group_id: groupId }),
  setUserRole: (id: string, role: string) => send(`/admin/users/${encodeURIComponent(id)}/role`, 'POST', { role }),
  setUserPermissions: (id: string, permissions: string[]) => send(`/admin/users/${encodeURIComponent(id)}/permissions`, 'PUT', { permissions }),
  setUserGroups: (id: string, groups: string[]) => send(`/admin/users/${encodeURIComponent(id)}/groups`, 'PUT', { groups }),
  adjustBalance: (userId: string, amount: number, note: string) => send('/admin/wallets/adjustments', 'POST', { user_id: userId, amount, note }),
  getMigrationRequests: () => get<{ data: MigrationRequest[] }>('/admin/migrate/requests'),

  getOAuthProviders: () => get<{ data: OAuthProvider[] }>('/admin/oauth/providers'),
  saveOAuthProvider: (provider: string, form: OAuthProviderForm) => send(`/admin/oauth/providers/${encodeURIComponent(provider)}`, 'POST', form),
  deleteOAuthProvider: (provider: string) => send(`/admin/oauth/providers/${encodeURIComponent(provider)}`, 'DELETE'),

  getOAuthConnections: () => get<{ data: OAuthConnection[] }>('/account/oauth/connections'),
  unlinkOAuthConnection: (provider: string) => send(`/account/oauth/${encodeURIComponent(provider)}/unlink`, 'POST'),
}
