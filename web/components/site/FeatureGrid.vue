<script setup lang="ts">
import { BarChart3, GitBranch, Layers, ShieldCheck, Wallet, Zap } from 'lucide-vue-next'

const { t } = useI18n()

const features = [
  { icon: Layers, key: 'Multi' },
  { icon: GitBranch, key: 'Protocol' },
  { icon: ShieldCheck, key: 'Failover' },
  { icon: Wallet, key: 'Wallet' },
  { icon: BarChart3, key: 'Usage' },
  { icon: Zap, key: 'Pricing' },
]
</script>

<template>
  <section id="features" class="shell scroll-mt-24 py-20 md:py-24" aria-labelledby="features-title">
    <div class="grid gap-5 md:grid-cols-[1.2fr_1fr] md:items-end md:gap-16">
      <div class="space-y-4">
        <p class="text-xs font-semibold tracking-wider text-clay uppercase">{{ t('site.featuresEyebrow') }}</p>
        <h2 id="features-title" class="max-w-xl text-balance font-sans text-3xl font-semibold leading-tight tracking-[-0.035em] text-ink sm:text-4xl lg:text-5xl">{{ t('site.featuresTitle') }}</h2>
      </div>
      <p class="max-w-lg text-pretty text-sm leading-7 text-muted">{{ t('site.featuresLead') }}</p>
    </div>

    <div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <article
        v-for="feature in features"
        :key="feature.key"
        class="rounded-card border border-line bg-surface p-6 lg:p-8"
      >
        <span class="inline-flex size-11 items-center justify-center rounded-control bg-clay-soft text-clay">
          <component :is="feature.icon" class="size-5" aria-hidden="true" />
        </span>
        <h3 class="mt-6 text-base font-semibold tracking-tight text-ink">{{ t(`site.feature${feature.key}Title`) }}</h3>
        <p class="mt-2 text-pretty text-sm leading-7 text-muted">{{ t(`site.feature${feature.key}Body`) }}</p>
      </article>
    </div>
  </section>
</template>
