import { ApiError, clearLegacySession, clearPublicCache, endpoints, invalidateSessionRequests, setSessionExpiredHandler, type Account } from '~/src/api'

let accountRequest: Promise<void> | null = null
let accountRequestId = 0

export function useAccount() {
  const { t } = useI18n()
  const { toast } = useToast()
  const account = useState<Account | null>('account', () => null)
  const loading = useState('account-loading', () => false)
  const loaded = useState('account-loaded', () => false)
  const error = useState('account-error', () => '')
  const sessionRevision = useState('account-session-revision', () => 0)
  const { cancel: cancelReauthentication } = useReauthentication()

  const authenticated = computed(() => Boolean(account.value))
  const isAdmin = computed(() => account.value?.role === 'admin')
  const sessionScope = computed(() => `${sessionRevision.value}:${account.value?.id ?? 'anonymous'}`)

  function can(permission: string): boolean {
    if (!account.value) return false
    if (account.value.role === 'admin') return true
    return account.value.permissions.includes(permission)
  }

  function clearSession() {
    invalidateSessionRequests()
    cancelReauthentication()
    accountRequestId += 1
    accountRequest = null
    account.value = null
    loading.value = false
    loaded.value = true
    error.value = ''
    sessionRevision.value += 1
    clearLegacySession()
    clearPublicCache('model-catalog')
  }

  if (import.meta.client) setSessionExpiredHandler(clearSession)

  async function loadAccount(force = false): Promise<void> {
    if (!import.meta.client) return
    if (accountRequest && !force) return accountRequest
    if (loaded.value && !force) return

    const requestId = ++accountRequestId
    loading.value = true
    error.value = ''
    clearLegacySession()
    const request = (async () => {
      try {
        const next = await endpoints.getAccount()
        if (requestId === accountRequestId) {
          account.value = next
          loaded.value = true
        }
      } catch (cause) {
        if (requestId === accountRequestId) {
          if (cause instanceof ApiError && cause.status === 401) clearSession()
          else error.value = cause instanceof Error ? cause.message : t('common.loadFailed')
        }
      } finally {
        if (requestId === accountRequestId) {
          accountRequest = null
          loading.value = false
        }
      }
    })()
    accountRequest = request
    return request
  }

  async function signIn() {
    clearSession()
    await loadAccount(true)
    if (!account.value) throw new Error(error.value || t('common.sessionExpired'))
  }

  async function signOut() {
    cancelReauthentication()
    invalidateSessionRequests()
    try {
      await endpoints.logout()
    } catch (cause) {
      if (!(cause instanceof ApiError && cause.status === 401)) {
        toast.error(cause instanceof Error ? cause.message : t('common.actionFailed'))
        return
      }
    }
    clearSession()
    await navigateTo('/auth')
  }

  return { account, loading, loaded, error, authenticated, isAdmin, sessionScope, can, loadAccount, signIn, signOut }
}
