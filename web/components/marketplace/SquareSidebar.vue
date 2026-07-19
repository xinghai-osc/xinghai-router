<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronDown, RotateCcw } from 'lucide-vue-next'
import type { CatalogGroup } from '~/src/api'
import { FILTER_ALL, extractVendors, formatRatio, vendorIconUrl, type SquareModel } from '~/src/marketplace'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

const props = defineProps<{
  models: SquareModel[]
  groups: CatalogGroup[]
  vendorFilter: string
  groupFilter: string
  quotaTypeFilter: string
  hasActiveFilters: boolean
  bare?: boolean
}>()
const emit = defineEmits<{
  'update:vendorFilter': [value: string]
  'update:groupFilter': [value: string]
  'update:quotaTypeFilter': [value: string]
  clear: []
}>()
const { t } = useI18n()

const vendors = computed(() => extractVendors(props.models))
const iconErrors = ref(new Set<string>())
function iconFailed(slug: string) {
  iconErrors.value = new Set(iconErrors.value).add(slug)
}

const collapsed = ref<Record<string, boolean>>({})
function toggleSection(key: string) {
  collapsed.value = { ...collapsed.value, [key]: !collapsed.value[key] }
}

const quotaOptions = computed(() => [
  { value: 'all', label: t('msAllModels'), count: props.models.length },
  { value: 'token', label: t('msTokenBased'), count: props.models.length },
])

const chipClass = 'inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium transition-colors'
const chipActive = 'border-primary bg-primary text-primary-foreground'
const chipIdle = 'border-border bg-card text-muted-foreground hover:bg-accent'
</script>

<template>
  <aside :class="['flex flex-col gap-3', props.bare ? '' : 'rounded-lg border border-border bg-card p-4']">
    <div class="flex items-start justify-between gap-2">
      <div>
        <h2 class="text-sm font-semibold">{{ t('msFilter') }}</h2>
        <p class="text-xs text-muted-foreground">{{ t('msFilterDesc') }}</p>
      </div>
      <Button variant="ghost" size="sm" :disabled="!props.hasActiveFilters" @click="emit('clear')">
        <RotateCcw :size="13" />{{ t('msReset') }}
      </Button>
    </div>
    <Badge v-if="props.hasActiveFilters" variant="secondary" class="w-fit">{{ t('msFiltersActive') }}</Badge>

    <div class="flex flex-col gap-3">
      <section class="flex flex-col gap-2">
        <button type="button" class="flex items-center justify-between text-sm font-medium" @click="toggleSection('group')">
          <span>{{ t('msGroups') }}</span>
          <ChevronDown :size="15" class="text-muted-foreground transition-transform" :class="{ '-rotate-90': collapsed.group }" />
        </button>
        <div v-show="!collapsed.group" class="flex flex-wrap gap-1.5">
          <button type="button" :class="[chipClass, props.groupFilter === FILTER_ALL ? chipActive : chipIdle]" @click="emit('update:groupFilter', FILTER_ALL)">
            <span>{{ t('msAllGroups') }}</span>
          </button>
          <button v-for="group in props.groups" :key="group.id" type="button" :class="[chipClass, props.groupFilter === group.id ? chipActive : chipIdle]" :title="group.name" @click="emit('update:groupFilter', group.id)">
            <span>{{ group.name }}</span>
            <span class="text-[10px] opacity-70">{{ formatRatio(Number(group.multiplier)) }}</span>
          </button>
        </div>
      </section>

      <section class="flex flex-col gap-2">
        <button type="button" class="flex items-center justify-between text-sm font-medium" @click="toggleSection('vendor')">
          <span>{{ t('msVendors') }}</span>
          <ChevronDown :size="15" class="text-muted-foreground transition-transform" :class="{ '-rotate-90': collapsed.vendor }" />
        </button>
        <div v-show="!collapsed.vendor" class="flex flex-wrap gap-1.5">
          <button type="button" :class="[chipClass, props.vendorFilter === FILTER_ALL ? chipActive : chipIdle]" @click="emit('update:vendorFilter', FILTER_ALL)">
            <span>{{ t('msAllVendors') }}</span>
            <span class="text-[10px] opacity-70">{{ props.models.length }}</span>
          </button>
          <button v-for="vendor in vendors" :key="vendor.name" type="button" :class="[chipClass, props.vendorFilter === vendor.name ? chipActive : chipIdle]" :title="vendor.name" @click="emit('update:vendorFilter', vendor.name)">
            <img v-if="vendor.slug && !iconErrors.has(vendor.slug)" class="h-3.5 w-3.5 rounded-full" :src="vendorIconUrl(vendor.slug)" :alt="vendor.name" loading="lazy" @error="iconFailed(vendor.slug)">
            <span>{{ vendor.name }}</span>
            <span class="text-[10px] opacity-70">{{ vendor.count }}</span>
          </button>
        </div>
      </section>

      <section class="flex flex-col gap-2">
        <button type="button" class="flex items-center justify-between text-sm font-medium" @click="toggleSection('quota')">
          <span>{{ t('msPricingType') }}</span>
          <ChevronDown :size="15" class="text-muted-foreground transition-transform" :class="{ '-rotate-90': collapsed.quota }" />
        </button>
        <div v-show="!collapsed.quota" class="flex flex-wrap gap-1.5">
          <button v-for="option in quotaOptions" :key="option.value" type="button" :class="[chipClass, props.quotaTypeFilter === option.value ? chipActive : chipIdle]" @click="emit('update:quotaTypeFilter', option.value)">
            <span>{{ option.label }}</span>
            <span class="text-[10px] opacity-70">{{ option.count }}</span>
          </button>
        </div>
      </section>
    </div>
  </aside>
</template>
