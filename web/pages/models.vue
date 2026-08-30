<script setup lang="ts">
import { SearchX } from 'lucide-vue-next'
import {
  extractVendors, filterAndSort, FILTER_ALL, PAGE_SIZE, squareModelKey,
  type ContextBucket, type Modality, type SortOption, type SquareModel, type TokenUnit, type ViewMode,
} from '~/src/marketplace'

const { t } = useI18n()
const { settings } = useSiteSettings()
const { models, groups, loaded, error, loadCatalog } = useCatalog()
const route = useRoute()
const router = useRouter()

useHead({
  title: () => `${t('site.sqMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.sqMetaDescription') }],
})

const search = ref('')
const vendor = ref(FILTER_ALL)
const group = ref(FILTER_ALL)
const inputModalities = ref<Modality[]>([])
const outputModalities = ref<Modality[]>([])
const contextBuckets = ref<ContextBucket[]>([])
const sortBy = ref<SortOption>('name')
const view = ref<ViewMode>('card')
const unit = ref<TokenUnit>('M')
const compareMode = ref(false)
const comparedKeys = ref<string[]>([])
const page = ref(1)

const toolbar = ref<{ focusSearch: () => void } | null>(null)
const resultsAnchor = ref<HTMLElement | null>(null)
const hydrated = ref(false)
const applyingQuery = ref(false)
const pageReady = ref(false)

const VIEW_PREF_KEY = 'xinghai.marketplace.view'
const UNIT_PREF_KEY = 'xinghai.marketplace.unit'

const detailOpen = ref(false)
const selected = ref<SquareModel | null>(null)

const vendors = computed(() => extractVendors(models.value))
const comparedModels = computed(() => models.value.filter(model => comparedKeys.value.includes(squareModelKey(model))))

const filtered = computed(() => filterAndSort(models.value, {
  search: search.value,
  vendor: vendor.value,
  group: group.value,
  sortBy: sortBy.value,
  inputModalities: inputModalities.value,
  outputModalities: outputModalities.value,
  contextBuckets: contextBuckets.value,
}))

const filtersActive = computed(() =>
  Boolean(search.value.trim()) || vendor.value !== FILTER_ALL || group.value !== FILTER_ALL || inputModalities.value.length > 0 || outputModalities.value.length > 0 || contextBuckets.value.length > 0)
const metadataAvailability = computed(() => ({
  input: models.value.some(model => model.input_modalities?.length),
  output: models.value.some(model => model.output_modalities?.length),
  context: models.value.some(model => model.context_window != null),
}))
const featuredModel = computed(() => {
  if (filtersActive.value || !settings.value.featured_enabled) return undefined
  const configured = settings.value.featured_model.trim()
  return models.value.find(model => model.model === configured) ?? filtered.value[0]
})
const promoEligible = computed(() => view.value === 'card' && !filtersActive.value && Boolean(featuredModel.value))
const promoVisible = computed(() => page.value === 1 && promoEligible.value)
const listedModels = computed(() => {
  if (!promoEligible.value || !featuredModel.value) return filtered.value
  const featuredKey = squareModelKey(featuredModel.value)
  return filtered.value.filter(model => squareModelKey(model) !== featuredKey)
})

const totalPages = computed(() => Math.max(1, Math.ceil(listedModels.value.length / PAGE_SIZE)))
const paged = computed(() => listedModels.value.slice((page.value - 1) * PAGE_SIZE, page.value * PAGE_SIZE))

const { locale } = useI18n()
const featuredCopy = computed(() => {
  const fallback = {
    badge: t('site.sqPromoBadge'),
    title: t('site.sqPromoTitle'),
    body: t('site.sqPromoBody', { model: featuredModel.value?.model ?? '' }),
    cta: t('site.sqPromoCta'),
  }
  const configured = settings.value.featured_copy?.[locale.value]
  const simplified = settings.value.featured_copy?.zh
  return {
    badge: configured?.badge || simplified?.badge || fallback.badge,
    title: configured?.title || simplified?.title || fallback.title,
    body: configured?.body || simplified?.body || fallback.body,
    cta: configured?.cta || simplified?.cta || fallback.cta,
  }
})
const unitHint = computed(() => unit.value === 'K' ? t('site.sqUnitHintThousand') : t('site.sqUnitHintMillion'))

function queryValue(key: string) {
  const value = route.query[key]
  return Array.isArray(value) ? value[0] ?? '' : value ?? ''
}

function validEnum<T extends string>(value: string, values: readonly T[], fallback: T): T {
  return (values as readonly string[]).includes(value) ? value as T : fallback
}

function readPage(value: string) {
  const parsed = Number.parseInt(value, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 1
}

function readList<T extends string>(value: string, allowed: readonly T[]): T[] {
  return [...new Set(value.split(',').map(item => item.trim()).filter(item => allowed.includes(item as T)))] as T[]
}

function applyQuery() {
  applyingQuery.value = true
  search.value = queryValue('q')
  vendor.value = queryValue('vendor') || FILTER_ALL
  group.value = queryValue('group') || FILTER_ALL
  inputModalities.value = readList(queryValue('input'), ['text', 'image', 'audio', 'video', 'file'] as const)
  outputModalities.value = readList(queryValue('output'), ['text', 'image', 'audio', 'video', 'file'] as const)
  contextBuckets.value = readList(queryValue('context'), ['64k', '128k', '256k', '1m', '1m-plus'] as const)
  sortBy.value = validEnum(queryValue('sort'), ['name', 'price-low', 'price-high'] as const, 'name')
  view.value = validEnum(queryValue('view'), ['card', 'table'] as const, view.value)
  unit.value = validEnum(queryValue('unit'), ['M', 'K'] as const, unit.value)
  page.value = readPage(queryValue('page'))
  nextTick(() => { applyingQuery.value = false })
}

function syncQuery() {
  if (!hydrated.value || applyingQuery.value) return
  const query: Record<string, string> = {}
  if (search.value.trim()) query.q = search.value.trim()
  if (vendor.value !== FILTER_ALL) query.vendor = vendor.value
  if (group.value !== FILTER_ALL) query.group = group.value
  if (inputModalities.value.length) query.input = inputModalities.value.join(',')
  if (outputModalities.value.length) query.output = outputModalities.value.join(',')
  if (contextBuckets.value.length) query.context = contextBuckets.value.join(',')
  if (sortBy.value !== 'name') query.sort = sortBy.value
  if (view.value !== 'card') query.view = view.value
  if (unit.value !== 'M') query.unit = unit.value
  if (page.value > 1) query.page = String(page.value)
  const model = queryValue('model')
  if (model) query.model = model
  router.replace({ query })
}

watch([search, vendor, group, inputModalities, outputModalities, contextBuckets, sortBy], () => { page.value = 1 })
watch(totalPages, (next) => { if (page.value > next) page.value = next })
watch([search, vendor, group, inputModalities, outputModalities, contextBuckets, sortBy, view, unit, page], syncQuery)
watch(() => route.query, () => {
  if (hydrated.value && !applyingQuery.value) applyQuery()
}, { deep: true })
watch(page, (next, previous) => {
  if (!pageReady.value || next === previous || !import.meta.client) return
  resultsAnchor.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
})
watch(filtered, () => {
  const visible = new Set(filtered.value.map(model => squareModelKey(model)))
  comparedKeys.value = comparedKeys.value.filter(key => visible.has(key))
})

function resetFilters() {
  search.value = ''
  vendor.value = FILTER_ALL
  group.value = FILTER_ALL
  inputModalities.value = []
  outputModalities.value = []
  contextBuckets.value = []
}

function openDetail(model: SquareModel) {
  selected.value = model
  detailOpen.value = true
  router.replace({ query: { ...route.query, model: model.model } })
}

function toggleCompare(model: SquareModel) {
  const key = squareModelKey(model)
  if (comparedKeys.value.includes(key)) {
    comparedKeys.value = comparedKeys.value.filter(item => item !== key)
    return
  }
  if (comparedKeys.value.length >= 3) return
  comparedKeys.value = [...comparedKeys.value, key]
}

watch(detailOpen, (open) => {
  if (!open && queryValue('model')) {
    const query = { ...route.query }
    delete query.model
    router.replace({ query })
    selected.value = null
  }
})

watch([loaded, models], ([isLoaded]) => {
  if (!isLoaded || !import.meta.client) return
  const modelName = queryValue('model')
  if (!modelName || detailOpen.value) return
  const match = models.value.find(item => item.model === modelName)
  if (match) {
    selected.value = match
    detailOpen.value = true
  }
})

onMounted(async () => {
  const storedView = localStorage.getItem(VIEW_PREF_KEY)
  const storedUnit = localStorage.getItem(UNIT_PREF_KEY)
  if (!queryValue('view') && (storedView === 'card' || storedView === 'table')) view.value = storedView
  if (!queryValue('unit') && (storedUnit === 'M' || storedUnit === 'K')) unit.value = storedUnit
  applyQuery()
  hydrated.value = true
  await loadCatalog()
  page.value = Math.min(page.value, totalPages.value)
  pageReady.value = true
})

watch([view, unit], () => {
  if (!import.meta.client) return
  localStorage.setItem(VIEW_PREF_KEY, view.value)
  localStorage.setItem(UNIT_PREF_KEY, unit.value)
})

onMounted(() => {
  const onKeydown = (event: KeyboardEvent) => {
    const target = event.target as HTMLElement | null
    if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey) return
    if (target?.matches('input, textarea, select, [contenteditable="true"]')) return
    event.preventDefault()
    toolbar.value?.focusSearch()
  }
  window.addEventListener('keydown', onKeydown)
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
})
</script>

<template>
  <div>
    <section class="shell !max-w-[120rem] border-b border-line pt-8 pb-5 sm:pt-10 sm:pb-6 md:pt-12 md:pb-7">
      <div class="flex flex-col gap-5 xl:flex-row xl:items-end xl:justify-between">
        <div class="min-w-0 xl:flex-1">
          <div class="flex items-baseline gap-2.5">
            <h1 class="text-2xl font-semibold tracking-tight text-ink md:text-3xl">{{ t('site.sqTitle') }}</h1>
            <span class="numeric text-xs text-faint">{{ t('site.sqResultCount', { count: models.length }) }}</span>
          </div>
          <p class="mt-2 max-w-2xl text-[13px] text-muted">{{ t('site.sqLead') }}</p>
        </div>
        <MarketplaceToolbar
          ref="toolbar"
          v-model:search="search"
          v-model:sort-by="sortBy"
          v-model:view="view"
          v-model:unit="unit"
          v-model:compare-mode="compareMode"
          :compare-count="comparedModels.length"
          class="xl:max-w-[52rem] xl:flex-1"
        />
      </div>
    </section>

    <section class="shell !max-w-[120rem] pb-20 pt-4 sm:pt-5 md:pb-24 md:pt-6">
      <div class="grid items-start gap-5 lg:grid-cols-[14rem_minmax(0,1fr)] lg:gap-8 xl:grid-cols-[15rem_minmax(0,1fr)] xl:gap-10">
        <MarketplaceFilterSidebar
            v-model:vendor="vendor"
            v-model:group="group"
            v-model:input-modalities="inputModalities"
            v-model:output-modalities="outputModalities"
            v-model:context-buckets="contextBuckets"
            :vendors="vendors"
            :groups="groups"
            :metadata-availability="metadataAvailability"
            @clear="resetFilters"
          />

        <div ref="resultsAnchor" class="w-full min-w-0 scroll-mt-28 lg:flex-1">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p class="text-sm font-medium text-ink">{{ t('site.sqResultsTitle') }}</p>
              <p class="mt-0.5 text-2xs text-faint">{{ unitHint }}</p>
            </div>
            <div class="flex items-center gap-2">
              <span v-if="comparedModels.length" class="rounded-control bg-clay-soft px-2.5 py-1 text-2xs text-clay">
                {{ t('site.sqCompareSelected', { count: comparedModels.length }) }}
              </span>
              <UiButton v-if="filtersActive" variant="ghost" size="sm" @click="resetFilters">{{ t('site.sqReset') }}</UiButton>
            </div>
          </div>

          <UiAlert v-if="error" tone="danger" :title="t('site.sqErrorTitle')" class="mt-5">
            {{ error }}
            <UiButton variant="link" size="sm" class="ml-1 h-auto p-0" @click="loadCatalog(true)">
              {{ t('common.retry') }}
            </UiButton>
          </UiAlert>

          <div v-else-if="!loaded" class="mt-5 grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
            <div v-for="index in 8" :key="index" class="min-h-[350px] rounded-card border border-line bg-surface p-5">
              <UiSkeleton :rows="7" />
            </div>
          </div>

          <UiEmptyState
            v-else-if="!models.length"
            class="mt-5 rounded-card border border-line bg-surface"
            :title="t('site.sqCatalogEmptyTitle')"
            :description="t('site.sqCatalogEmptyBody')"
          />

          <UiEmptyState
            v-else-if="!filtered.length"
            class="mt-5 rounded-card border border-line bg-surface"
            :icon="SearchX"
            :title="t('site.sqEmptyTitle')"
            :description="t('site.sqEmptyBody')"
          >
            <UiButton variant="secondary" size="sm" @click="resetFilters">{{ t('site.sqReset') }}</UiButton>
          </UiEmptyState>

          <template v-else>
            <div v-if="view === 'card'" class="mt-5 grid min-w-0 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
              <MarketplacePromoBanner v-if="promoVisible" :model="featuredModel" :copy="featuredCopy" @select="openDetail" />
              <MarketplaceModelCard
                v-for="model in paged"
                :key="squareModelKey(model)"
                :model="model"
                :group="group"
                :unit="unit"
                :compare-mode="compareMode"
                :compared="comparedKeys.includes(squareModelKey(model))"
                @select="openDetail"
                @toggle-compare="toggleCompare"
              />
            </div>

            <div v-else class="mt-5">
              <MarketplaceModelTable :models="paged" :group="group" :unit="unit" @select="openDetail" />
            </div>

            <div class="mt-8 flex flex-wrap items-center justify-between gap-3">
              <span v-if="totalPages <= 1" class="text-2xs text-faint">{{ t('site.sqResultCount', { count: listedModels.length }) }}</span>
              <MarketplacePagination v-model="page" :total-pages="totalPages" :total="listedModels.length" />
            </div>
          </template>
        </div>
      </div>
    </section>

    <MarketplaceModelDialog v-model:open="detailOpen" :model="selected" :unit="unit" />
  </div>
</template>
