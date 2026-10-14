import { endpoints, type SubscriptionProduct } from '~/src/api'

/** Public subscription products, shared by the landing page and /pricing. */
export function usePlans() {
  const { t } = useI18n()
  const products = useState<SubscriptionProduct[]>('public-plans', () => [])
  const loading = useState('public-plans-loading', () => false)
  const loaded = useState('public-plans-loaded', () => false)
  const error = useState('public-plans-error', () => '')

  async function loadPlans(force = false) {
    if (!import.meta.client || loading.value || (loaded.value && !force)) return
    loading.value = true
    error.value = ''
    try {
      const response = await endpoints.getPublicSubscriptionPlans()
      products.value = response.data ?? []
      loaded.value = true
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : t('common.loadFailed')
    } finally {
      loading.value = false
    }
  }

  return { products, loading, loaded, error, loadPlans }
}
