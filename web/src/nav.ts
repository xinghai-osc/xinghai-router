import {
  Activity, ArrowRightLeft, BadgeDollarSign, Bell, Blocks, CalendarCheck, CreditCard, Database, FileClock, FileText,
  KeyRound, LayoutDashboard, Layers, LogIn, MessageSquareText, Repeat, RotateCcw, ScrollText, Server,
  Settings, ShieldAlert, ShieldCheck, ShoppingCart, Sparkles, Stamp, Ticket, UserCog, UserPlus, Users, Wallet,
} from 'lucide-vue-next'
import type { Component } from 'vue'

export interface NavItem {
  to: string
  /** i18n key under the `nav` namespace. */
  labelKey: string
  icon: Component
  /** Required permission; omitted items are visible to every signed-in user. */
  permission?: string
}

export interface NavSection {
  titleKey: string
  items: NavItem[]
}

export const NAV_SECTIONS: NavSection[] = [
  {
    titleKey: 'nav.sectionOverview',
    items: [
      { to: '/console', labelKey: 'nav.overview', icon: LayoutDashboard },
      { to: '/console/keys', labelKey: 'nav.keys', icon: KeyRound },
      { to: '/console/usage', labelKey: 'nav.usage', icon: Activity },
      { to: '/console/checkin', labelKey: 'nav.checkin', icon: CalendarCheck },
    ],
  },
  {
    titleKey: 'nav.sectionBilling',
    items: [
      { to: '/console/ledger', labelKey: 'nav.orders', icon: ShoppingCart },
      { to: '/console/wallet', labelKey: 'nav.wallet', icon: Wallet },
      { to: '/console/redeem', labelKey: 'nav.redeem', icon: Ticket },
      { to: '/console/subscriptions', labelKey: 'nav.subscriptions', icon: Sparkles },
      { to: '/console/invoices', labelKey: 'nav.invoices', icon: FileText },
    ],
  },
  {
    titleKey: 'nav.sectionAccount',
    items: [
      { to: '/console/account', labelKey: 'nav.account', icon: UserCog },
      { to: '/console/invitations', labelKey: 'nav.invitations', icon: UserPlus },
    ],
  },
  {
    titleKey: 'nav.sectionOperations',
    items: [
      { to: '/console/users', labelKey: 'nav.users', icon: Users, permission: 'users.read' },
      { to: '/console/admin-checkins', labelKey: 'nav.adminCheckins', icon: CalendarCheck, permission: 'users.read' },
      { to: '/console/admin-orders', labelKey: 'nav.adminOrders', icon: ShoppingCart, permission: 'users.read' },
      { to: '/console/admin-wallet-ledger', labelKey: 'nav.walletLedger', icon: Wallet, permission: 'users.read' },
      { to: '/console/groups', labelKey: 'nav.groups', icon: Layers, permission: 'users.read' },
      { to: '/console/channels', labelKey: 'nav.channels', icon: Server, permission: 'channels.read' },
      { to: '/console/providers', labelKey: 'nav.providers', icon: Blocks, permission: 'system.manage' },
      { to: '/console/pricing', labelKey: 'nav.pricing', icon: BadgeDollarSign, permission: 'pricing.read' },
      { to: '/console/model-routes', labelKey: 'nav.modelRoutes', icon: ArrowRightLeft, permission: 'routes.manage' },
      { to: '/console/logs', labelKey: 'nav.logs', icon: ScrollText, permission: 'logs.read' },
      { to: '/console/conversation-cache', labelKey: 'nav.conversationCache', icon: MessageSquareText, permission: 'logs.read' },
      { to: '/console/audit', labelKey: 'nav.audit', icon: FileClock, permission: 'audit.read' },
      { to: '/console/request-audits', labelKey: 'nav.requestAudits', icon: ShieldAlert, permission: 'logs.read' },
    ],
  },
  {
    titleKey: 'nav.sectionSystem',
    items: [
      { to: '/console/subscription-plans', labelKey: 'nav.subscriptionPlans', icon: Sparkles, permission: 'system.manage' },
      { to: '/console/redemption-codes', labelKey: 'nav.redemptionCodes', icon: Ticket, permission: 'system.manage' },
      { to: '/console/reset-cards', labelKey: 'nav.resetCards', icon: RotateCcw, permission: 'system.manage' },
      { to: '/console/admin-subscriptions', labelKey: 'nav.adminSubscriptions', icon: Repeat, permission: 'users.read' },
      { to: '/console/reliability', labelKey: 'nav.reliability', icon: ShieldCheck, permission: 'system.manage' },
      { to: '/console/content-policy', labelKey: 'nav.contentPolicy', icon: ShieldAlert, permission: 'system.manage' },
      { to: '/console/conversation-cache-settings', labelKey: 'nav.conversationCacheSettings', icon: MessageSquareText, permission: 'system.manage' },
      { to: '/console/payment-settings', labelKey: 'nav.paymentSettings', icon: CreditCard, permission: 'system.manage' },
      { to: '/console/invoice-settings', labelKey: 'nav.invoiceSettings', icon: Stamp, permission: 'system.manage' },
      { to: '/console/oauth-settings', labelKey: 'nav.oauthSettings', icon: LogIn, permission: 'system.manage' },
      { to: '/console/notifications', labelKey: 'nav.notifications', icon: Bell, permission: 'system.manage' },
      { to: '/console/site-settings', labelKey: 'nav.siteSettings', icon: Settings, permission: 'system.manage' },
      { to: '/console/migrate', labelKey: 'nav.migrate', icon: Database, permission: 'system.manage' },
    ],
  },
]

/** Route → i18n label key, used by the console header to title the page. */
export const NAV_TITLE_KEYS: Record<string, string> = Object.fromEntries(
  NAV_SECTIONS.flatMap(section => section.items.map(item => [item.to, item.labelKey])),
)
