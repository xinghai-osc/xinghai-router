<script setup lang="ts">
import { FILTER_ALL } from '~/src/marketplace'

const props = defineProps<{
  vendors: { name: string; slug: string; count: number }[]
}>()

const vendor = defineModel<string>({ required: true })
const { t } = useI18n()

const visibleVendors = computed(() => props.vendors.slice(0, 8))

function selectVendor(name: string) {
  vendor.value = vendor.value === name ? FILTER_ALL : name
}
</script>

<template>
  <div class="-mx-1 overflow-x-auto px-1 pb-1" role="group" :aria-label="t('site.sqFilterbarLabel')">
    <div class="flex min-w-max items-center gap-2">
      <button
        type="button"
        class="inline-flex h-8 shrink-0 items-center rounded-control border px-3 text-[13px] transition-colors duration-150"
        :class="vendor === FILTER_ALL ? 'border-clay bg-clay-soft text-clay' : 'border-line-strong bg-surface text-muted hover:border-faint hover:text-ink'"
        :aria-pressed="vendor === FILTER_ALL"
        @click="vendor = FILTER_ALL"
      >
        {{ t('site.sqVendorChipsAll') }}
      </button>

      <button
        v-for="item in visibleVendors"
        :key="item.name"
        type="button"
        class="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-control border px-2.5 text-[13px] transition-colors duration-150"
        :class="vendor === item.name ? 'border-clay bg-clay-soft text-clay' : 'border-line-strong bg-surface text-muted hover:border-faint hover:text-ink'"
        :aria-label="t('site.sqVendorChipLabel', { name: item.name, count: item.count })"
        :aria-pressed="vendor === item.name"
        @click="selectVendor(item.name)"
      >
        <SiteVendorMark :name="item.name" :slug="item.slug" size="sm" />
        <span>{{ item.name }}</span>
        <span class="numeric text-2xs text-faint">{{ item.count }}</span>
      </button>
    </div>
  </div>
</template>
