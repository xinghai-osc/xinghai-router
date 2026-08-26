<script setup lang="ts">
import { SearchX } from 'lucide-vue-next'
import {
  extractVendors, filterAndSort, FILTER_ALL, PAGE_SIZE, squareModelKey,
  type SortOption, type SquareModel, type TokenUnit, type ViewMode,
} from '~/src/marketplace'

const { t } = useI18n()
const { settings } = useSiteSettings()
const { models, groups, loading, loaded, error, loadCatalog } = useCatalog()
const route = useRoute()
const router = useRouter()

useHead({
  title: () => `${t('site.sqMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.sqMetaDescription') }],
})

const search = ref('')
const vendor = ref(FILTER_ALL)
const group = ref(FILTER_ALL)
const sortBy = ref<SortOption>('name')
const view = ref<ViewMode>('card')
const unit = ref<TokenUnit>('M')
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

const filtered = computed(() => filterAndSort(models.value, {
  search: search.value,
  vendor: vendor.value,
  group: group.value,
  sortBy: sortBy.value,
}))

const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)))
const paged = computed(() => filtered.value.slice((page.value - 1) * PAGE_SIZE, page.value * PAGE_SIZE))

const filtersActive = computed(() =>
  Boolean(search.value) || vendor.value !== FILTER_ALL || group.value !== FILTER_ALL)

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

function applyQuery() {
  applyingQuery.value = true
  search.value = queryValue('q')
  vendor.value = queryValue('vendor') || FILTER_ALL
  group.value = queryValue('group') || FILTER_ALL
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
  if (sortBy.value !== 'name') query.sort = sortBy.value
  if (view.value !== 'card') query.view = view.value
  if (unit.value !== 'M') query.unit = unit.value
  if (page.value > 1) query.page = String(page.value)
  const model = queryValue('model')
  if (model) query.model = model
  router.replace({ query })
}

watch([search, vendor, group, sortBy], () => { page.value = 1 })
watch(totalPages, (next) => { if (page.value > next) page.value = next })
watch([search, vendor, group, sortBy, view, unit, page], syncQuery)
watch(() => route.query, () => {
  if (hydrated.value && !applyingQuery.value) applyQuery()
}, { deep: true })
watch(page, (next, previous) => {
  if (!pageReady.value || next === previous || !import.meta.client) return
  resultsAnchor.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
})

function resetFilters() {
  search.value = ''
  vendor.value = FILTER_ALL
  group.value = FILTER_ALL
}

function openDetail(model: SquareModel) {
  selected.value = model
  detailOpen.value = true
  router.replace({ query: { ...route.query, model: model.model } })
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
    <section class="shell pt-16 pb-10 md:pt-20">
      <div class="max-w-2xl space-y-3">
        <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.sqEyebrow') }}</p>
        <h1 class="display text-4xl text-ink md:text-5xl">{{ t('site.sqTitle') }}</h1>
        <p class="text-muted">{{ t('site.sqLead') }}</p>
      </div>
    </section>

    <section class="shell pb-24">
      <div class="sticky top-16 z-20 -mx-5 border-y border-line bg-paper/90 px-5 py-3 backdrop-blur-md md:-mx-8 md:px-8">
        <MarketplaceToolbar
          ref="toolbar"
          v-model:search="search"
          v-model:vendor="vendor"
          v-model:group="group"
          v-model:sort-by="sortBy"
          v-model:view="view"
          v-model:unit="unit"
          :vendors="vendors"
          :groups="groups"
        />
      </div>

      <MarketplaceVendorChips v-model="vendor" :vendors="vendors" class="mt-4" />

      <div ref="resultsAnchor" class="mt-4 scroll-mt-32 flex flex-wrap items-center justify-between gap-3">
        <p class="numeric text-2xs text-faint">{{ t('site.sqResultCount', { count: filtered.length }) }}</p>
        <p class="text-2xs text-faint">{{ unitHint }}</p>
      </div>

      <UiAlert v-if="error" tone="danger" :title="t('site.sqErrorTitle')" class="mt-6">
        {{ error }}
        <UiButton variant="link" size="sm" class="ml-1 h-auto p-0" @click="loadCatalog(true)">
          {{ t('common.retry') }}
        </UiButton>
      </UiAlert>

      <div v-else-if="loading && !loaded" class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="index in 9" :key="index" class="rounded-card border border-line bg-surface p-5">
          <UiSkeleton :rows="4" />
        </div>
      </div>

      <UiEmptyState
        v-else-if="!models.length"
        class="mt-6 rounded-card border border-line bg-surface"
        :title="t('site.sqCatalogEmptyTitle')"
        :description="t('site.sqCatalogEmptyBody')"
      />

      <UiEmptyState
        v-else-if="!filtered.length"
        class="mt-6 rounded-card border border-line bg-surface"
        :icon="SearchX"
        :title="t('site.sqEmptyTitle')"
        :description="t('site.sqEmptyBody')"
      >
        <UiButton variant="secondary" size="sm" @click="resetFilters">{{ t('site.sqReset') }}</UiButton>
      </UiEmptyState>

      <template v-else>
        <div v-if="view === 'card'" class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <MarketplaceModelCard
            v-for="model in paged"
            :key="squareModelKey(model)"
            :model="model"
            :group="group"
            :unit="unit"
            @select="openDetail"
          />
        </div>

        <div v-else class="mt-6">
          <MarketplaceModelTable :models="paged" :group="group" :unit="unit" @select="openDetail" />
        </div>

        <div class="mt-8 flex items-center justify-between gap-3">
          <UiButton v-if="filtersActive" variant="ghost" size="sm" @click="resetFilters">
            {{ t('site.sqReset') }}
          </UiButton>
          <MarketplacePagination v-model="page" :total-pages="totalPages" :total="filtered.length" class="flex-1" />
        </div>
      </template>
    </section>

    <MarketplaceModelDialog v-model:open="detailOpen" :model="selected" :unit="unit" />
  </div>
</template>
