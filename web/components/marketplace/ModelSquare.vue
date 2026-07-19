<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Search } from 'lucide-vue-next'
import type { CatalogGroup, CatalogModel } from '~/src/api'
import { FILTER_ALL, PAGE_SIZE, filterAndSort, toSquareModel, type SortOption, type SquareModel, type TokenUnit, type ViewMode } from '~/src/marketplace'
import SquareSearch from './SquareSearch.vue'
import SquareSidebar from './SquareSidebar.vue'
import SquareToolbar from './SquareToolbar.vue'
import SquareCard from './SquareCard.vue'
import SquareTable from './SquareTable.vue'
import SquarePagination from './SquarePagination.vue'
import SquareDrawer from './SquareDrawer.vue'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'

const props = defineProps<{
  catalog: CatalogModel[]
  groups: CatalogGroup[]
  loaded: boolean
}>()
const { t } = useI18n()

const models = computed(() => props.catalog.map(toSquareModel))

const searchInput = ref('')
const sortBy = ref<SortOption>('name')
const vendorFilter = ref(FILTER_ALL)
const groupFilter = ref(FILTER_ALL)
const quotaTypeFilter = ref('all')
const tokenUnit = ref<TokenUnit>('M')
const viewMode = ref<ViewMode>('card')
const page = ref(1)
const mobileFiltersOpen = ref(false)
const selectedModel = ref<SquareModel | null>(null)

const filteredModels = computed(() =>
  filterAndSort(models.value, {
    search: searchInput.value,
    vendor: vendorFilter.value,
    group: groupFilter.value,
    quotaType: quotaTypeFilter.value,
    sortBy: sortBy.value,
  }),
)

const hasActiveFilters = computed(() => vendorFilter.value !== FILTER_ALL || groupFilter.value !== FILTER_ALL || quotaTypeFilter.value !== 'all')
const activeFilterCount = computed(() => (vendorFilter.value !== FILTER_ALL ? 1 : 0) + (groupFilter.value !== FILTER_ALL ? 1 : 0) + (quotaTypeFilter.value !== 'all' ? 1 : 0))

const totalPages = computed(() => Math.max(1, Math.ceil(filteredModels.value.length / PAGE_SIZE)))
const currentPage = computed(() => Math.min(page.value, totalPages.value))
const pagedModels = computed(() => filteredModels.value.slice((currentPage.value - 1) * PAGE_SIZE, currentPage.value * PAGE_SIZE))

watch([searchInput, vendorFilter, groupFilter, quotaTypeFilter, sortBy], () => { page.value = 1 })

function clearFilters() {
  vendorFilter.value = FILTER_ALL
  groupFilter.value = FILTER_ALL
  quotaTypeFilter.value = 'all'
}
function clearAll() {
  clearFilters()
  searchInput.value = ''
}

const origin = computed(() => (import.meta.client ? window.location.origin : ''))
const skeletonCards = Array.from({ length: 6 }, (_, index) => index)
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <div class="mx-auto w-full max-w-7xl px-4 py-8 sm:px-6 sm:py-12">
      <header class="mb-8">
        <h1 class="text-3xl font-bold tracking-tight sm:text-4xl">{{ t('marketplace') }}</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ t('msSubtitle1a') }}{{ props.catalog.length }}{{ t('msSubtitle1b') }}</p>
        <p class="text-sm text-muted-foreground">{{ t('msSubtitle2') }}</p>
        <SquareSearch v-model="searchInput" class="msq-header-search mt-4" @clear="searchInput = ''" />
      </header>

      <div v-if="!props.loaded" class="flex flex-col gap-4">
        <Skeleton class="h-10 w-full" />
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="item in skeletonCards" :key="item" class="rounded-lg border border-border p-4">
            <div class="flex items-center gap-3">
              <Skeleton class="h-10 w-10 rounded-lg" />
              <div class="flex-1 space-y-2"><Skeleton class="h-4 w-2/3" /><Skeleton class="h-3 w-1/2" /></div>
            </div>
            <Skeleton class="mt-3 h-3 w-full" />
            <Skeleton class="mt-2 h-3 w-3/4" />
          </div>
        </div>
      </div>

      <div v-else class="grid gap-6 lg:grid-cols-[260px_1fr]">
        <SquareSidebar
          v-model:vendor-filter="vendorFilter"
          v-model:group-filter="groupFilter"
          v-model:quota-type-filter="quotaTypeFilter"
          :models="models"
          :groups="props.groups"
          :has-active-filters="hasActiveFilters"
          class="hidden lg:block"
          @clear="clearFilters"
        />

        <main>
          <SquareToolbar
            v-model:sort-by="sortBy"
            v-model:token-unit="tokenUnit"
            v-model:view-mode="viewMode"
            :filtered-count="filteredModels.length"
            :total-count="models.length"
            :has-active-filters="hasActiveFilters"
            :active-filter-count="activeFilterCount"
            @open-filters="mobileFiltersOpen = true"
          />

          <div v-if="!filteredModels.length" class="flex flex-col items-center justify-center gap-3 py-16 text-center">
            <Search :size="38" class="text-muted-foreground" />
            <h3 class="text-lg font-semibold">{{ t('msEmptyTitle') }}</h3>
            <p class="text-sm text-muted-foreground">{{ searchInput.trim() ? `${t('msEmptySearch1')}${searchInput}${t('msEmptySearch2')}` : t('msEmptyFilter') }}</p>
            <Button v-if="hasActiveFilters || searchInput.trim()" variant="outline" @click="clearAll">
              {{ t('msClearFilters') }}
            </Button>
          </div>

          <template v-else-if="viewMode === 'card'">
            <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              <SquareCard
                v-for="model in pagedModels"
                :key="model.model"
                :model="model"
                :token-unit="tokenUnit"
                :selected-group="groupFilter"
                @open="selectedModel = $event"
              />
            </div>
            <SquarePagination v-model:page="page" :total-pages="totalPages" />
          </template>

          <SquareTable
            v-else
            v-model:page="page"
            :models="pagedModels"
            :token-unit="tokenUnit"
            :selected-group="groupFilter"
            :total-pages="totalPages"
            @open="selectedModel = $event"
          />
        </main>
      </div>
    </div>

    <Dialog :open="mobileFiltersOpen" @update:open="v => !v && (mobileFiltersOpen = false)">
      <DialogContent class="sm:max-w-md">
        <DialogTitle class="sr-only">{{ t('msFilter') }}</DialogTitle>
        <SquareSidebar
          v-model:vendor-filter="vendorFilter"
          v-model:group-filter="groupFilter"
          v-model:quota-type-filter="quotaTypeFilter"
          :models="models"
          :groups="props.groups"
          :has-active-filters="hasActiveFilters"
          bare
          @clear="clearFilters"
        />
      </DialogContent>
    </Dialog>

    <SquareDrawer
      v-if="selectedModel"
      :model="selectedModel"
      :token-unit="tokenUnit"
      :origin="origin"
      @close="selectedModel = null"
    />
  </div>
</template>
