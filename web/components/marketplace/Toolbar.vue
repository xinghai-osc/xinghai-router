<script setup lang="ts">
import { GitCompareArrows, LayoutGrid, Rows3, Search } from 'lucide-vue-next'
import type { SortOption, TokenUnit, ViewMode } from '~/src/marketplace'

const search = defineModel<string>('search', { required: true })
const sortBy = defineModel<SortOption>('sortBy', { required: true })
const view = defineModel<ViewMode>('view', { required: true })
const unit = defineModel<TokenUnit>('unit', { required: true })
const compareMode = defineModel<boolean>('compareMode', { default: false })

defineProps<{ compareCount?: number }>()

const { t } = useI18n()
const searchShell = ref<HTMLElement | null>(null)

function focusSearch() {
  searchShell.value?.querySelector<HTMLInputElement>('input')?.focus()
}

defineExpose({ focusSearch })

const sortOptions = computed(() => [
  { value: 'name', label: t('site.sqSortName') },
  { value: 'price-low', label: t('site.sqSortPriceLow') },
  { value: 'price-high', label: t('site.sqSortPriceHigh') },
])

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
  <div class="flex min-w-0 flex-col gap-2.5 sm:flex-row sm:items-start sm:justify-end">
    <div ref="searchShell" class="min-w-0 flex-1 sm:w-auto sm:max-w-md">
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

    <div class="flex min-w-0 flex-wrap items-center justify-end gap-2">
      <button
        type="button"
        class="inline-flex h-9 shrink-0 items-center gap-1.5 rounded-control border px-3 text-[13px] transition-colors duration-150"
        :class="compareMode ? 'border-clay bg-clay-soft text-clay' : 'border-line-strong bg-surface text-muted hover:border-faint hover:text-ink'"
        :aria-pressed="compareMode"
        :title="t('site.sqCompareHint')"
        @click="compareMode = !compareMode"
      >
        <GitCompareArrows class="size-3.5" />
        <span class="hidden sm:inline">{{ t('site.sqCompare') }}</span>
        <span v-if="compareCount" class="numeric text-2xs">{{ compareCount }}</span>
      </button>

      <div class="w-28 shrink-0 sm:w-32">
        <label for="sq-sort" class="sr-only">{{ t('site.sqSortLabel') }}</label>
        <UiSelect id="sq-sort" v-model="sortProxy" :options="sortOptions" size="sm" />
      </div>

      <div :class="[SEGMENT, 'shrink-0']" role="group" :aria-label="t('site.sqUnitLabel')">
        <button
          v-for="item in units"
          :key="item.value"
          type="button"
          :class="[SEGMENT_ITEM, 'numeric px-2 py-1.5 text-[12px]', unit === item.value ? 'bg-sunken text-ink' : 'text-muted hover:text-ink']"
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
