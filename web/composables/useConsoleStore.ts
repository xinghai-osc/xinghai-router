import type { Component, InjectionKey, Ref, ComputedRef } from 'vue'
import type { Account, ActivityLog, AdminSiteSettings, AdminSubscription, ApiKey, CatalogGroup, CatalogModel, Channel, Group, LedgerEntry, MigrationStatus, ModelProvider, PaymentMethod, PaymentOrder, PaymentSettings, Pricing, PublicSubscriptionPlan, ReliabilitySettings, SiteSettings, SubscriptionOrder, SubscriptionPlan, UsageRecord, User, UserSubscription } from '~/src/api'
import type { useI18n } from '~/composables/useI18n'
import type { View } from '~/src/views'

type NavItem = readonly [id: string, label: string, icon: Component]
type ManagementNavItem = readonly [id: string, label: string, icon: Component, permission: string]

/**
 * Shared store for the console shell.
 *
 * `RouterApp.vue` owns all reactive state and actions, and provides this
 * object via `provide`. Each view/modal component injects it via
 * `useConsoleStore()` so that we can split templates into separate files
 * without prop drilling.
 */
export interface ConsoleStore {
  locale: Ref<Locale>
  t: ReturnType<typeof useI18n>['t']
  setLocale: ReturnType<typeof useI18n>['setLocale']
  toggleLocale: ReturnType<typeof useI18n>['toggleLocale']
  initializeLocale: ReturnType<typeof useI18n>['initializeLocale']

  route: ReturnType<typeof useRoute>
  router: ReturnType<typeof useRouter>
  view: ComputedRef<View>
  views: View[]
  openConsole: (nextView: string) => void

  authenticated: Ref<boolean>
  busy: Ref<boolean>
  error: Ref<string>
  sidebarCollapsed: Ref<boolean>
  setupCollapsed: Ref<boolean>
  account: Ref<Account | null>
  can: (permission: string) => boolean
  isAdmin: ComputedRef<boolean>
  signOut: () => Promise<void>

  // Navigation sections (labels are localized computeds)
  generalNav: ComputedRef<NavItem[]>
  billingNav: ComputedRef<NavItem[]>
  personalNav: ComputedRef<NavItem[]>
  managementNav: ComputedRef<ManagementNavItem[]>
  localizedManagementNavItems: ComputedRef<ManagementNavItem[]>
  localizedAdminExtraNav: ComputedRef<ManagementNavItem[]>

  // Data refs
  users: Ref<User[]>
  groups: Ref<Group[]>
  ownGroups: Ref<string[]>
  keys: Ref<ApiKey[]>
  accountKeys: Ref<ApiKey[]>
  channels: Ref<Channel[]>
  providers: Ref<ModelProvider[]>
  activityLogs: Ref<ActivityLog[]>
  usageRecords: Ref<UsageRecord[]>
  ledger: Ref<LedgerEntry[]>
  payments: Ref<PaymentOrder[]>
  paymentMethods: Ref<PaymentMethod[]>
  paymentsEnabled: Ref<boolean>
  paymentMessage: Ref<string>
  paymentForm: { amount: number; type: string }
  paymentSettings: PaymentSettings
  paymentSettingsForm: { enabled: boolean; base_url: string; merchant_id: string; merchant_key: string; public_base_url: string }
  paymentMethodForm: { code: string; name: string; enabled: boolean }
  pricing: Ref<Pricing[]>
  siteSettings: Ref<SiteSettings>
  siteSettingsForm: AdminSiteSettings & { geetest_captcha_key: string; smtp_password: string }
  reliabilityForm: ReliabilitySettings
  newAPIPricingForm: { base_url: string; api_key: string; price_per_quota_unit: number }
  catalog: Ref<CatalogModel[]>
  catalogGroups: Ref<CatalogGroup[]>
  catalogLoaded: Ref<boolean>
  catalogGroup: Ref<string>
  catalogSearch: Ref<string>
  activityModels: Ref<string[]>
  activityFilters: { user_id: string; model: string; group_id: string; start: string; end: string; type: string }
  leaderboardPrefs: { opt_in: boolean; mask_name: boolean }

  // Subscription state
  subscriptionPlans: Ref<SubscriptionPlan[]>
  publicPlans: Ref<PublicSubscriptionPlan[]>
  userSubscriptions: Ref<UserSubscription[]>
  subscriptionOrders: Ref<SubscriptionOrder[]>
  adminSubscriptions: Ref<AdminSubscription[]>
  subscriptionPlanForm: { name: string; description: string; price: string; currency: string; billing_period: string; credit_amount: string; group_id: string; model_whitelist: string; max_requests_per_period: string | number; max_tokens_per_period: string | number; sort_order: number; enabled: boolean }
  editingPlanID: Ref<string>
  showPlanModal: Ref<boolean>
  subscribingPlan: Ref<PublicSubscriptionPlan | null>
  subscribeForm: { payment_type: string; auto_renew: boolean }
  subscriptionMessage: Ref<string>
  extendForm: { plan_id: string; days: number }

  // Modal state
  createdKey: Ref<string>
  showKey: Ref<boolean>
  showAccountKey: Ref<boolean>
  editingAccountKey: Ref<ApiKey | null>
  showChannel: Ref<boolean>
  editingChannel: Ref<Channel | null>
  showProvider: Ref<boolean>
  selectedUser: Ref<User | null>
  originalUser: Ref<User | null>
  selectedPermissions: Ref<string[]>
  selectedGroups: Ref<string[]>
  userPassword: Ref<string>
  userBalance: Ref<number | null>
  userBalanceNote: Ref<string>
  keyForm: { user_id: string; name: string; expires_at: string; group_id: string }
  accountKeyForm: { name: string; expires_at: string; group_id: string }
  channelForm: { name: string; provider: string; base_url: string; api_key: string; models: string; priority: number; groups: string[] }
  providerForm: { name: string; slug: string; prefixes: string; priority: number }
  editingProviderID: Ref<string>
  groupForm: { name: string; multiplier: number }
  groupImportText: Ref<string>
  avatarUrlInput: Ref<string>
  avatarInput: Ref<HTMLInputElement | null>

  // Migration
  migrateForm: { source_dsn: string; source_driver: string }
  migrateStatus: Ref<MigrationStatus | null>
  migratePolling: Ref<boolean>

  // Actions
  load: () => Promise<void>
  action: (work: () => Promise<void>) => Promise<void>
  loadActivity: (filters?: boolean) => Promise<void>
  filterActivity: () => Promise<void>
  resetActivityFilters: () => Promise<void>
  createKey: () => Promise<void>
  createAccountKey: () => Promise<void>
  editAccountKey: (key: ApiKey) => void
  updateAccountKey: () => Promise<void>
  fetchChannelModels: () => Promise<void>
  createChannel: () => Promise<void>
  editChannel: (channel: Channel) => void
  updateChannel: () => Promise<void>
  saveProvider: () => Promise<void>
  openProvider: () => void
  editProvider: (provider: ModelProvider) => void
  removeProvider: (provider: ModelProvider) => Promise<void>
  createGroup: () => Promise<void>
  editGroupMultiplier: (group: Group, event: Event) => Promise<void>
  importGroups: () => Promise<void>
  toggleChannel: (channel: Channel) => Promise<void>
  revokeKey: (key: ApiKey) => Promise<void>
  revokeAccountKey: (key: ApiKey) => Promise<void>
  createPayment: () => Promise<void>
  savePaymentSettings: () => Promise<void>
  createPaymentMethod: () => Promise<void>
  updatePaymentMethod: (method: PaymentMethod) => Promise<void>
  deletePaymentMethod: (method: PaymentMethod) => Promise<void>
  copyKey: () => Promise<void>
  savePricing: () => Promise<void>
  syncNewAPIPricing: () => Promise<void>
  manageUser: (user: User) => void
  saveUserAccess: () => Promise<void>
  chooseAvatar: (event: Event) => Promise<void>
  removeAvatar: () => Promise<void>
  saveAvatarUrl: () => Promise<void>
  saveLeaderboardPrefs: () => Promise<void>
  saveSiteSettings: () => Promise<void>
  saveReliabilitySettings: () => Promise<void>
  loadPublicPlans: () => Promise<void>
  openSubscribeModal: (plan: PublicSubscriptionPlan) => Promise<void>
  confirmSubscribe: () => Promise<void>
  cancelSubscription: (sub: UserSubscription) => Promise<void>
  savePlan: () => Promise<void>
  openPlanModal: () => void
  editPlan: (plan: SubscriptionPlan) => void
  deletePlan: (plan: SubscriptionPlan) => Promise<void>
  extendSubscriptions: () => Promise<void>

  // Migration actions
  runMigration: () => Promise<void>
  pollMigrationStatus: () => Promise<void>
  stopMigrationPolling: () => void

  // Computed helpers
  personalRequests: ComputedRef<number>
  personalTokens: ComputedRef<number>
  personalCost: ComputedRef<number>
  setupProgress: ComputedRef<number>
  filteredCatalog: ComputedRef<CatalogModel[]>
  apiEndpoint: ComputedRef<string>
  usageChart: ComputedRef<Array<{ key: string; label: string; tokens: number; cost: number; tokenHeight: number; costHeight: number }>>
  usageLinePoints: ComputedRef<string>
  userName: (id: string | null) => string
  formatDate: (value: string | null) => string
  short: (value: string | null) => string
  formatPrice: (value: number | null, multiplier?: number) => string
  providerIcon: (slug: string) => string
  modelProvider: (model: string) => string
  selectedCatalogGroup: (item: CatalogModel) => CatalogGroup | undefined
  actualMultiplier: (item: CatalogModel) => number
  activityTypeLabel: ComputedRef<Record<ActivityLog['type'], string>>
  actionLabel: (item: ActivityLog) => string
  activityDetail: (item: ActivityLog) => string
  Empty: ReturnType<typeof h>
  permissions: string[]
}

export const CONSOLE_STORE_KEY: InjectionKey<ConsoleStore> = Symbol('xinghai-console-store')

export function useConsoleStore(): ConsoleStore {
  const store = inject(CONSOLE_STORE_KEY)
  if (!store) throw new Error('useConsoleStore() must be used within <RouterApp> with a provided store')
  return store
}
