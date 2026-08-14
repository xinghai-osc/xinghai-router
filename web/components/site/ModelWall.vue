<script setup lang="ts">
import { ArrowRight } from 'lucide-vue-next'
import type { SquareModel } from '~/src/marketplace'
import { extractVendors } from '~/src/marketplace'

const props = withDefaults(defineProps<{
  models: SquareModel[]
  loading?: boolean
  maxVendors?: number
  perVendor?: number
}>(), { maxVendors: 6, perVendor: 5 })

const { t } = useI18n()

const vendors = computed(() => {
  const ordered = extractVendors(props.models).slice(0, props.maxVendors)
  return ordered.map(vendor => ({
    ...vendor,
    models: props.models.filter(model => model.vendor_name === vendor.name).slice(0, props.perVendor),
  }))
})
</script>

<template>
  <section class="border-y border-line bg-sunken/50">
    <div class="shell py-20 md:py-24">
      <div class="flex flex-wrap items-end justify-between gap-4">
        <div class="max-w-xl space-y-3">
          <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.wallEyebrow') }}</p>
          <h2 class="display text-4xl text-ink md:text-5xl">{{ t('site.wallTitle') }}</h2>
          <p class="text-muted">{{ t('site.wallLead') }}</p>
        </div>
        <UiButton to="/models" variant="secondary">
          {{ t('site.wallCta') }}
          <ArrowRight class="size-4" />
        </UiButton>
      </div>

      <div v-if="loading" class="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="index in 6" :key="index" class="rounded-card border border-line bg-surface p-5">
          <UiSkeleton :rows="4" />
        </div>
      </div>

      <UiEmptyState
        v-else-if="!vendors.length"
        class="mt-12"
        :title="t('site.wallEmptyTitle')"
        :description="t('site.wallEmptyBody')"
      />

      <div v-else class="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <article
          v-for="vendor in vendors"
          :key="vendor.name"
          class="group space-y-4 rounded-card border border-line bg-surface p-5 transition-[border-color,transform,background-color] duration-150 ease-out hover:-translate-y-0.5 hover:border-line-strong hover:bg-surface"
        >
          <header class="flex items-center gap-3">
            <span class="transition-transform duration-150 ease-out group-hover:scale-105">
              <SiteVendorMark :name="vendor.name" :slug="vendor.slug" />
            </span>
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-ink">{{ vendor.name }}</p>
              <p class="numeric text-2xs text-faint">{{ t('site.wallModelCount', { count: vendor.count }) }}</p>
            </div>
          </header>

          <ul class="space-y-1.5">
            <li
              v-for="model in vendor.models"
              :key="model.id"
              class="truncate font-mono text-[12.5px] text-muted transition-colors duration-150 group-hover:text-ink"
            >{{ model.model }}</li>
          </ul>
        </article>
      </div>
    </div>
  </section>
</template>
