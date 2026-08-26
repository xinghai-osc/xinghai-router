<script setup lang="ts">
const { settings } = useSiteSettings()
const { t } = useI18n()
const { models, loading: catalogLoading, loadCatalog } = useCatalog()
const { plans, loading: plansLoading, loadPlans } = usePlans()

useHead({
  title: () => `${settings.value.name} · ${t('common.tagline')}`,
  meta: [{ name: 'description', content: () => t('site.metaDescription') }],
})

onMounted(() => {
  loadCatalog()
  loadPlans()
})
</script>

<template>
  <div>
    <SiteHero :model-count="models.length" />
    <SiteFeatureGrid />
    <SiteModelWall :models="models" :loading="catalogLoading" />

    <section class="relative overflow-hidden py-20 md:py-24">
      <div class="shell relative">
      <div class="max-w-2xl space-y-3">
        <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.pricingEyebrow') }}</p>
        <h2 class="display text-4xl text-ink md:text-5xl">{{ t('site.pricingTitle') }}</h2>
        <p class="text-muted">{{ t('site.pricingLead') }}</p>
      </div>

      <div class="mt-12">
        <SitePlanCards :plans="plans" :loading="plansLoading" />
      </div>
      </div>
    </section>

    <SiteCtaBand />
  </div>
</template>
