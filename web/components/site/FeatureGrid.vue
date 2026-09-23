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
  <section id="features" class="shell scroll-mt-24 py-20 md:py-24">
    <div class="grid gap-5 md:grid-cols-[1.2fr_1fr] md:items-end md:gap-16">
      <div class="space-y-4">
        <p class="flex items-center gap-2.5 text-xs font-medium tracking-wider text-clay uppercase">
          <span class="h-px w-6 bg-clay" aria-hidden="true" />
          {{ t('site.featuresEyebrow') }}
        </p>
        <h2 class="display text-4xl text-ink md:text-5xl">{{ t('site.featuresTitle') }}</h2>
      </div>
      <p class="max-w-lg text-sm leading-7 text-muted">{{ t('site.featuresLead') }}</p>
    </div>

    <div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <article
        v-for="(feature, index) in features"
        :key="feature.key"
        :class="[
          'group relative overflow-hidden rounded-card border p-6 transition-colors duration-150 ease-out hover:border-clay/35',
          index === 0 || index === 5 ? 'border-clay/20 bg-clay-soft/40 lg:col-span-2 lg:p-8' : 'border-line bg-surface',
        ]"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="flex size-11 items-center justify-center rounded-xl border border-clay/15 bg-surface text-clay transition-transform duration-150 ease-out group-hover:-translate-y-0.5">
            <component :is="feature.icon" class="size-5" aria-hidden="true" />
          </div>
          <span class="numeric text-2xs text-faint" aria-hidden="true">0{{ index + 1 }}</span>
        </div>
        <h3 class="mt-7 text-base font-semibold tracking-tight text-ink">{{ t(`site.feature${feature.key}Title`) }}</h3>
        <p class="mt-2 max-w-md text-[13px] leading-6 text-muted">{{ t(`site.feature${feature.key}Body`) }}</p>
      </article>
    </div>
  </section>
</template>
