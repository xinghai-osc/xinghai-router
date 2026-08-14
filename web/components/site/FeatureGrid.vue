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
  <section class="shell py-20 md:py-24">
    <div class="max-w-2xl space-y-3">
      <p class="text-2xs font-medium tracking-wide text-clay uppercase">{{ t('site.featuresEyebrow') }}</p>
      <h2 class="display text-4xl text-ink md:text-5xl">{{ t('site.featuresTitle') }}</h2>
      <p class="text-muted">{{ t('site.featuresLead') }}</p>
    </div>

    <div class="mt-12 grid gap-px overflow-hidden rounded-card border border-line bg-line sm:grid-cols-2 lg:grid-cols-3">
      <article
        v-for="feature in features"
        :key="feature.key"
        class="group space-y-3 bg-surface p-6 transition-colors duration-150 hover:bg-sunken"
      >
        <div class="flex size-9 items-center justify-center rounded-[10px] bg-clay-soft text-clay transition-transform duration-150 ease-out group-hover:scale-110">
          <component :is="feature.icon" class="size-[18px]" />
        </div>
        <h3 class="text-[15px] font-semibold text-ink">{{ t(`site.feature${feature.key}Title`) }}</h3>
        <p class="text-[13px] leading-relaxed text-muted">{{ t(`site.feature${feature.key}Body`) }}</p>
      </article>
    </div>
  </section>
</template>
