import { clearToken, endpoints, getToken, setToken, type Account } from '~/src/api'

/**
 * Signed-in account state. The bearer token lives in a cookie (see src/api.ts)
 * so the SSR route middleware can gate /console before hydration.
 */
export function useAccount() {
  const { t } = useI18n()
  const account = useState<Account | null>('account', () => null)
  const loading = useState('account-loading', () => false)
  const error = useState('account-error', () => '')

  const authenticated = computed(() => Boolean(account.value))
  const isAdmin = computed(() => account.value?.role === 'admin')

  function can(permission: string): boolean {
    if (!account.value) return false
    if (account.value.role === 'admin') return true
    return account.value.permissions.includes(permission)
  }

  async function loadAccount(force = false) {
    if (!import.meta.client) return
    if (!getToken()) { account.value = null; return }
    if (account.value && !force) return
    loading.value = true
    error.value = ''
    try {
      account.value = await endpoints.getAccount()
    } catch (cause) {
      account.value = null
      clearToken()
      error.value = cause instanceof Error ? cause.message : t('common.sessionExpired')
    } finally {
      loading.value = false
    }
  }

  async function signIn(token: string) {
    setToken(token)
    await loadAccount(true)
  }

  async function signOut() {
    try {
      await endpoints.logout()
    } catch {
      // The local session is dropped regardless of the server response.
    }
    clearToken()
    account.value = null
    await navigateTo('/auth')
  }

  return { account, loading, error, authenticated, isAdmin, can, loadAccount, signIn, signOut }
}
