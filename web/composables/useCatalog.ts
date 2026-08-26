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

  async function loadCatalog(force = false) {
    if (!import.meta.client || loading.value || (loaded.value && !force)) return
    loading.value = true
    error.value = ''
    try {
      const response = await endpoints.getModelCatalog()
      models.value = dedupeSquareModels((response.data ?? []).map(toSquareModel))
      groups.value = response.groups ?? []
      loaded.value = true
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : t('common.loadFailed')
    } finally {
      loading.value = false
    }
  }

  return { models, groups, loading, loaded, error, loadCatalog }
}
