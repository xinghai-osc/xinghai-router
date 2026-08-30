<script setup lang="ts">
import { ChevronDown, RotateCcw } from 'lucide-vue-next'
import { FILTER_ALL, type ContextBucket, type Modality } from '~/src/marketplace'
import type { CatalogGroup } from '~/src/api'

const props = defineProps<{
  vendors: { name: string; slug: string; count: number }[]
  groups: CatalogGroup[]
  metadataAvailability?: { input: boolean; output: boolean; context: boolean }
}>()

const vendor = defineModel<string>('vendor', { required: true })
const group = defineModel<string>('group', { required: true })
const inputModalities = defineModel<Modality[]>('inputModalities', { default: () => [] })
const outputModalities = defineModel<Modality[]>('outputModalities', { default: () => [] })
const contextBuckets = defineModel<ContextBucket[]>('contextBuckets', { default: () => [] })
const emit = defineEmits<{ clear: [] }>()
const { t } = useI18n()

type Section = 'input' | 'output' | 'context' | 'vendor' | 'group'

const sectionOpen = reactive<Record<Section, boolean>>({
  input: true,
  output: true,
  context: true,
  vendor: true,
  group: true,
})

const modeOptions = computed(() => [
  { value: 'text', label: t('site.sqModeText') },
  { value: 'image', label: t('site.sqModeImage') },
  { value: 'audio', label: t('site.sqModeAudio') },
  { value: 'video', label: t('site.sqModeVideo') },
  { value: 'file', label: t('site.sqModeFile') },
])

const contextOptions = computed(() => [
  { value: '64k', label: t('site.sqContext64') },
  { value: '128k', label: t('site.sqContext128') },
  { value: '256k', label: t('site.sqContext256') },
  { value: '1m', label: t('site.sqContext1m') },
  { value: '1m-plus', label: t('site.sqContext1mPlus') },
])

const hasFilters = computed(() => vendor.value !== FILTER_ALL || group.value !== FILTER_ALL || inputModalities.value.length > 0 || outputModalities.value.length > 0 || contextBuckets.value.length > 0)

function toggleValue<T>(values: T[], value: T, checked: boolean): T[] {
  return checked ? [...new Set([...values, value])] : values.filter(item => item !== value)
}

function titleFor(section: Section) {
  const keys: Record<Section, string> = {
    input: 'site.sqFilterInputModes',
    output: 'site.sqFilterOutputModes',
    context: 'site.sqFilterContext',
    vendor: 'site.sqFilterVendors',
    group: 'site.sqFilterGroups',
  }
  return t(keys[section])
}

function toggle(section: Section) {
  sectionOpen[section] = !sectionOpen[section]
}

function selectVendor(name: string, checked: boolean) {
  vendor.value = checked ? name : FILTER_ALL
}

function selectGroup(id: string, checked: boolean) {
  group.value = checked ? id : FILTER_ALL
}

function clearFilters() {
  emit('clear')
}
</script>

<template>
  <aside class="w-full rounded-card border border-line bg-surface p-3 sm:p-4 lg:sticky lg:top-24 lg:max-h-[calc(100dvh-7rem)] lg:w-full lg:overflow-y-auto lg:overscroll-contain">
    <header class="flex items-center justify-between gap-3">
      <h2 class="text-sm font-semibold text-ink">{{ t('site.sqFilterTitle') }}</h2>
      <button
        type="button"
        class="inline-flex items-center gap-1 rounded-control px-2 py-1 text-2xs text-muted transition-colors hover:bg-sunken hover:text-ink disabled:pointer-events-none disabled:opacity-45"
        :disabled="!hasFilters"
        :aria-label="t('site.sqFilterClear')"
        @click="clearFilters"
      >
        <RotateCcw class="size-3" />
        {{ t('site.sqFilterClear') }}
      </button>
    </header>

    <div class="mt-2 divide-y divide-line sm:mt-3">
      <section>
        <button
          type="button"
          class="flex w-full items-center justify-between py-2.5 text-left text-[13px] font-medium text-ink sm:py-3"
          :aria-expanded="sectionOpen.input"
          aria-controls="sq-filter-input"
          @click="toggle('input')"
        >
          {{ titleFor('input') }}
          <ChevronDown class="size-4 text-faint transition-transform duration-150" :class="sectionOpen.input && 'rotate-180'" />
        </button>
        <div v-show="sectionOpen.input" id="sq-filter-input" class="grid grid-cols-2 gap-x-4 gap-y-2 pb-3 sm:block sm:space-y-2">
          <UiCheckbox
            v-for="option in modeOptions"
            :key="option.value"
            :model-value="inputModalities.includes(option.value as Modality)"
            :disabled="props.metadataAvailability?.input === false"
            :label="option.label"
            @update:model-value="value => inputModalities = toggleValue(inputModalities, option.value as Modality, value)"
          />
          <p v-if="props.metadataAvailability?.input === false" class="pt-1 text-2xs leading-relaxed text-faint">{{ t('site.sqFilterUnavailable') }}</p>
        </div>
      </section>

      <section>
        <button
          type="button"
          class="flex w-full items-center justify-between py-2.5 text-left text-[13px] font-medium text-ink sm:py-3"
          :aria-expanded="sectionOpen.output"
          aria-controls="sq-filter-output"
          @click="toggle('output')"
        >
          {{ titleFor('output') }}
          <ChevronDown class="size-4 text-faint transition-transform duration-150" :class="sectionOpen.output && 'rotate-180'" />
        </button>
        <div v-show="sectionOpen.output" id="sq-filter-output" class="grid grid-cols-2 gap-x-4 gap-y-2 pb-3 sm:block sm:space-y-2">
          <UiCheckbox
            v-for="option in modeOptions"
            :key="option.value"
            :model-value="outputModalities.includes(option.value as Modality)"
            :disabled="props.metadataAvailability?.output === false"
            :label="option.label"
            @update:model-value="value => outputModalities = toggleValue(outputModalities, option.value as Modality, value)"
          />
          <p v-if="props.metadataAvailability?.output === false" class="pt-1 text-2xs leading-relaxed text-faint">{{ t('site.sqFilterUnavailable') }}</p>
        </div>
      </section>

      <section>
        <button
          type="button"
          class="flex w-full items-center justify-between py-2.5 text-left text-[13px] font-medium text-ink sm:py-3"
          :aria-expanded="sectionOpen.context"
          aria-controls="sq-filter-context"
          @click="toggle('context')"
        >
          {{ titleFor('context') }}
          <ChevronDown class="size-4 text-faint transition-transform duration-150" :class="sectionOpen.context && 'rotate-180'" />
        </button>
        <div v-show="sectionOpen.context" id="sq-filter-context" class="grid grid-cols-2 gap-x-4 gap-y-2 pb-3 sm:block sm:space-y-2">
          <UiCheckbox
            v-for="option in contextOptions"
            :key="option.value"
            :model-value="contextBuckets.includes(option.value as ContextBucket)"
            :disabled="props.metadataAvailability?.context === false"
            :label="option.label"
            @update:model-value="value => contextBuckets = toggleValue(contextBuckets, option.value as ContextBucket, value)"
          />
          <p v-if="props.metadataAvailability?.context === false" class="pt-1 text-2xs leading-relaxed text-faint">{{ t('site.sqFilterUnavailable') }}</p>
        </div>
      </section>

      <section>
        <button
          type="button"
          class="flex w-full items-center justify-between py-2.5 text-left text-[13px] font-medium text-ink sm:py-3"
          :aria-expanded="sectionOpen.vendor"
          aria-controls="sq-filter-vendor"
          @click="toggle('vendor')"
        >
          {{ titleFor('vendor') }}
          <ChevronDown class="size-4 text-faint transition-transform duration-150" :class="sectionOpen.vendor && 'rotate-180'" />
        </button>
        <div v-show="sectionOpen.vendor" id="sq-filter-vendor" class="space-y-2 pb-3">
          <UiCheckbox :model-value="vendor === FILTER_ALL" :label="t('site.sqVendorChipsAll')" @update:model-value="vendor = FILTER_ALL" />
          <UiCheckbox
            v-for="item in props.vendors"
            :key="item.name"
            :model-value="vendor === item.name"
            :label="t('site.sqFilterVendorOption', { name: item.name, count: item.count })"
            @update:model-value="value => selectVendor(item.name, value)"
          />
          <p v-if="!props.vendors.length" class="text-2xs text-faint">{{ t('site.sqCatalogEmptyBody') }}</p>
        </div>
      </section>

      <section>
        <button
          type="button"
          class="flex w-full items-center justify-between py-2.5 text-left text-[13px] font-medium text-ink sm:py-3"
          :aria-expanded="sectionOpen.group"
          aria-controls="sq-filter-group"
          @click="toggle('group')"
        >
          {{ titleFor('group') }}
          <ChevronDown class="size-4 text-faint transition-transform duration-150" :class="sectionOpen.group && 'rotate-180'" />
        </button>
        <div v-show="sectionOpen.group" id="sq-filter-group" class="space-y-2 pb-1">
          <UiCheckbox :model-value="group === FILTER_ALL" :label="t('site.sqAllGroups')" @update:model-value="group = FILTER_ALL" />
          <UiCheckbox
            v-for="item in props.groups"
            :key="item.id"
            :model-value="group === item.id"
            :label="item.name"
            @update:model-value="value => selectGroup(item.id, value)"
          />
        </div>
      </section>
    </div>
  </aside>
</template>
