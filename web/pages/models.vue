<script setup lang="ts">
import { SearchX } from 'lucide-vue-next'
import {
  extractVendors, filterAndSort, FILTER_ALL, PAGE_SIZE,
  type SortOption, type SquareModel, type TokenUnit, type ViewMode,
} from '~/src/marketplace'

const { t } = useI18n()
const { settings } = useSiteSettings()
const { models, groups, loading, loaded, error, loadCatalog } = useCatalog()

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

watch([search, vendor, group, sortBy], () => { page.value = 1 })
watch(totalPages, (next) => { if (page.value > next) page.value = next })

function resetFilters() {
  search.value = ''
  vendor.value = FILTER_ALL
  group.value = FILTER_ALL
}

function openDetail(model: SquareModel) {
  selected.value = model
  detailOpen.value = true
}

onMounted(() => loadCatalog())
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

      <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
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
            :key="model.id"
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
