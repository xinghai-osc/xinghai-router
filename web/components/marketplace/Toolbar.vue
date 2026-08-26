<script setup lang="ts">
import { LayoutGrid, Rows3, Search } from 'lucide-vue-next'
import type { CatalogGroup } from '~/src/api'
import { FILTER_ALL, type SortOption, type TokenUnit, type ViewMode } from '~/src/marketplace'

const search = defineModel<string>('search', { required: true })
const vendor = defineModel<string>('vendor', { required: true })
const group = defineModel<string>('group', { required: true })
const sortBy = defineModel<SortOption>('sortBy', { required: true })
const view = defineModel<ViewMode>('view', { required: true })
const unit = defineModel<TokenUnit>('unit', { required: true })

const props = defineProps<{
  vendors: { name: string; slug: string; count: number }[]
  groups: CatalogGroup[]
}>()

const { t } = useI18n()
const searchShell = ref<HTMLElement | null>(null)

function focusSearch() {
  searchShell.value?.querySelector<HTMLInputElement>('input')?.focus()
}

defineExpose({ focusSearch })

const vendorOptions = computed(() => [
  { value: FILTER_ALL, label: t('site.sqAllVendors') },
  ...props.vendors.map(item => ({ value: item.name, label: `${item.name} · ${item.count}` })),
])

const groupOptions = computed(() => [
  { value: FILTER_ALL, label: t('site.sqAllGroups') },
  ...props.groups.map(item => ({ value: item.id, label: item.name })),
])

const sortOptions = computed(() => [
  { value: 'name', label: t('site.sqSortName') },
  { value: 'price-low', label: t('site.sqSortPriceLow') },
  { value: 'price-high', label: t('site.sqSortPriceHigh') },
])

// UiSelect models a plain string; the union type stays on the page side.
const sortProxy = computed({
  get: () => sortBy.value as string,
  set: (value: string) => { sortBy.value = value as SortOption },
})

const views: { value: ViewMode; labelKey: string; icon: typeof LayoutGrid }[] = [
  { value: 'card', labelKey: 'site.sqViewCard', icon: LayoutGrid },
  { value: 'table', labelKey: 'site.sqViewTable', icon: Rows3 },
]

const units: { value: TokenUnit; labelKey: string }[] = [
  { value: 'M', labelKey: 'site.sqUnitMillion' },
  { value: 'K', labelKey: 'site.sqUnitThousand' },
]

const SEGMENT = 'inline-flex items-center gap-0.5 rounded-control border border-line-strong bg-surface p-0.5'
const SEGMENT_ITEM = 'rounded-[7px] transition-colors duration-150'
</script>

<template>
  <div class="flex flex-col gap-3 lg:flex-row lg:items-center">
    <div ref="searchShell" class="lg:max-w-xs lg:flex-1">
      <label for="sq-search" class="sr-only">{{ t('site.sqSearchPlaceholder') }}</label>
      <UiInput id="sq-search" v-model="search" :placeholder="t('site.sqSearchPlaceholder')">
        <template #leading>
          <Search class="size-4" />
        </template>
        <template #trailing>
          <kbd class="rounded border border-line px-1.5 py-0.5 font-mono text-[10px] text-faint" :title="t('site.sqSearchShortcutHint')">/</kbd>
        </template>
      </UiInput>
    </div>

    <div class="flex min-w-0 flex-wrap items-center gap-2 lg:ml-auto">
      <div class="min-w-0 flex-1 basis-36 lg:w-40 lg:flex-none">
        <label for="sq-vendor" class="sr-only">{{ t('site.sqVendorLabel') }}</label>
        <UiSelect id="sq-vendor" v-model="vendor" :options="vendorOptions" />
      </div>
      <div class="min-w-0 flex-1 basis-32 lg:w-36 lg:flex-none">
        <label for="sq-group" class="sr-only">{{ t('site.sqGroupLabel') }}</label>
        <UiSelect id="sq-group" v-model="group" :options="groupOptions" />
      </div>
      <div class="min-w-0 flex-1 basis-36 lg:w-40 lg:flex-none">
        <label for="sq-sort" class="sr-only">{{ t('site.sqSortLabel') }}</label>
        <UiSelect id="sq-sort" v-model="sortProxy" :options="sortOptions" />
      </div>

      <div :class="[SEGMENT, 'shrink-0']" role="group" :aria-label="t('site.sqUnitLabel')">
        <button
          v-for="item in units"
          :key="item.value"
          type="button"
          :class="[SEGMENT_ITEM, 'numeric px-2.5 py-1.5 text-[13px]', unit === item.value ? 'bg-sunken text-ink' : 'text-muted hover:text-ink']"
          :aria-pressed="unit === item.value"
          @click="unit = item.value"
        >{{ t(item.labelKey) }}</button>
      </div>

      <div :class="[SEGMENT, 'shrink-0']" role="group" :aria-label="t('site.sqViewLabel')">
        <button
          v-for="item in views"
          :key="item.value"
          type="button"
          :class="[SEGMENT_ITEM, 'flex p-2', view === item.value ? 'bg-sunken text-ink' : 'text-muted hover:text-ink']"
          :aria-pressed="view === item.value"
          :aria-label="t(item.labelKey)"
          :title="t(item.labelKey)"
          @click="view = item.value"
        >
          <component :is="item.icon" class="size-4" />
        </button>
      </div>
    </div>
  </div>
</template>
