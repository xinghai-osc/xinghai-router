<script setup lang="ts">
import { ArrowUpDown, Check, Filter, Grid2X2, Table2 } from 'lucide-vue-next'
import { computed } from 'vue'
import type { SortOption, TokenUnit, ViewMode } from '~/src/marketplace'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'

const props = defineProps<{
  filteredCount: number
  totalCount: number
  hasActiveFilters: boolean
  activeFilterCount: number
  sortBy: SortOption
  tokenUnit: TokenUnit
  viewMode: ViewMode
}>()
const emit = defineEmits<{
  'update:sortBy': [value: SortOption]
  'update:tokenUnit': [value: TokenUnit]
  'update:viewMode': [value: ViewMode]
  openFilters: []
}>()
const { t } = useI18n()

const sortLabels = computed<Record<SortOption, string>>(() => ({
  name: t('msSortName'),
  'price-low': t('msSortPriceLow'),
  'price-high': t('msSortPriceHigh'),
}))

function chooseSort(value: SortOption) {
  emit('update:sortBy', value)
}

const segmentedBtn = 'inline-flex items-center justify-center px-3 py-1.5 text-xs font-medium transition-colors'
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-3 py-3">
    <div class="flex items-center gap-3">
      <Button variant="outline" size="sm" class="lg:hidden" @click="emit('openFilters')">
        <Filter :size="15" />{{ t('msFilter') }}
        <Badge v-if="props.activeFilterCount > 0" variant="secondary" class="ml-1 h-4 px-1 text-[10px]">{{ props.activeFilterCount }}</Badge>
      </Button>
      <div class="text-sm">
        <strong class="font-semibold">{{ props.filteredCount.toLocaleString() }}</strong>
        <span class="ml-1 text-muted-foreground">{{ t('msModels') }}</span>
        <small v-if="props.hasActiveFilters && props.totalCount" class="ml-1 text-muted-foreground">/ {{ props.totalCount.toLocaleString() }}</small>
      </div>
    </div>

    <div class="flex items-center gap-2">
      <div class="hidden items-center gap-0 rounded-md border border-border sm:inline-flex" role="group" :aria-label="t('msTokenUnit')">
        <button type="button" :class="[segmentedBtn, 'rounded-l-md', props.tokenUnit === 'M' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent/50']" :aria-pressed="props.tokenUnit === 'M'" @click="emit('update:tokenUnit', 'M')">/1M</button>
        <button type="button" :class="[segmentedBtn, 'rounded-r-md border-l border-border', props.tokenUnit === 'K' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent/50']" :aria-pressed="props.tokenUnit === 'K'" @click="emit('update:tokenUnit', 'K')">/1K</button>
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button variant="outline" size="sm">
            <ArrowUpDown :size="13" /><span>{{ sortLabels[props.sortBy] || t('msSort') }}</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuItem v-for="(label, value) in sortLabels" :key="value" @select="chooseSort(value as SortOption)">
            <Check :size="14" :class="props.sortBy === value ? 'opacity-100' : 'opacity-0'" />{{ label }}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <div class="flex items-center gap-0 rounded-md border border-border" role="group" :aria-label="t('msViewMode')">
        <button type="button" :class="[segmentedBtn, 'rounded-l-md', props.viewMode === 'card' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent/50']" :title="t('msCardView')" :aria-pressed="props.viewMode === 'card'" @click="emit('update:viewMode', 'card')">
          <Grid2X2 :size="13" />
        </button>
        <button type="button" :class="[segmentedBtn, 'rounded-r-md border-l border-border', props.viewMode === 'table' ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:bg-accent/50']" :title="t('msTableView')" :aria-pressed="props.viewMode === 'table'" @click="emit('update:viewMode', 'table')">
          <Table2 :size="13" />
        </button>
      </div>
    </div>
  </div>
</template>
