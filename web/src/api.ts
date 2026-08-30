export interface User { id: string; email: string; name: string; role: string; enabled: boolean; leaderboard_opt_in: boolean; leaderboard_mask_name: boolean; data_usage_enabled: boolean; max_concurrency: number | null; balance: number; reserved: number; permissions: string[]; groups: string[]; inviter_id: string | null; inviter_email: string | null; inviter_name: string | null; created_at: string }
export interface Page<T> { data: T[]; total: number; page: number; page_size: number }
export interface ApiKey { id: string; user_id: string; name: string; key_prefix: string; group_id: string; group_name: string; expires_at: string | null; revoked_at: string | null; last_used_at: string | null; created_at: string; revealable: boolean }
export interface KeyQuotaLimit { id: string; window: 'day' | 'month' | 'total'; max_requests: number | null; max_tokens: number | null; max_cost: number | null; created_at: string }
export interface KeyQuotaUsage { window: string; requests: number; tokens: number; cost: number }
export interface KeyQuota { limits: KeyQuotaLimit[]; usage: KeyQuotaUsage[] }
export interface KeyQuotaForm { window: 'day' | 'month' | 'total'; max_requests?: number | null; max_tokens?: number | null; max_cost?: number | null }
/** `groups` holds group ids, not names — resolve them through /admin/groups. */
export interface RequestOverrides { delete: string[]; set: Record<string, unknown> }
export interface ChannelUsageWindow { window: string; used: number | null; limit: number | null; remaining: number | null; percent: number | null; reset_at?: string; unit?: string }
export interface Channel { id: string; name: string; base_url: string; avg_first_token_ms: number | null; upstream_balance: number | null; upstream_used: number | null; upstream_total: number | null; upstream_currency: string; upstream_usage_windows: ChannelUsageWindow[]; upstream_balance_supported: boolean; upstream_balance_error: string; upstream_balance_fetched_at: string | null; provider: 'openai' | 'ollama' | 'kimi' | 'opencode_go' | 'anthropic' | 'deepseek' | 'commandcode' | 'custom'; models: string[]; test_model: string; enabled: boolean; auto_disabled: boolean; auto_disable: boolean; disabled_reason: string; priority: number; weight: number; last_test_time: string | null; last_error: string | null; response_time_ms: number; used_requests: number; used_tokens: number; groups: string[]; key_type: 'single' | 'multi'; key_count: number; upstream_path: string; upstream_format: string; request_overrides: RequestOverrides; ua_pool: string[]; created_at: string; updated_at: string; model_routes: ModelRoute[]; user_id: string | null; user_email: string; user_name: string }

export interface ChannelKey { id: string; name: string; enabled: boolean; priority: number; last_checked_at: string | null; last_error: string | null; created_at: string; upstream_balance: number | null; upstream_used: number | null; upstream_total: number | null; upstream_currency: string; upstream_usage_windows: ChannelUsageWindow[]; upstream_balance_supported: boolean; upstream_balance_error: string; upstream_balance_fetched_at: string | null }
export interface ChannelKeyForm { name?: string; api_key?: string; priority?: number }
export interface ChannelKeyTestResult { success: boolean; status_code: number; latency_ms: number; reason?: string; auto_disabled: boolean; channel_disabled?: boolean }
export interface ChannelTestKeyResult { key_id: string; success: boolean; status_code: number; latency_ms: number; reason?: string; auto_disabled: boolean }
export interface ChannelTestResult { success: boolean; status_code: number; latency_ms: number; reason?: string; channel_disabled: boolean; keys: ChannelTestKeyResult[] }
export interface ChannelBalance { balance: number | null; used: number | null; total: number | null; currency: string; usage_windows: ChannelUsageWindow[]; supported: boolean; fetched_at: string }

export interface ChannelQuotaLimit { id: string; window: 'minute' | 'day' | 'month'; max_requests: number | null; max_tokens: number | null; created_at: string }
export interface ChannelQuotaUsage { window: string; max_requests: number | null; max_tokens: number | null; requests: number; tokens: number }
export interface ChannelQuota { limits: ChannelQuotaLimit[]; usage: ChannelQuotaUsage[] }
export interface ChannelQuotaForm { window: 'minute' | 'day' | 'month'; max_requests?: number | null; max_tokens?: number | null }
export interface ChannelUsageStats {
  total_requests: number
  success_count: number
  error_count: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost: string
  avg_duration_ms: number
  avg_first_token_ms: number | null
}

export interface ModelRoute { id: string; public_model: string; upstream_model: string; channel_id?: string; priority: number; weight: number; enabled: boolean; hidden: boolean; created_at: string }
export interface ModelRouteForm { public_model: string; upstream_model: string; priority?: number; weight?: number; hidden?: boolean }
export interface Group { id: string; name: string; display_name: string | null; description: string | null; multiplier: number; max_concurrency: number | null; public: boolean; created_at: string }
export interface GroupUpdate { id: string; multiplier: number; max_concurrency: number | null; public: boolean; display_name?: string; description?: string }
export interface RequestLog { request_id: string; user_id: string | null; user_name: string; api_key_id: string | null; key_name: string; channel_id: string | null; channel_name: string; channel_key_id: string | null; channel_key_name: string; group_id: string | null; group_name: string; model: string; status_code: number; prompt_tokens: number | null; completion_tokens: number | null; total_tokens: number | null; duration_ms: number; first_token_ms: number | null; error_code: string | null; error_detail: string; client_ip: string; user_agent: string; created_at: string }
export interface Account { id: string; email: string; name: string; role: string; avatar_url: string; permissions: string[]; balance: number; reserved: number; pending_settlement: number; leaderboard_opt_in: boolean; leaderboard_mask_name: boolean; data_usage_enabled: boolean; must_change_password?: boolean }
export interface Pricing { id: string; model: string; input_per_million: number; cached_input_per_million: number; output_per_million: number; multiplier: number; enabled: boolean; updated_at: string }
export interface PricingTier { id: string; model: string; from_tokens: number; input_per_million: number; cached_input_per_million: number; output_per_million: number; created_at: string }
export interface PricingTierForm { id?: string; model: string; from_tokens: number; input_per_million: number; cached_input_per_million: number; output_per_million: number }
export interface PricingTimeRule { id: string; model: string; name: string; start_minute: number; end_minute: number; weekdays: string; input_per_million: number; cached_input_per_million: number; output_per_million: number; enabled: boolean; created_at: string }
export interface PricingTimeRuleForm { id?: string; model: string; name: string; start_minute: number; end_minute: number; weekdays: string; input_per_million: number; cached_input_per_million: number; output_per_million: number; enabled?: boolean }
export interface CatalogGroup { id: string; name: string; multiplier: number; public: boolean; display_name?: string | null }
export interface ModelCatalogMetadata {
  description: string | null
  input_modalities: string[]
  output_modalities: string[]
  context_window: number | null
}
export interface ModelCatalogPerformance {
  requests: number | null
  success_rate: number | null
  avg_latency_ms: number | null
  avg_first_token_ms: number | null
}
export interface CatalogModel {
  id: string
  model: string
  provider: string
  provider_slug: string
  input_per_million: number | null
  cached_input_per_million: number | null
  output_per_million: number | null
  multiplier: number | null
  description: string | null
  input_modalities: string[]
  output_modalities: string[]
  context_window: number | null
  performance: ModelCatalogPerformance | null
  groups: CatalogGroup[]
}
export interface ModelMetadata {
  id: string
  model: string
  description: string
  input_modalities: string[]
  output_modalities: string[]
  context_window: number | null
  created_at?: string
  updated_at?: string
}
export type ModelMetadataForm = Omit<ModelMetadata, 'id' | 'created_at' | 'updated_at'>
export interface ModelPerformanceGroup { group_id: string; group_name: string; requests: number; tps: number; avg_latency_ms: number; avg_first_token_ms: number | null; success_rate: number }
export interface ModelPerformance { model: string; window_hours: number; groups: ModelPerformanceGroup[]; updated_at: string }
export interface ModelProvider { id: string; name: string; slug: string; prefixes: string[]; priority: number }
export interface UsageRecord { request_id: string; model: string; prompt_tokens: number; cached_prompt_tokens: number; completion_tokens: number; cost: string; status: string; created_at: string; client_ip: string; user_agent: string; error: string; key_name: string; subscription: boolean; duration_ms: number; first_token_ms: number | null; group_name: string }

export interface AccountUsageSummary { requests: number; tokens: number; cost: string }
export interface DailyUsageRecord { day: string; requests: number; prompt_tokens: number; completion_tokens: number }
export interface ActivityLog { id: string; type: 'request' | 'login' | 'register' | 'logout' | 'topup' | 'operation'; action: string; user_id: string; user_name: string; model: string; group_id: string; group_name: string; status_code: number | null; duration_ms: number | null; first_token_ms: number | null; prompt_tokens: number; completion_tokens: number; total_tokens: number; cost: number; details: Record<string, unknown>; created_at: string }
export interface LedgerEntry { id: string; amount: string; balance_after: string; kind: string; request_id: string | null; note: string | null; created_at: string; settlement_status: 'not_applicable' | 'pending' | 'processing' | 'settled' | 'failed'; settlement_date: string | null; settled_at: string | null; settlement_error?: string }
export interface AdminLedgerEntry extends LedgerEntry { user_id: string; user_email: string; user_name: string }

export interface CheckinEntry { checkin_date: string; streak: number; reward: string; created_at: string }
export interface CheckinStatus { checked_in: boolean; data: CheckinEntry[] }
export interface AdminCheckin { user_id: string; email: string; user_name: string; checkin_date: string; streak: number; reward: string; created_at: string }
export interface PaymentOrder { order_no: string; payment_type: string; amount: string; status: 'pending' | 'paid' | 'failed' | 'expired'; provider_trade_no?: string; paid_at: string | null; created_at: string }
export interface PaymentMethod { id: string; code: string; name: string; enabled: boolean; created_at: string | null }
export interface PaymentSettings { enabled: boolean; base_url: string; merchant_id: string; has_merchant_key: boolean; public_base_url: string; methods: PaymentMethod[] }
export interface ModelRanking { rank: number; previous_rank?: number; model_name: string; vendor: string; total_tokens: number; share: number; growth_pct: number }
export interface VendorRanking { rank: number; vendor: string; total_tokens: number; share: number; growth_pct: number; models_count: number; top_model: string }
export interface RankingMover { model_name: string; vendor: string; rank_delta: number; current_rank: number; growth_pct: number }
export interface UserRanking { rank: number; name: string; total_tokens: number; total_cost: number; share: number; growth_pct: number; requests: number; top_model: string }
export interface Rankings { period: string; models: ModelRanking[]; vendors: VendorRanking[]; top_movers: RankingMover[]; top_droppers: RankingMover[]; users: UserRanking[]; total_tokens: number; updated_at: string }
export type FeaturedLocale = 'zh' | 'zh-Hant' | 'en'
export type FeaturedCopyField = 'badge' | 'title' | 'body' | 'cta'
export type FeaturedCopy = Record<FeaturedLocale, Record<FeaturedCopyField, string>>
export interface SiteSettings { name: string; icon_url: string; announcement: string; contact_email?: string; auto_disable_failed_channels: boolean; invitations_enabled?: boolean; registration_email_whitelist_enabled?: boolean; registration_email_alias_blocked?: boolean; featured_enabled: boolean; featured_model: string; featured_copy: FeaturedCopy; captcha_provider?: string; geetest_enabled?: boolean; geetest_captcha_id?: string; corptcha_site_id?: string; email_verification_enabled?: boolean; oauth_providers?: string[] }
export interface AdminSiteSettings { name: string; icon_url: string; announcement: string; contact_email: string; auto_disable_failed_channels: boolean; featured_enabled: boolean; featured_model: string; featured_copy: FeaturedCopy; captcha_provider: string; geetest_captcha_id: string; has_geetest_captcha_key: boolean; corptcha_site_id: string; has_corptcha_secret: boolean; smtp_host: string; smtp_port: string; smtp_username: string; has_smtp_password: boolean; smtp_from: string; public_base_url: string; invitations_enabled: boolean; inviter_reward: string; invitee_reward: string; checkin_base_reward: string; checkin_streak_bonus: string; checkin_max_bonus_days: number; registration_email_whitelist_enabled: boolean; registration_email_whitelist: string[]; registration_email_alias_blocked: boolean }
export interface Notification { id: string; title: string; content: string; enabled: boolean; sort_order: number; created_at: string; updated_at: string }
export interface NotificationForm { title: string; content: string; enabled: boolean; sort_order: number }
export interface Invitation { id: string; name: string; email: string; reward: string; created_at: string }
export interface InvitationSummary { enabled: boolean; code: string; inviter_reward: string; invitee_reward: string; data: Invitation[] }
export interface ReliabilitySettings { request_timeout_seconds: number; retry_count: number; retry_status_codes: string; health_check_mode: 'off' | 'scheduled_all' | 'passive_recovery'; health_check_interval_minutes: number; health_check_auto_recover: boolean; health_check_channel_ids: string; auto_disable_on_test_failure: boolean; auto_disable_slow_seconds: number; auto_disable_status_codes: string; auto_disable_keywords: string }
export interface ContentPolicySettings { request_audit_enabled: boolean; request_audit_store_mode: 'none' | 'hash' | 'excerpt'; request_audit_retention_days: number; content_policy_mode: 'off' | 'audit' | 'block' }
export interface ContentPolicyRule { id: string; name: string; term: string; action: 'block' | 'audit'; case_sensitive: boolean; enabled: boolean; priority: number; created_at: string; updated_at: string }
export interface ContentPolicyBatchResult { created: number; skipped: number }
export interface RequestContentAudit { id: string; request_id: string; user_id: string; api_key_id: string; model: string; endpoint: string; decision: 'allow' | 'audit' | 'block'; matched_rule_ids: string[]; request_bytes: number; content_length: number; content_hash: string; excerpt: string; created_at: string }

export interface ConversationCacheSettings { conversation_cache_enabled: boolean }

export interface ConversationLog {
  id: string
  request_id: string
  user_id: string
  api_key_id: string
  model: string
  status_code: number
  stream: boolean
  duration_ms: number
  first_token_ms: number | null
  created_at: string
}

export interface ConversationLogDetail extends ConversationLog {
  request_body: unknown
  response_body: unknown
}

export interface PaymentRedirect { order_no: string; amount: string; status: 'pending'; pay_url: string }
export interface BalanceSubscriptionResult { order_no: string; subscription_id: string; amount: string; status: 'paid'; balance: string }

export interface AccountGroups { data: string[]; groups: Group[]; user_groups: string[]; user_group: string }

export interface UsageLog {
  request_id: string
  user_id: string | null
  user_name: string
  api_key_id: string | null
  key_name: string
  channel_id: string | null
  channel_name: string
  channel_key_id: string | null
  channel_key_name: string
  group_id: string
  group_name: string
  model: string
  status_code: number
  prompt_tokens: number | null
  completion_tokens: number | null
  total_tokens: number | null
  cached_prompt_tokens: number
  duration_ms: number
  first_token_ms: number | null
  error_code: string | null
  error_detail: string
  client_ip: string
  user_agent: string
  cost: string
  subscription: boolean
  created_at: string
}

export interface UsageStats {
  total_requests: number
  success_count: number
  error_count: number
  prompt_tokens: number
  cached_prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost: string
  avg_duration_ms: number
  avg_first_token_ms: number | null
  breakdown?: UsageStatBreakdown[]
}

export interface UsageStatBreakdown {
  period: string
  requests: number
  prompt_tokens: number
  cached_prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  avg_first_token_ms: number | null
  cost: string
}

export type BillingPeriod = 'hour' | 'day' | 'week' | 'month' | 'year'

export type OveragePolicy = 'allow_wallet' | 'block'

export interface SubscriptionPlanModelQuota {
  model: string
  max_requests_per_period: number | null
  max_credit_per_period: number | null
}

export interface SubscriptionModelUsage {
  model: string
  requests: number
  credit: number
}

export interface SubscriptionPlan {
  id: string
  name: string
  description: string
  price: string
  currency: string
  billing_period: BillingPeriod
  credit_amount: string
  group_id: string
  group_name: string
  model_whitelist: string[]
  max_requests_per_period: number | null
  max_credit_per_period: number | null
  overage_policy: OveragePolicy
  model_quotas: SubscriptionPlanModelQuota[]
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
  billing_period: BillingPeriod
  credit_amount: string
  group_name: string
  model_whitelist: string[]
  max_requests_per_period: number | null
  max_credit_per_period: number | null
  overage_policy: OveragePolicy
  model_quotas: SubscriptionPlanModelQuota[]
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
  billing_period: BillingPeriod
  credit_amount: string
  group_id: string
  group_name: string
  model_whitelist: string[]
  max_requests_per_period: number | null
  max_credit_per_period: number | null
  overage_policy: OveragePolicy
  model_quotas: SubscriptionPlanModelQuota[]
  usage_requests: number
  usage_credit: number
  model_usage: SubscriptionModelUsage[]
  reset_card_count: number
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

export interface OrderRecord {
  order_no: string
  order_type: 'payment' | 'subscription'
  plan_name: string
  payment_type: string
  amount: string
  status: 'pending' | 'paid' | 'failed' | 'expired'
  provider_trade_no?: string
  period_kind: string
  paid_at: string | null
  created_at: string
}

export interface AdminOrder extends OrderRecord {
  user_id: string
  user_email: string
  user_name: string
}

export interface InvoiceSettings { enabled: boolean; need_pay_tax: boolean }

export type RedemptionRewardType = 'balance' | 'subscription'

export interface RedemptionCode {
  id: string
  batch_id: string
  code: string
  reward_type: RedemptionRewardType
  amount: string
  plan_id: string
  plan_name: string
  period_days: number | null
  max_uses: number
  used_count: number
  expires_at: string | null
  enabled: boolean
  note: string
  redeemed_by: string | null
  redeemed_at: string | null
  created_at: string
  updated_at: string
}

export interface RedemptionCodeRedemption {
  id: string
  code_id: string
  code: string
  user_id: string
  user_email: string
  user_name: string
  amount: string
  plan_id: string
  plan_name: string
  subscription_id: string
  created_at: string
}

export interface RedemptionCodeForm {
  reward_type: RedemptionRewardType
  amount: string
  plan_id: string
  period_days: number | null
  max_uses: number
  quantity: number
  expires_at: string
  note: string
}

export interface RedemptionResult {
  reward_type: RedemptionRewardType
  amount?: string
  subscription_id?: string
  plan_id?: string
}

export interface RedemptionCodeUpdate {
  enabled?: boolean
  expires_at?: string | null
  note?: string
  max_uses?: number
}

export interface ResetCard {
  id: string
  batch_id: string
  subscription_id: string
  user_id: string
  user_email: string
  user_name: string
  plan_id: string
  plan_name: string
  enabled: boolean
  expires_at: string | null
  note: string
  used_by: string | null
  used_at: string | null
  created_at: string
  updated_at: string
}

export interface ResetCardForm {
  subscription_id: string
  quantity: number
  expires_at: string
  note: string
}

export interface ResetCardUpdate {
  enabled?: boolean
  expires_at?: string | null
  note?: string
}

export interface ResetCardByPlanForm {
  plan_id: string
  status: string
  quantity: number
  expires_at: string
  note: string
}

export interface ResetCardAllForm {
  status: string
  quantity: number
  expires_at: string
  note: string
}

export interface AccountResetCard {
  id: string
  subscription_id: string
  enabled: boolean
  expires_at: string | null
  note: string
  used_at: string | null
  created_at: string
  available: boolean
}
export interface AdminInvoiceSettings { enabled: boolean; base_url: string; client_id: string; has_client_secret: boolean; need_pay_tax: boolean }
export interface AdminInvoiceSettingsForm { enabled: boolean; base_url: string; client_id: string; client_secret: string; need_pay_tax: boolean }
export interface InvoiceEligibleOrder { order_no: string; invoice_no: string; order_type: 'payment' | 'subscription'; plan_name: string; amount: string; paid_at: string | null }
export interface InvoiceCheckout { taxOrderNo: string; payUrl: string }
export interface InvoiceValidatedOrder { platformOrderId: number | null; orderNo: string; externalNo: string; productName: string; amount: string; currency: string; paidAt: string; verifiedAt: string; transactionId?: string }
export interface InvoiceValidation {
  orders: InvoiceValidatedOrder[]
  totalAmount: string
  currency: string
  taxAmount: string
  taxPaidAmount: string
  taxDueAmount: string
  taxPayments: Record<string, InvoiceCheckout>
  taxOrderNo: string
  payUrl: string
}
export interface InvoiceApplication {
  id: string
  application_id: string
  status: 'pending' | 'approved' | 'rejected' | 'completed' | 'canceled'
  buyer_type: 'individual' | 'company'
  title: string
  recipient_email: string
  total_amount: string
  currency: string
  need_pay_tax: boolean
  created_at: string
}
export interface InvoiceCreateResult { id: string; application_id: string; status: 'pending' }
export interface InvoiceTaxStatus { paid: boolean }

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
  max_requests_per_period: number | null
  max_credit_per_period: number | null
  remaining_requests: number | null
  remaining_credit: number | null
  model_quota_count: number
}

export interface AdminUserSubscriptionModelUsage {
  model: string
  max_requests_per_period: number | null
  max_credit_per_period: number | null
  remaining_requests: number | null
  remaining_credit: number | null
}

export interface AdminUserSubscription {
  id: string
  plan_id: string
  plan_name: string
  status: 'pending' | 'active' | 'expired' | 'cancelled'
  current_period_start: string | null
  current_period_end: string | null
  auto_renew: boolean
  cancelled_at: string | null
  created_at: string
  updated_at: string
  max_requests_per_period: number | null
  max_credit_per_period: number | null
  remaining_requests: number | null
  remaining_credit: number | null
  model_usage: AdminUserSubscriptionModelUsage[]
}

export interface AdminCreateSubscriptionForm {
  plan_id: string
  start_at: string
  end_at: string
  auto_renew: boolean
}

export interface AdminUpdateSubscriptionForm {
  status?: string
  current_period_start?: string | null
  current_period_end?: string | null
  auto_renew?: boolean
  remaining_requests?: number
  remaining_credit?: number
}

export interface PublicActivityItem {
  model: string
  status_code: number
  duration_ms: number
  first_token_ms: number | null
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

/**
 * API error carrying the backend's stable `code` (e.g. "email_not_allowed",
 * "invalid_credentials") alongside the human-readable message, so views can map
 * known codes to translated strings while still falling back to the raw message.
 */
export class ApiError extends Error {
  code: string
  status: number
  constructor(message: string, status: number, code = '') {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api${path}`, { ...init, headers: { Authorization: `Bearer ${token}`, ...(init.body ? { 'Content-Type': 'application/json' } : {}), ...init.headers } })
  // A 401 from the console means the session is gone. Drop the stale token and
  // send the user back to sign-in instead of leaving them staring at dead UI.
  if (response.status === 401 && path !== '/auth/login') {
    clearToken()
    if (import.meta.client && !window.location.pathname.startsWith('/auth')) {
      const current = window.location.pathname + window.location.search
      window.location.assign(`/auth?redirect=${encodeURIComponent(current)}`)
    }
  }
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    const code = typeof body?.error?.code === 'string' ? body.error.code : ''
    throw new ApiError(body?.error?.message ?? `请求失败 (${response.status})`, response.status, code)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

async function get<T>(path: string): Promise<T> { return api<T>(path) }
async function post<T>(path: string, body?: unknown): Promise<T> { return api<T>(path, { method: 'POST', body: body === undefined ? undefined : JSON.stringify(body) }) }
async function put<T>(path: string, body?: unknown): Promise<T> { return api<T>(path, { method: 'PUT', body: body === undefined ? undefined : JSON.stringify(body) }) }
async function send(path: string, method: 'POST' | 'PUT' | 'DELETE', body?: unknown): Promise<void> { await api<unknown>(path, { method, body: body === undefined ? undefined : JSON.stringify(body) }) }

/** Downloads a binary response (invoice PDF) as a Blob, keeping the auth header. */
async function download(path: string): Promise<Blob> {
  const response = await fetch(`/api${path}`, { headers: { Authorization: `Bearer ${token}` } })
  if (response.status === 401) {
    clearToken()
    if (import.meta.client && !window.location.pathname.startsWith('/auth')) {
      const current = window.location.pathname + window.location.search
      window.location.assign(`/auth?redirect=${encodeURIComponent(current)}`)
    }
  }
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    const code = typeof body?.error?.code === 'string' ? body.error.code : ''
    throw new ApiError(body?.error?.message ?? `请求失败 (${response.status})`, response.status, code)
  }
  return response.blob()
}

export interface LoginBody { email: string; password: string; code?: string; captcha_id?: string; lot_number?: string; captcha_output?: string; pass_token?: string; gen_time?: string; captcha_token?: string; captcha_purpose?: string }
export interface RegisterBody { name: string; email: string; password: string; code?: string; invitation_code?: string; captcha_id?: string; lot_number?: string; captcha_output?: string; pass_token?: string; gen_time?: string; captcha_token?: string; captcha_purpose?: string }
export interface KeyForm { user_id?: string; name: string; expires_at: string; group_id: string }
export interface AccountKeyForm { name: string; expires_at: string; group_id: string }
export interface ChannelForm { name: string; provider: string; base_url: string; key_type: 'single' | 'multi'; api_keys: string; models: string[]; test_model?: string; priority: number; groups: string[]; upstream_path?: string; upstream_format?: string; model_routes?: ModelRouteForm[]; auto_disable: boolean; request_overrides?: RequestOverrides; ua_pool?: string[]; user_email?: string }
export interface ProviderForm { name: string; slug: string; prefixes: string[]; priority: number; id?: string }
export interface PaymentSettingsForm { enabled: boolean; base_url: string; merchant_id: string; merchant_key: string; public_base_url: string }
export interface PaymentMethodForm { code: string; name: string; enabled: boolean }
export interface PricingForm { model: string; input_per_million: number; cached_input_per_million: number; output_per_million: number; multiplier: number }
export interface NewApiPricingForm { base_url: string; api_key: string; price_per_quota_unit: number }
export interface SubscriptionPlanForm { name: string; description: string; price: string; currency: string; billing_period: string; credit_amount: string; group_id: string; model_whitelist: string[]; max_requests_per_period: number | null; max_credit_per_period: number | null; overage_policy: OveragePolicy; model_quotas: SubscriptionPlanModelQuota[]; sort_order: number; enabled: boolean }
export interface UserUpdate { id?: number; name?: string; email?: string; role?: string; enabled?: boolean; password?: string; balance?: number | null; note?: string; permissions?: string[]; groups?: string[]; leaderboard_opt_in?: boolean; leaderboard_mask_name?: boolean; data_usage_enabled?: boolean; max_concurrency?: number | null; inviter_id?: number | null }
export interface UserCreate { name: string; email: string; password: string; role: string; enabled: boolean; permissions: string[]; groups: string[] }
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
  Omit<AdminSiteSettings, 'has_geetest_captcha_key' | 'has_corptcha_secret' | 'has_smtp_password'>
  & { geetest_captcha_key: string; corptcha_secret: string; smtp_password: string }

export const endpoints = {
  getSiteSettings: () => get<SiteSettings>('/site-settings'),
  getAccount: () => get<Account>('/account/me'),
  getAccountKeys: () => get<{ data: ApiKey[] }>('/account/keys'),
  getAccountUsage: (query = '') => get<{ data: UsageRecord[] }>(`/account/usage${query}`),
  getAccountUsageDaily: (days: number, offsetMinutes: number) => get<{ data: DailyUsageRecord[] }>(`/account/usage/daily?days=${days}&offset=${offsetMinutes}`),
  getAccountUsageSummary: () => get<AccountUsageSummary>('/account/usage/summary'),
  getAccountLedger: () => get<{ data: LedgerEntry[] }>('/account/ledger'),
  getCheckinStatus: () => get<CheckinStatus>('/account/checkin'),
  checkin: (captcha?: Record<string, string>) => post<{ checked_in: boolean; already_checked_in: boolean; checkin_date: string; streak?: number; reward?: number }>('/account/checkin', captcha ?? {}),
  getAccountGroups: () => get<AccountGroups>('/account/groups'),
  getAccountPayments: () => get<{ enabled: boolean; payment_methods: PaymentMethod[]; data: PaymentOrder[] }>('/account/payments'),
  getAccountPayment: (orderNo: string) => get<PaymentOrder>(`/account/payments/${encodeURIComponent(orderNo)}`),
  createAccountPayment: (amount: string, type: string) => post<PaymentRedirect>('/account/payments', { amount, type }),
  getAccountSubscriptions: () => get<{ data: UserSubscription[] }>('/account/subscriptions'),
  createAccountSubscription: (planId: string, paymentType: string, autoRenew: boolean) => post<PaymentRedirect>('/account/subscriptions', { plan_id: planId, payment_type: paymentType, auto_renew: autoRenew }),
  createBalanceSubscription: (planId: string) => post<BalanceSubscriptionResult>('/account/subscriptions/balance', { plan_id: planId }),
  cancelAccountSubscription: (id: string) => send(`/account/subscriptions/${encodeURIComponent(id)}/cancel`, 'POST'),
  getAccountSubscriptionOrders: () => get<{ data: SubscriptionOrder[] }>('/account/subscription-orders'),
  getAccountSubscriptionOrder: (orderNo: string) => get<SubscriptionOrder>(`/account/subscription-orders/${encodeURIComponent(orderNo)}`),
  getAccountOrders: () => get<{ data: OrderRecord[] }>('/account/orders'),
  redeemCode: (code: string) => post<RedemptionResult>('/account/redeem', { code }),
  getAccountRedemptions: () => get<{ data: RedemptionCodeRedemption[] }>('/account/redemptions'),
  useResetCard: (subscriptionId: string) => post<{ used: boolean; card_id: string }>(`/account/subscriptions/${encodeURIComponent(subscriptionId)}/reset`),
  getAccountResetCards: () => get<{ data: AccountResetCard[] }>('/account/reset-cards'),
  getAccountInvitations: () => get<InvitationSummary>('/account/invitations'),
  getInvoiceSettings: () => get<InvoiceSettings>('/account/invoice/settings'),
  getInvoiceEligibleOrders: () => get<{ data: InvoiceEligibleOrder[] }>('/account/invoices/eligible-orders'),
  validateInvoiceOrders: (orderNos: string[], needPayTax: boolean, taxOrderNos: string[] = []) => post<InvoiceValidation>('/account/invoices/validate', { orderNos, needPayTax, ...(taxOrderNos.length ? { taxOrderNos } : {}) }),
  getInvoiceTaxStatus: (taxOrderNo: string) => post<InvoiceTaxStatus>('/account/invoices/tax-status', { taxOrderNo }),
  createInvoiceApplication: (body: Record<string, unknown>) => post<InvoiceCreateResult>('/account/invoices', body),
  getInvoices: (sync = false) => get<{ data: InvoiceApplication[] }>(`/account/invoices${sync ? '?sync=1' : ''}`),
  cancelInvoice: (id: string) => post<{ id: string; status: string }>(`/account/invoices/${encodeURIComponent(id)}/cancel`),
  downloadInvoicePDF: (id: string) => download(`/account/invoices/${encodeURIComponent(id)}/pdf`),
  getAdminInvoiceSettings: () => get<AdminInvoiceSettings>('/admin/invoice-settings'),
  updateAdminInvoiceSettings: (form: AdminInvoiceSettingsForm) => put<AdminInvoiceSettings>('/admin/invoice-settings', form),
  updateAccountProfile: (avatarUrl: string) => send('/account/profile', 'PUT', { avatar_url: avatarUrl }),
  changeAccountPassword: (currentPassword: string, newPassword: string) => send('/account/password', 'PUT', { current_password: currentPassword, new_password: newPassword }),
  revokeAccountKey: (id: string) => send(`/account/keys/${encodeURIComponent(id)}/revoke`, 'POST'),
  updateAccountPreferences: (leaderboardOptIn: boolean, leaderboardMaskName: boolean, dataUsageEnabled: boolean) => send('/account/preferences', 'PUT', { leaderboard_opt_in: leaderboardOptIn, leaderboard_mask_name: leaderboardMaskName, data_usage_enabled: dataUsageEnabled }),
  createAccountKey: (form: AccountKeyForm) => post<{ key: string }>('/account/keys', form),
  updateAccountKey: (id: string, form: AccountKeyForm) => send(`/account/keys/${encodeURIComponent(id)}`, 'PUT', form),
  revealAccountKey: (id: string) => get<{ key: string }>(`/account/keys/${encodeURIComponent(id)}/secret`),
  getKeyQuota: (id: string) => get<KeyQuota>(`/account/keys/${encodeURIComponent(id)}/quota`),
  upsertKeyQuota: (id: string, form: KeyQuotaForm) => post<{ id: string }>(`/account/keys/${encodeURIComponent(id)}/quota`, form),
  deleteKeyQuota: (id: string, window: string) => send(`/account/keys/${encodeURIComponent(id)}/quota?window=${encodeURIComponent(window)}`, 'DELETE'),

  getActivityLogs: (query = '') => get<{ data: ActivityLog[] }>(`/activity-logs${query}`),
  getModelCatalog: () => get<{ data: CatalogModel[]; groups: CatalogGroup[] }>('/model-catalog'),
  getModelPerformance: (model: string) => get<ModelPerformance>(`/model-performance?model=${encodeURIComponent(model)}`),
  getAdminModelMetadata: () => get<{ data: ModelMetadata[] }>('/admin/model-metadata'),
  createAdminModelMetadata: (form: ModelMetadataForm) => post<{ id: string; model: string }>('/admin/model-metadata', form),
  updateAdminModelMetadata: (id: string, form: ModelMetadataForm) => put<{ id: string; model: string }>(`/admin/model-metadata/${encodeURIComponent(id)}`, form),
  deleteAdminModelMetadata: (id: string) => send(`/admin/model-metadata/${encodeURIComponent(id)}`, 'DELETE'),
  getPublicSubscriptionPlans: () => get<{ data: PublicSubscriptionPlan[] }>('/subscription-plans'),

  login: (body: LoginBody) => post<{ token: string }>('/auth/login', body),
  register: (body: RegisterBody) => post<{ token: string }>('/auth/register', body),
  logout: () => send('/auth/logout', 'POST'),
  sendEmailCode: (email: string, captcha?: Record<string, string>) => send('/auth/email-code', 'POST', { email, ...captcha }),
  requestPasswordReset: (email: string, captcha?: Record<string, string>) => send('/auth/password-reset/request', 'POST', { email, ...captcha }),
  confirmPasswordReset: (token: string, password: string) => send('/auth/password-reset/confirm', 'POST', { token, password }),

  getAdminUsers: (query = '') => get<Page<User>>(`/admin/users${query}`),
  getAdminCheckins: (query = '') => get<Page<AdminCheckin>>(`/admin/checkins${query}`),
  withdrawAdminCheckin: (userId: string, date: string) => post<{ withdrawn: boolean; reward: number }>(`/admin/users/${encodeURIComponent(userId)}/checkins/${encodeURIComponent(date)}/withdraw`),
  createUser: (form: UserCreate) => post<{ id: string }>('/admin/users', form),
  updateUser: (id: string, update: UserUpdate) => send(`/admin/users/${encodeURIComponent(id)}`, 'PUT', update),
  getAdminGroups: (query = '') => get<Page<Group>>(`/admin/groups${query}`),
  createGroup: (name: string, multiplier: number, maxConcurrency: number | null, publicGroup: boolean, displayName = '', description = '') => send('/admin/groups', 'POST', { name, multiplier, max_concurrency: maxConcurrency, public: publicGroup, display_name: displayName, description }),
  updateGroup: (id: string, multiplier: number, maxConcurrency: number | null, publicGroup: boolean, displayName = '', description = '') => send(`/admin/groups/${encodeURIComponent(id)}`, 'PUT', { multiplier, max_concurrency: maxConcurrency, public: publicGroup, display_name: displayName, description }),
  deleteGroup: (id: string) => send(`/admin/groups/${encodeURIComponent(id)}`, 'DELETE'),
  importGroups: (entries: Record<string, number>) => send('/admin/groups/import', 'POST', entries),
  batchUpdateGroups: (groups: GroupUpdate[]) => post<{ affected: number }>('/admin/groups/batch-update', { groups }),
  getAdminKeys: (query = '') => get<Page<ApiKey>>(`/admin/keys${query}`),
  createKey: (form: KeyForm) => post<{ key: string }>('/admin/keys', form),
  revokeKey: (id: string) => send(`/admin/keys/${encodeURIComponent(id)}/revoke`, 'POST'),
  revealKey: (id: string) => get<{ key: string }>(`/admin/keys/${encodeURIComponent(id)}/secret`),
  getAdminChannels: (query = '') => get<Page<Channel>>(`/admin/channels${query}`),
  batchToggleChannels: (ids: string[], enabled: boolean) => post<{ affected: number }>('/admin/channels/batch-status', { ids, enabled }),
  createChannel: (form: ChannelForm) => send('/admin/channels', 'POST', form),
  copyChannel: (id: string, form?: { name?: string }) => post<{ id: string; name: string; enabled: boolean }>(`/admin/channels/${encodeURIComponent(id)}/copy`, form),
  fetchChannelModels: (baseUrl: string, apiKey: string) => post<{ models: string[] }>('/admin/channels/models', { base_url: baseUrl, api_key: apiKey }),
  updateChannel: (id: string, form: ChannelForm) => send(`/admin/channels/${encodeURIComponent(id)}`, 'PUT', form),
  updateChannelGroups: (id: string, groups: string[]) => send(`/admin/channels/${encodeURIComponent(id)}/groups`, 'PUT', { groups }),
  toggleChannel: (id: string, enabled: boolean) => send(`/admin/channels/${encodeURIComponent(id)}/status`, 'POST', { enabled }),
  getChannelKeys: (id: string) => get<{ data: ChannelKey[] }>(`/admin/channels/${encodeURIComponent(id)}/keys`),
  createChannelKey: (id: string, form: ChannelKeyForm) => post<{ id: string }>(`/admin/channels/${encodeURIComponent(id)}/keys`, form),
  deleteChannelKey: (channelId: string, keyId: string) => send(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}`, 'DELETE'),
  toggleChannelKey: (channelId: string, keyId: string, enabled: boolean) => send(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}/status`, 'POST', { enabled }),
  updateChannelKey: (channelId: string, keyId: string, form: { name?: string; priority?: number }) => send(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}`, 'PUT', form),
  revealChannelKey: (channelId: string, keyId: string) => get<{ key: string }>(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}/secret`),
  testChannelKey: (channelId: string, keyId: string) => post<ChannelKeyTestResult>(`/admin/channels/${encodeURIComponent(channelId)}/keys/${encodeURIComponent(keyId)}/test`),
  testChannel: (id: string) => post<ChannelTestResult>(`/admin/channels/${encodeURIComponent(id)}/test`),
  refreshChannelBalance: (id: string, keyId?: string) => post<ChannelBalance>(`/admin/channels/${encodeURIComponent(id)}/balance${keyId ? `?key_id=${encodeURIComponent(keyId)}` : ''}`),
  migrateChannelKeys: (id: string) => post<{ migrated: boolean }>(`/admin/channels/${encodeURIComponent(id)}/keys/migrate`),
  getAdminProviders: () => get<{ data: ModelProvider[] }>('/admin/providers'),
  saveProvider: (form: ProviderForm) => send('/admin/providers', 'POST', form),
  removeProvider: (id: string) => send(`/admin/providers/${encodeURIComponent(id)}`, 'DELETE'),
  getAdminPricing: () => get<{ data: Pricing[] }>('/admin/pricing'),
  savePricing: (form: PricingForm) => send('/admin/pricing', 'POST', form),
  syncNewApiPricing: (form: NewApiPricingForm) => post<{ synced: number; skipped: number }>('/admin/pricing/newapi/sync', form),
  getAdminPricingTiers: (model: string) => get<{ data: PricingTier[] }>(`/admin/pricing/tiers?model=${encodeURIComponent(model)}`),
  savePricingTier: (form: PricingTierForm) => send('/admin/pricing/tiers', 'POST', form),
  deletePricingTier: (id: string, model: string) => send(`/admin/pricing/tiers/${encodeURIComponent(id)}?model=${encodeURIComponent(model)}`, 'DELETE'),
  getAdminPricingTimeRules: (model: string) => get<{ data: PricingTimeRule[] }>(`/admin/pricing/time-rules?model=${encodeURIComponent(model)}`),
  savePricingTimeRule: (form: PricingTimeRuleForm) => send('/admin/pricing/time-rules', 'POST', form),
  deletePricingTimeRule: (id: string, model: string) => send(`/admin/pricing/time-rules/${encodeURIComponent(id)}?model=${encodeURIComponent(model)}`, 'DELETE'),
  getAdminReliabilitySettings: () => get<ReliabilitySettings>('/admin/reliability-settings'),
  updateReliabilitySettings: (form: ReliabilitySettings) => put<ReliabilitySettings>('/admin/reliability-settings', form),
  getContentPolicySettings: () => get<ContentPolicySettings>('/admin/content-policy/settings'),
  updateContentPolicySettings: (form: Partial<ContentPolicySettings>) => put<ContentPolicySettings>('/admin/content-policy/settings', form),
  getContentPolicyRules: () => get<{ data: ContentPolicyRule[] }>('/admin/content-policy/rules'),
  createContentPolicyRule: (form: Omit<ContentPolicyRule, 'id' | 'created_at' | 'updated_at'>) => post<{ id: string }>('/admin/content-policy/rules', form),
  updateContentPolicyRule: (id: string, form: Partial<Omit<ContentPolicyRule, 'id' | 'created_at' | 'updated_at'>>) => put<{ id: string }>(`/admin/content-policy/rules/${encodeURIComponent(id)}`, form),
  deleteContentPolicyRule: (id: string) => send(`/admin/content-policy/rules/${encodeURIComponent(id)}`, 'DELETE'),
  batchCreateContentPolicyRules: (form: { terms: string; action: ContentPolicyRule['action']; enabled?: boolean; priority?: number }) => post<ContentPolicyBatchResult>('/admin/content-policy/rules/batch', form),

  getConversationCacheSettings: () => get<ConversationCacheSettings>('/admin/conversation-cache/settings'),
  updateConversationCacheSettings: (form: ConversationCacheSettings) => put<ConversationCacheSettings>('/admin/conversation-cache/settings', form),
  getConversationLogs: (query = '') => get<{ data: ConversationLog[]; total: number; page: number; page_size: number }>(`/admin/conversation-cache${query}`),
  getConversationLogDetail: (id: string) => get<ConversationLogDetail>(`/admin/conversation-cache/${encodeURIComponent(id)}`),
  getAdminSiteSettings: () => get<AdminSiteSettings>('/admin/site-settings'),
  updateAdminSiteSettings: (form: SiteSettingsForm) => put<AdminSiteSettings>('/admin/site-settings', form),
  getNotifications: () => get<{ data: Notification[] }>('/notifications'),
  getAdminNotifications: () => get<{ data: Notification[] }>('/admin/notifications'),
  createNotification: (form: NotificationForm) => post<Notification>('/admin/notifications', form),
  updateNotification: (id: string, form: NotificationForm) => put<Notification>(`/admin/notifications/${encodeURIComponent(id)}`, form),
  deleteNotification: (id: string) => send(`/admin/notifications/${encodeURIComponent(id)}`, 'DELETE'),
  getAdminOrders: (query = '') => get<Page<AdminOrder>>(`/admin/orders${query}`),
  getAdminWalletLedger: (query = '') => get<Page<AdminLedgerEntry>>(`/admin/wallet-ledger${query}`),
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
  resetActiveSubscriptionQuotas: (status: 'active' | 'inactive' | 'all') => post<{ affected: number; status: string }>('/admin/subscriptions/reset-quotas', { status }),
  resetAdminSubscriptionQuota: (id: string) => post<{ ok: boolean }>(`/admin/subscriptions/${encodeURIComponent(id)}/reset-quotas`),
  getAdminUserSubscriptions: (userId: string) => get<{ data: AdminUserSubscription[] }>(`/admin/users/${encodeURIComponent(userId)}/subscriptions`),
  createAdminUserSubscription: (userId: string, form: AdminCreateSubscriptionForm) => post<{ id: string }>(`/admin/users/${encodeURIComponent(userId)}/subscriptions`, form),
  updateAdminSubscription: (id: string, form: AdminUpdateSubscriptionForm) => send(`/admin/subscriptions/${encodeURIComponent(id)}`, 'PUT', form),
  voidAdminSubscription: (id: string) => send(`/admin/subscriptions/${encodeURIComponent(id)}/void`, 'POST'),
  deleteAdminSubscription: (id: string) => send(`/admin/subscriptions/${encodeURIComponent(id)}`, 'DELETE'),
  runMigration: (form: MigrateForm) => post<MigrateResult>('/admin/migrate', form),
  getMigrationStatus: () => get<MigrationStatus>('/admin/migrate'),

  getUsageLogs: (query = '') => get<{ data: UsageLog[]; total: number; page: number; page_size: number }>(`/admin/usage-logs${query}`),
  getUsageStats: (query = '') => get<UsageStats>(`/admin/usage-stats${query}`),
  getRequestAudits: (query = '') => get<{ data: RequestContentAudit[]; total: number; page: number; page_size: number }>(`/admin/request-audits${query}`),

  getRequestLogs: () => get<{ data: RequestLog[] }>('/admin/request-logs'),
  getAuditLogs: () => get<{ data: AuditLog[] }>('/admin/audit-logs'),
  getModelRoutes: () => get<{ data: ModelRoute[] }>('/admin/model-routes'),
  createModelRoute: (form: Record<string, unknown>) => send('/admin/model-routes', 'POST', form),
  updateModelRoute: (id: string, form: Record<string, unknown>) => send(`/admin/model-routes/${encodeURIComponent(id)}`, 'PUT', form),
  getChannelRoutes: (channelId: string) => get<{ data: ModelRoute[] }>(`/admin/channels/${encodeURIComponent(channelId)}/routes`),
  createChannelRoute: (channelId: string, form: ModelRouteForm) => post<{ id: string }>(`/admin/channels/${encodeURIComponent(channelId)}/routes`, form),
  updateChannelRoute: (channelId: string, routeId: string, form: Record<string, unknown>) => send(`/admin/channels/${encodeURIComponent(channelId)}/routes/${encodeURIComponent(routeId)}`, 'PUT', form),
  deleteChannelRoute: (channelId: string, routeId: string) => send(`/admin/channels/${encodeURIComponent(channelId)}/routes/${encodeURIComponent(routeId)}`, 'DELETE'),
  getChannelQuota: (channelId: string) => get<ChannelQuota>(`/admin/channels/${encodeURIComponent(channelId)}/quota`),
  upsertChannelQuota: (channelId: string, form: ChannelQuotaForm) => post<{ id: string }>(`/admin/channels/${encodeURIComponent(channelId)}/quota`, form),
  deleteChannelQuota: (channelId: string, window: string) => send(`/admin/channels/${encodeURIComponent(channelId)}/quota?window=${encodeURIComponent(window)}`, 'DELETE'),
  getChannelUsageStats: (channelId: string) => get<ChannelUsageStats>(`/admin/channels/${encodeURIComponent(channelId)}/usage-stats`),
  getGroupNames: () => get<{ data: string[] }>('/group'),
  setAccountKeyGroup: (id: string, groupId: string) => send(`/account/keys/${encodeURIComponent(id)}/group`, 'PUT', { group_id: groupId }),
  setKeyGroup: (id: string, groupId: string) => send(`/admin/keys/${encodeURIComponent(id)}/group`, 'PUT', { group_id: groupId }),
  setUserRole: (id: string, role: string) => send(`/admin/users/${encodeURIComponent(id)}/role`, 'POST', { role }),
  setUserPermissions: (id: string, permissions: string[]) => send(`/admin/users/${encodeURIComponent(id)}/permissions`, 'PUT', { permissions }),
  setUserGroups: (id: string, groups: string[]) => send(`/admin/users/${encodeURIComponent(id)}/groups`, 'PUT', { groups }),
  adjustBalance: (userId: string, amount: number, note: string) => send('/admin/wallets/adjustments', 'POST', { user_id: userId, amount, note }),
  getRedemptionCodes: (query = '') => get<Page<RedemptionCode>>(`/admin/redemption-codes${query}`),
  createRedemptionCodes: (form: RedemptionCodeForm) => post<{ batch_id: string; codes: string[] }>('/admin/redemption-codes', form),
  updateRedemptionCode: (id: string, form: RedemptionCodeUpdate) => send(`/admin/redemption-codes/${encodeURIComponent(id)}`, 'PUT', form),
  deleteRedemptionCode: (id: string) => send(`/admin/redemption-codes/${encodeURIComponent(id)}`, 'DELETE'),
  getRedemptionCodeRedemptions: (id: string) => get<Page<RedemptionCodeRedemption>>(`/admin/redemption-codes/${encodeURIComponent(id)}/redemptions`),
  getResetCards: (query = '') => get<Page<ResetCard>>(`/admin/reset-cards${query}`),
  createResetCards: (form: ResetCardForm) => post<{ batch_id: string; quantity: number }>('/admin/reset-cards', form),
  createResetCardsByPlan: (form: ResetCardByPlanForm) => post<{ batch_id: string; subscriptions: number; quantity: number }>('/admin/reset-cards/by-plan', form),
  createResetCardsAll: (form: ResetCardAllForm) => post<{ batch_id: string; subscriptions: number; quantity: number }>('/admin/reset-cards/all', form),
  updateResetCard: (id: string, form: ResetCardUpdate) => send(`/admin/reset-cards/${encodeURIComponent(id)}`, 'PUT', form),
  deleteResetCard: (id: string) => send(`/admin/reset-cards/${encodeURIComponent(id)}`, 'DELETE'),
  resetAdminUserQuota: (userId: string) => post<{ reset_subscriptions: number }>(`/admin/users/${encodeURIComponent(userId)}/reset-quotas`),
  getMigrationRequests: () => get<{ data: MigrationRequest[] }>('/admin/migrate/requests'),

  getOAuthProviders: () => get<{ data: OAuthProvider[] }>('/admin/oauth/providers'),
  saveOAuthProvider: (provider: string, form: OAuthProviderForm) => send(`/admin/oauth/providers/${encodeURIComponent(provider)}`, 'POST', form),
  deleteOAuthProvider: (provider: string) => send(`/admin/oauth/providers/${encodeURIComponent(provider)}`, 'DELETE'),

  getOAuthConnections: () => get<{ data: OAuthConnection[] }>('/account/oauth/connections'),
  unlinkOAuthConnection: (provider: string) => send(`/account/oauth/${encodeURIComponent(provider)}/unlink`, 'POST'),
}
