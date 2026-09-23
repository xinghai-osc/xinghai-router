<script setup lang="ts">
const { settings } = useSiteSettings()
const { t } = useI18n()
const { models, loading: catalogLoading, error: catalogError, loadCatalog } = useCatalog()
const { plans, loading: plansLoading, error: plansError, loadPlans } = usePlans()
const siteOrigin = useSiteOrigin()
const { target: catalogTarget, started: catalogStarted } = useDeferredLoad(() => { void loadCatalog() })
const { target: plansTarget, started: plansStarted } = useDeferredLoad(() => { void loadPlans() })

usePageSeo({
  title: () => `${settings.value.name} · ${t('common.tagline')}`,
  description: () => t('site.metaDescription'),
  structuredData: () => ({
    '@context': 'https://schema.org',
    '@type': 'WebSite',
    name: settings.value.name,
    url: siteOrigin.value,
    description: t('site.metaDescription'),
  }),
})

</script>

<template>
  <div>
    <SiteHero :model-count="models.length" />
    <SiteFeatureGrid />
    <div ref="catalogTarget">
      <SiteModelWall :models="models" :loading="catalogLoading || !catalogStarted" :error="catalogError" @retry="loadCatalog(true)" />
    </div>

    <section ref="plansTarget" class="relative overflow-hidden py-20 md:py-24">
      <div class="shell relative">
        <div class="grid gap-5 md:grid-cols-[1.2fr_1fr] md:items-end md:gap-16">
          <div class="space-y-4">
            <p class="flex items-center gap-2.5 text-xs font-medium tracking-wider text-clay uppercase">
              <span class="h-px w-6 bg-clay" aria-hidden="true" />
              {{ t('site.pricingEyebrow') }}
            </p>
            <h2 class="display text-4xl text-ink md:text-5xl">{{ t('site.pricingTitle') }}</h2>
          </div>
          <p class="max-w-lg text-sm leading-7 text-muted">{{ t('site.pricingLead') }}</p>
        </div>

        <div class="mt-12">
          <SitePlanCards :plans="plans" :loading="plansLoading || !plansStarted" :error="plansError" @retry="loadPlans(true)" />
        </div>
      </div>
    </section>

    <SiteCtaBand />
  </div>
</template>
