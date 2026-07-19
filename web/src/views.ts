/** All views handled by the console shell. */
export type View =
  | 'overview' | 'users' | 'groups' | 'keys' | 'channels' | 'providers'
  | 'logs' | 'account' | 'profile' | 'wallet' | 'usage'
  | 'usage-overview' | 'ledger' | 'pricing'
  | 'site-settings' | 'payment-settings' | 'audit' | 'reliability'
  | 'subscriptions' | 'subscription-plans' | 'admin-subscriptions'

export const VIEWS: View[] = [
  'overview', 'users', 'groups', 'keys', 'channels', 'providers',
  'logs', 'account', 'profile', 'wallet', 'usage',
  'usage-overview', 'ledger', 'pricing',
  'site-settings', 'payment-settings', 'audit', 'reliability',
  'subscriptions', 'subscription-plans', 'admin-subscriptions',
]
