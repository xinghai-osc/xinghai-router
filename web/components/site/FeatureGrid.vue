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

    <div class="mt-12 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <article
        v-for="(feature, index) in features"
        :key="feature.key"
        :class="[
          'group relative overflow-hidden rounded-card border border-line bg-surface p-6 transition-[border-color,transform,background-color] duration-150 ease-out hover:-translate-y-0.5 hover:border-line-strong hover:bg-sunken',
          index === 0 || index === 5 ? 'sm:col-span-2 lg:col-span-1' : '',
        ]"
      >
        <span class="absolute top-4 right-5 font-mono text-2xs text-faint">0{{ index + 1 }}</span>
        <div class="flex size-10 items-center justify-center rounded-[10px] bg-clay-soft text-clay transition-transform duration-150 ease-out group-hover:scale-110">
          <component :is="feature.icon" class="size-[18px]" />
        </div>
        <h3 class="mt-5 text-[15px] font-semibold text-ink">{{ t(`site.feature${feature.key}Title`) }}</h3>
        <p class="mt-2 text-[13px] leading-relaxed text-muted">{{ t(`site.feature${feature.key}Body`) }}</p>
      </article>
    </div>
  </section>
</template>
