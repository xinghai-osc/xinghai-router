import { endpoints, type CatalogGroup } from '~/src/api'
import { dedupeSquareModels, toSquareModel, type SquareModel } from '~/src/marketplace'

/**
 * Public model catalog. Fetched on the client because the public pages are
 * prerendered — baking the catalog at build time would ship stale pricing.
 */
export function useCatalog() {
  const { t } = useI18n()
  const models = useState<SquareModel[]>('catalog-models', () => [])
  const groups = useState<CatalogGroup[]>('catalog-groups', () => [])
  const loading = useState('catalog-loading', () => false)
  const loaded = useState('catalog-loaded', () => false)
  const error = useState('catalog-error', () => '')
  const scope = useState('catalog-scope', () => '')
  const { sessionScope, loadAccount } = useAccount()

  function resetScope(nextScope: string) {
    if (scope.value === nextScope) return
    models.value = []
    groups.value = []
    loaded.value = false
    loading.value = false
    error.value = ''
    scope.value = nextScope
  }

  watch(sessionScope, resetScope, { flush: 'sync', immediate: true })

  async function loadCatalog(force = false) {
    if (!import.meta.client) return
    await loadAccount()
    const nextScope = sessionScope.value
    resetScope(nextScope)
    if (loading.value || (loaded.value && !force)) return
    loading.value = true
    error.value = ''
    try {
      const response = await endpoints.getModelCatalog()
      if (scope.value !== nextScope || sessionScope.value !== nextScope) return
      models.value = dedupeSquareModels((response.data ?? []).map(toSquareModel))
      groups.value = response.groups ?? []
      loaded.value = true
    } catch (cause) {
      if (scope.value === nextScope) error.value = cause instanceof Error ? cause.message : t('common.loadFailed')
    } finally {
      if (scope.value === nextScope) loading.value = false
    }
  }

  return { models, groups, loading, loaded, error, loadCatalog }
}
