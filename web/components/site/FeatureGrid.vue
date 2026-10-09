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
        <h2 class="max-w-xl text-balance font-sans text-4xl font-semibold leading-tight tracking-[-0.04em] text-ink md:text-5xl">{{ t('site.featuresTitle') }}</h2>
      </div>
      <p class="max-w-lg text-sm leading-7 text-muted">{{ t('site.featuresLead') }}</p>
    </div>

    <div class="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <article
        v-for="(feature, index) in features"
        :key="feature.key"
        :class="[
          'feature-card group relative overflow-hidden rounded-card border p-6 hover:border-clay/35',
          index === 0 || index === 5 ? 'feature-card-accent border-clay/20 bg-clay-soft/40 lg:col-span-2 lg:p-8' : 'border-line bg-surface',
        ]"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="feature-icon flex size-12 items-center justify-center rounded-2xl border border-clay/15 bg-surface text-clay">
            <component :is="feature.icon" class="size-5" aria-hidden="true" />
          </div>
          <span class="numeric border-b border-line pb-2 text-2xs tracking-widest text-faint" aria-hidden="true">0{{ index + 1 }}</span>
        </div>
        <h3 class="mt-8 text-base font-semibold tracking-tight text-ink">{{ t(`site.feature${feature.key}Title`) }}</h3>
        <p class="mt-2 max-w-md text-[13px] leading-6 text-muted">{{ t(`site.feature${feature.key}Body`) }}</p>
      </article>
    </div>
  </section>
</template>

<style scoped>
.feature-card::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 2px;
  background: var(--clay);
  opacity: 0;
  transform: scaleY(0.4);
  transform-origin: center;
  transition: opacity 150ms ease-out, transform 150ms ease-out;
  content: '';
}

.feature-card-accent {
  background-image: radial-gradient(ellipse at top right, var(--clay-soft), transparent 70%);
}

.feature-card {
  transition: border-color 150ms ease-out, transform 150ms ease-out;
}

.feature-icon {
  transition: transform 150ms ease-out, background-color 150ms ease-out;
}

@media (hover: hover) and (pointer: fine) {
  .feature-card:hover {
    transform: translateY(-3px);
  }

  .feature-card:hover::before {
    opacity: 0.7;
    transform: scaleY(1);
  }

  .feature-card:hover .feature-icon {
    background-color: var(--clay-soft);
    transform: translateY(-2px) rotate(-4deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .feature-card,
  .feature-card::before,
  .feature-icon {
    transition: none;
    transform: none !important;
  }
}
</style>
