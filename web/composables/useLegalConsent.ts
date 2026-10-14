import { endpoints, type LegalVersions } from '~/src/api'

const STORAGE_KEY = 'xinghai.legal-consent'

function storeGuestConsent(versions: LegalVersions) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(versions)) } catch { return }
}

export function useLegalConsent() {
  const { account, loaded, error: accountError, sessionScope, loadAccount } = useAccount()
  const versions = useRuntimeConfig().public.legalVersions as unknown as LegalVersions
  const acceptedScope = useState('legal-accepted-scope', () => '')
  const checkedScope = useState('legal-checked-scope', () => '')
  const busy = useState('legal-consent-busy', () => false)
  const failed = useState('legal-consent-failed', () => false)
  const requestId = useState('legal-consent-request', () => 0)
  const ready = computed(() => loaded.value && !accountError.value && checkedScope.value === sessionScope.value)
  const accepted = computed(() => ready.value && acceptedScope.value === sessionScope.value)

  function matches(value: unknown): boolean {
    if (!value || typeof value !== 'object') return false
    const stored = value as Partial<LegalVersions>
    return stored.terms_version === versions.terms_version && stored.privacy_version === versions.privacy_version
  }

  async function check() {
    const id = ++requestId.value
    const scope = sessionScope.value
    acceptedScope.value = ''
    checkedScope.value = ''
    failed.value = false
    if (!loaded.value || accountError.value) return
    busy.value = true
    try {
      let consent: unknown = null
      if (account.value) consent = await endpoints.getLegalConsent()
      else {
        try { consent = JSON.parse(localStorage.getItem(STORAGE_KEY) || 'null') } catch { consent = null }
      }
      if (id !== requestId.value || scope !== sessionScope.value) return
      checkedScope.value = scope
      if (matches(consent)) acceptedScope.value = scope
    } catch {
      if (id === requestId.value) failed.value = true
    } finally {
      if (id === requestId.value) busy.value = false
    }
  }

  async function accept() {
    if (!ready.value || busy.value) return
    const id = ++requestId.value
    const scope = sessionScope.value
    busy.value = true
    failed.value = false
    try {
      if (account.value) await endpoints.acceptLegalConsent(versions)
      else storeGuestConsent(versions)
      if (id === requestId.value && scope === sessionScope.value) acceptedScope.value = scope
    } catch {
      if (id === requestId.value) failed.value = true
    } finally {
      if (id === requestId.value) busy.value = false
    }
  }

  async function retry() {
    await loadAccount(true)
    await check()
  }

  return { accepted, ready, busy, failed, check, accept, retry }
}
