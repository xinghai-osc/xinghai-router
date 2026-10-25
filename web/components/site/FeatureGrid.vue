<script setup lang="ts">
import { Activity, ArrowDown, ArrowRight, BarChart3, Check, Code2, GitBranch, KeyRound, Layers, RefreshCw, ShieldCheck, Wallet, Zap } from 'lucide-vue-next'
import { extractVendors, type SquareModel } from '~/src/marketplace'

const props = withDefaults(defineProps<{ models?: SquareModel[] }>(), { models: () => [] })
const { t } = useI18n()

const vendorPositions = [
  'left-[5%] top-[9%]',
  'right-[5%] top-[9%]',
  'bottom-[9%] left-[5%]',
  'bottom-[9%] right-[5%]',
]

const vendors = computed(() => {
  const result = extractVendors(props.models).slice(0, 4).map(vendor => ({ ...vendor }))
  while (result.length < 4) {
    result.push({ name: t('site.featureEndpoint', { count: result.length + 1 }), slug: '', count: 0 })
  }
  return result
})

const failoverSteps = [
  { icon: Check, key: 'featureFailoverStepPrimary' },
  { icon: RefreshCw, key: 'featureFailoverStepRetry' },
  { icon: Activity, key: 'featureFailoverStepRestore' },
]
const walletSteps = ['featureWalletReserve', 'featureWalletSettle', 'featureWalletRefund']
const usageDimensions = ['featureUsageRequests', 'featureUsageTokens', 'featureUsageCost']
const usageBars = [34, 58, 44, 76, 52, 88, 66, 96, 72, 84, 62, 92]
</script>

<template>
  <section id="features" class="shell scroll-mt-24 py-20 md:py-24" aria-labelledby="features-title">
    <div class="grid gap-5 md:grid-cols-[1.2fr_1fr] md:items-end md:gap-16">
      <div class="space-y-4">
        <p class="flex items-center gap-2.5 text-xs font-semibold tracking-wider text-clay uppercase">
          <span class="h-px w-6 bg-clay" aria-hidden="true" />
          {{ t('site.featuresEyebrow') }}
        </p>
        <h2 id="features-title" class="max-w-xl text-balance font-sans text-3xl font-semibold leading-tight tracking-normal text-ink sm:text-4xl lg:text-5xl">{{ t('site.featuresTitle') }}</h2>
      </div>
      <p class="max-w-lg text-pretty text-sm leading-7 text-muted">{{ t('site.featuresLead') }}</p>
    </div>

    <div class="mt-12 grid gap-4 lg:grid-cols-12">
      <article class="feature-card group flex flex-col rounded-card border border-line bg-surface p-6 transition-colors duration-150 ease-out hover:border-clay/30 sm:p-7 lg:col-span-7 lg:row-span-2">
        <div class="flex items-start justify-between gap-4">
          <span class="feature-icon inline-flex size-11 items-center justify-center rounded-[14px] bg-clay-soft text-clay">
            <Layers class="size-5" aria-hidden="true" />
          </span>
          <span class="numeric text-2xs text-faint">01</span>
        </div>
        <h3 class="mt-6 text-lg font-semibold tracking-tight text-ink">{{ t('site.featureMultiTitle') }}</h3>
        <p class="mt-2 max-w-lg text-pretty text-sm leading-7 text-muted">{{ t('site.featureMultiBody') }}</p>

        <div class="route-map relative mt-8 min-h-72 flex-1 overflow-hidden rounded-card border border-line bg-sunken/45" :aria-label="t('site.featureMultiVisual')">
          <svg class="absolute inset-0 size-full text-line-strong" viewBox="0 0 600 290" preserveAspectRatio="none" aria-hidden="true">
            <path d="M160 70 L300 145 L440 70 M160 220 L300 145 L440 220" fill="none" stroke="currentColor" stroke-width="1" stroke-dasharray="4 5" vector-effect="non-scaling-stroke" />
          </svg>
          <div class="absolute top-1/2 left-1/2 flex -translate-x-1/2 -translate-y-1/2 flex-col items-center">
            <span class="absolute size-24 rounded-full bg-clay-soft/70 blur-xl" aria-hidden="true" />
            <span class="relative flex size-14 items-center justify-center rounded-[18px] border border-clay/30 bg-clay text-clay-ink">
              <KeyRound class="size-6" aria-hidden="true" />
            </span>
            <span class="relative mt-2 font-mono text-[10px] font-semibold tracking-wider text-clay">API KEY</span>
          </div>
          <div
            v-for="(vendor, index) in vendors"
            :key="`${vendor.name}-${index}`"
            :class="['absolute flex w-[43%] max-w-48 items-center gap-2.5 rounded-control border border-line bg-surface px-3 py-2.5', vendorPositions[index]]"
          >
            <SiteVendorMark :name="vendor.name" :slug="vendor.slug" size="sm" />
            <span class="min-w-0">
              <span class="block truncate text-xs font-medium text-ink" :title="vendor.name">{{ vendor.name }}</span>
              <span class="mt-0.5 block truncate text-[10px] text-faint">{{ vendor.count ? t('site.wallModelCount', { count: vendor.count }) : t('site.featureEndpointType') }}</span>
            </span>
          </div>
        </div>
      </article>

      <article class="feature-card rounded-card border border-line bg-surface p-6 transition-colors duration-150 ease-out hover:border-clay/30 sm:p-7 lg:col-span-5">
        <div class="flex items-start justify-between gap-4">
          <span class="feature-icon inline-flex size-11 items-center justify-center rounded-[14px] bg-clay-soft text-clay">
            <GitBranch class="size-5" aria-hidden="true" />
          </span>
          <span class="numeric text-2xs text-faint">02</span>
        </div>
        <h3 class="mt-5 text-base font-semibold tracking-tight text-ink">{{ t('site.featureProtocolTitle') }}</h3>
        <p class="mt-2 text-pretty text-sm leading-6 text-muted">{{ t('site.featureProtocolBody') }}</p>
        <div class="mt-5 space-y-2">
          <div class="flex items-center gap-2 rounded-control border border-line bg-sunken/55 px-3 py-2.5 font-mono text-[11px]">
            <span class="font-semibold text-clay">POST</span>
            <span class="truncate text-muted">/v1/chat/completions</span>
            <Check class="ml-auto size-3.5 shrink-0 text-success" aria-hidden="true" />
          </div>
          <div class="flex items-center gap-2 rounded-control border border-line bg-sunken/55 px-3 py-2.5 font-mono text-[11px]">
            <span class="font-semibold text-clay">POST</span>
            <span class="truncate text-muted">/v1/messages</span>
            <Check class="ml-auto size-3.5 shrink-0 text-success" aria-hidden="true" />
          </div>
        </div>
      </article>

      <article class="feature-card rounded-card border border-line bg-surface p-6 transition-colors duration-150 ease-out hover:border-clay/30 sm:p-7 lg:col-span-5">
        <div class="flex items-start justify-between gap-4">
          <span class="feature-icon inline-flex size-11 items-center justify-center rounded-[14px] bg-clay-soft text-clay">
            <ShieldCheck class="size-5" aria-hidden="true" />
          </span>
          <span class="numeric text-2xs text-faint">03</span>
        </div>
        <h3 class="mt-5 text-base font-semibold tracking-tight text-ink">{{ t('site.featureFailoverTitle') }}</h3>
        <p class="mt-2 text-pretty text-sm leading-6 text-muted">{{ t('site.featureFailoverBody') }}</p>
        <ol class="mt-5 grid grid-cols-[1fr_auto_1fr_auto_1fr] items-center gap-1 rounded-control border border-line bg-sunken/45 p-3">
          <template v-for="(step, index) in failoverSteps" :key="step.key">
            <li class="flex min-w-0 flex-col items-center gap-2 text-center">
              <span class="flex size-8 items-center justify-center rounded-full bg-clay-soft text-clay">
                <component :is="step.icon" class="size-3.5" aria-hidden="true" />
              </span>
              <span class="text-[10px] leading-4 text-muted">{{ t(`site.${step.key}`) }}</span>
            </li>
            <ArrowRight v-if="index < failoverSteps.length - 1" class="size-3 shrink-0 text-faint" aria-hidden="true" />
          </template>
        </ol>
      </article>

      <article class="feature-card rounded-card border border-line bg-surface p-6 transition-colors duration-150 ease-out hover:border-clay/30 lg:col-span-4">
        <div class="flex items-start justify-between gap-4">
          <span class="feature-icon inline-flex size-10 items-center justify-center rounded-[13px] bg-clay-soft text-clay">
            <Wallet class="size-[18px]" aria-hidden="true" />
          </span>
          <span class="numeric text-2xs text-faint">04</span>
        </div>
        <h3 class="mt-5 text-base font-semibold tracking-tight text-ink">{{ t('site.featureWalletTitle') }}</h3>
        <p class="mt-2 text-pretty text-sm leading-6 text-muted">{{ t('site.featureWalletBody') }}</p>
        <ol class="mt-5 space-y-1">
          <li v-for="(step, index) in walletSteps" :key="step" class="flex items-center gap-3 rounded-control bg-sunken/45 px-3 py-2 text-xs text-muted">
            <span class="numeric flex size-5 shrink-0 items-center justify-center rounded-full bg-surface text-[10px] text-clay">{{ index + 1 }}</span>
            {{ t(`site.${step}`) }}
            <ArrowDown v-if="index < walletSteps.length - 1" class="ml-auto size-3 text-faint" aria-hidden="true" />
            <Check v-else class="ml-auto size-3.5 text-success" aria-hidden="true" />
          </li>
        </ol>
      </article>

      <article class="feature-card rounded-card border border-line bg-surface p-6 transition-colors duration-150 ease-out hover:border-clay/30 lg:col-span-4">
        <div class="flex items-start justify-between gap-4">
          <span class="feature-icon inline-flex size-10 items-center justify-center rounded-[13px] bg-clay-soft text-clay">
            <BarChart3 class="size-[18px]" aria-hidden="true" />
          </span>
          <span class="numeric text-2xs text-faint">05</span>
        </div>
        <h3 class="mt-5 text-base font-semibold tracking-tight text-ink">{{ t('site.featureUsageTitle') }}</h3>
        <p class="mt-2 text-pretty text-sm leading-6 text-muted">{{ t('site.featureUsageBody') }}</p>
        <div class="mt-5 rounded-control border border-line bg-sunken/45 p-3">
          <div class="flex h-16 items-end gap-1.5" aria-hidden="true">
            <span v-for="(height, index) in usageBars" :key="index" class="flex-1 rounded-t-[3px] bg-clay/25" :style="{ height: `${height}%` }">
              <span class="block h-1 rounded-t-[3px] bg-clay/70" />
            </span>
          </div>
          <div class="mt-3 flex flex-wrap gap-1.5">
            <span v-for="dimension in usageDimensions" :key="dimension" class="rounded-full border border-line bg-surface px-2 py-1 text-[10px] text-muted">{{ t(`site.${dimension}`) }}</span>
          </div>
        </div>
      </article>

      <article class="feature-card rounded-card border border-line bg-surface p-6 transition-colors duration-150 ease-out hover:border-clay/30 lg:col-span-4">
        <div class="flex items-start justify-between gap-4">
          <span class="feature-icon inline-flex size-10 items-center justify-center rounded-[13px] bg-clay-soft text-clay">
            <Zap class="size-[18px]" aria-hidden="true" />
          </span>
          <span class="numeric text-2xs text-faint">06</span>
        </div>
        <h3 class="mt-5 text-base font-semibold tracking-tight text-ink">{{ t('site.featurePricingTitle') }}</h3>
        <p class="mt-2 text-pretty text-sm leading-6 text-muted">{{ t('site.featurePricingBody') }}</p>
        <div class="mt-5 rounded-control border border-clay/20 bg-clay-soft/60 p-4">
          <Code2 class="size-4 text-clay" aria-hidden="true" />
          <p class="mt-3 text-sm font-medium leading-6 text-clay">{{ t('site.featurePricingFormula') }}</p>
          <p class="mt-2 text-xs leading-5 text-muted">{{ t('site.featurePricingTransparent') }}</p>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.feature-card {
  background-image: linear-gradient(145deg, color-mix(in srgb, var(--clay-soft) 24%, transparent), transparent 42%);
}

.feature-icon {
  transition: transform 150ms ease-out;
}

.feature-card:hover .feature-icon {
  transform: translateY(-2px);
}

.route-map {
  background-image:
    radial-gradient(circle at center, var(--clay-soft), transparent 58%),
    radial-gradient(circle, color-mix(in srgb, var(--line-strong) 70%, transparent) 1px, transparent 1px);
  background-size: 100% 100%, 18px 18px;
}

@media (prefers-reduced-motion: reduce) {
  .feature-icon {
    transition: none;
  }

  .feature-card:hover .feature-icon {
    transform: none;
  }
}
</style>
