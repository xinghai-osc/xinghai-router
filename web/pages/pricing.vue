<script setup lang="ts">
import { Search, SearchX } from 'lucide-vue-next'
import { filterAndSort, FILTER_ALL, PAGE_SIZE, type SquareModel, type TokenUnit } from '~/src/marketplace'

const { t } = useI18n()
const { settings } = useSiteSettings()
const { plans, loading: plansLoading, error: plansError, loadPlans } = usePlans()
const { models, groups, loading: catalogLoading, loaded: catalogLoaded, error: catalogError, loadCatalog } = useCatalog()

useHead({
  title: () => `${t('site.pgMetaTitle')} · ${settings.value.name}`,
  meta: [{ name: 'description', content: () => t('site.pgMetaDescription') }],
})

const search = ref('')
const group = ref(FILTER_ALL)
const unit = ref<TokenUnit>('M')
const page = ref(1)

const detailOpen = ref(false)
const selected = ref<SquareModel | null>(null)

const groupOptions = computed(() => [
  { value: FILTER_ALL, label: t('site.sqAllGroups') },
  ...groups.value.map(item => ({ value: item.id, label: item.name })),
])

const filtered = computed(() => filterAndSort(models.value, {
  search: search.value,
  vendor: FILTER_ALL,
  group: group.value,
  sortBy: 'name',
}))

const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)))
const paged = computed(() => filtered.value.slice((page.value - 1) * PAGE_SIZE, page.value * PAGE_SIZE))

const unitHint = computed(() => unit.value === 'K' ? t('site.sqUnitHintThousand') : t('site.sqUnitHintMillion'))

const notes = ['site.pgNote1', 'site.pgNote2', 'site.pgNote3', 'site.pgNote4']

const UNITS: { value: TokenUnit; labelKey: string }[] = [
  { value: 'M', labelKey: 'site.sqUnitMillion' },
  { value: 'K', labelKey: 'site.sqUnitThousand' },
]

watch([search, group], () => { page.value = 1 })
watch(totalPages, (next) => { if (page.value > next) page.value = next })

function openDetail(model: SquareModel) {
  selected.value = model
  detailOpen.value = true
}

onMounted(() => {
  loadPlans()
  loadCatalog()
})
</script>

<template>
  <div>
    <section class="shell pt-16 pb-14 md:pt-20">
      <div class="max-w-2xl space-y-3">
        <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.pricingEyebrow') }}</p>
        <h1 class="display text-4xl text-ink md:text-5xl">{{ t('site.pricingTitle') }}</h1>
        <p class="text-muted">{{ t('site.pricingLead') }}</p>
      </div>

      <UiAlert v-if="plansError" tone="danger" :title="t('common.loadFailed')" class="mt-10">
        {{ plansError }}
        <UiButton variant="link" size="sm" class="ml-1 h-auto p-0" @click="loadPlans(true)">
          {{ t('common.retry') }}
        </UiButton>
      </UiAlert>

      <div v-else class="mt-12">
        <SitePlanCards :plans="plans" :loading="plansLoading" />
      </div>
    </section>

    <section class="border-y border-line bg-sunken/50">
      <div class="shell py-16 md:py-20">
        <h2 class="text-[15px] font-semibold text-ink">{{ t('site.pgNoteTitle') }}</h2>
        <ul class="mt-5 grid gap-4 md:grid-cols-2">
          <li
            v-for="(note, index) in notes"
            :key="note"
            class="flex gap-3 rounded-card border border-line bg-surface px-5 py-4"
          >
            <span class="numeric text-2xs text-clay">{{ String(index + 1).padStart(2, '0') }}</span>
            <p class="text-[13px] leading-relaxed text-muted">{{ t(note) }}</p>
          </li>
        </ul>
      </div>
    </section>

    <section class="shell py-20 md:py-24">
      <div class="max-w-2xl space-y-3">
        <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.pgTableEyebrow') }}</p>
        <h2 class="display text-4xl text-ink md:text-5xl">{{ t('site.pgTableTitle') }}</h2>
        <p class="text-muted">{{ t('site.pgTableLead') }}</p>
      </div>

      <div class="mt-10 flex flex-col gap-3 sm:flex-row sm:items-center">
        <div class="sm:max-w-xs sm:flex-1">
          <label for="pg-search" class="sr-only">{{ t('site.pgSearchPlaceholder') }}</label>
          <UiInput id="pg-search" v-model="search" :placeholder="t('site.pgSearchPlaceholder')">
            <template #leading>
              <Search class="size-4" />
            </template>
          </UiInput>
        </div>

        <div class="flex items-center gap-2 sm:ml-auto">
          <div class="w-40">
            <label for="pg-group" class="sr-only">{{ t('site.sqGroupLabel') }}</label>
            <UiSelect id="pg-group" v-model="group" :options="groupOptions" />
          </div>

          <div
            class="inline-flex items-center gap-0.5 rounded-control border border-line-strong bg-surface p-0.5"
            role="group"
            :aria-label="t('site.sqUnitLabel')"
          >
            <button
              v-for="item in UNITS"
              :key="item.value"
              type="button"
              :class="[
                'numeric rounded-[7px] px-2.5 py-1.5 text-[13px] transition-colors duration-150',
                unit === item.value ? 'bg-sunken text-ink' : 'text-muted hover:text-ink',
              ]"
              :aria-pressed="unit === item.value"
              @click="unit = item.value"
            >{{ t(item.labelKey) }}</button>
          </div>
        </div>
      </div>

      <div class="mt-4 flex flex-wrap items-center justify-between gap-3">
        <p class="numeric text-2xs text-faint">{{ t('site.sqResultCount', { count: filtered.length }) }}</p>
        <p class="text-2xs text-faint">{{ unitHint }}</p>
      </div>

      <UiAlert v-if="catalogError" tone="danger" :title="t('site.sqErrorTitle')" class="mt-6">
        {{ catalogError }}
        <UiButton variant="link" size="sm" class="ml-1 h-auto p-0" @click="loadCatalog(true)">
          {{ t('common.retry') }}
        </UiButton>
      </UiAlert>

      <div v-else-if="catalogLoading && !catalogLoaded" class="mt-6 rounded-card border border-line bg-surface p-5">
        <UiSkeleton :rows="10" />
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
        <UiButton variant="secondary" size="sm" @click="search = ''">{{ t('site.sqReset') }}</UiButton>
      </UiEmptyState>

      <template v-else>
        <div class="mt-6">
          <MarketplaceModelTable :models="paged" :group="group" :unit="unit" @select="openDetail" />
        </div>
        <MarketplacePagination v-model="page" :total-pages="totalPages" :total="filtered.length" class="mt-8" />
      </template>
    </section>

    <MarketplaceModelDialog v-model:open="detailOpen" :model="selected" :unit="unit" />

    <SiteCtaBand />
  </div>
</template>
