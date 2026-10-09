<script setup lang="ts">
import { ArrowDown, ArrowRight, Check, Code2, KeyRound, Layers, Route } from 'lucide-vue-next'

const props = defineProps<{ modelCount?: number }>()
const { t } = useI18n()

const countCopy = computed(() =>
  props.modelCount
    ? t('site.heroBodyCount', { count: props.modelCount })
    : t('site.heroBodyNoCount'))

const capabilities = [
  { icon: KeyRound, key: 'featureMultiTitle' },
  { icon: Code2, key: 'featureProtocolTitle' },
  { icon: Route, key: 'featureFailoverTitle' },
]
</script>

<template>
  <section class="hero-surface relative isolate overflow-hidden border-b border-line">
    <div class="hero-grid pointer-events-none absolute inset-0" aria-hidden="true" />
    <div class="hero-orbit pointer-events-none absolute right-0 top-12 hidden size-[38rem] translate-x-1/4 rounded-full border border-clay/10 lg:block" aria-hidden="true">
      <div class="absolute inset-10 rounded-full border border-clay/10" />
      <div class="absolute inset-20 rounded-full border border-clay/10" />
    </div>

    <div class="shell relative">
      <div class="grid items-center gap-12 py-20 md:py-24 lg:min-h-[44rem] lg:grid-cols-[minmax(0,1.08fr)_minmax(0,1fr)] lg:gap-12">
        <div class="flex min-w-0 flex-col items-start">
          <p class="hero-enter inline-flex items-center gap-2.5 rounded-full border border-clay/20 bg-clay-soft px-3.5 py-1.5 text-xs font-medium text-clay">
            <span class="size-1.5 shrink-0 rounded-full bg-clay" aria-hidden="true" />
            {{ t('site.heroBadge') }}
          </p>

          <h1 class="hero-enter mt-7 max-w-xl text-balance font-sans text-[clamp(2.75rem,5.3vw,4.5rem)] font-semibold leading-[1.1] tracking-[-0.055em] text-ink" style="animation-delay: 60ms">
            {{ t('site.heroTitleLine1') }}<br>
            <span class="hero-title-accent relative inline-block text-clay">{{ t('site.heroTitleLine2') }}</span><span class="text-clay" aria-hidden="true">.</span>
          </h1>

          <p class="hero-enter mt-6 max-w-lg text-base leading-8 text-muted" style="animation-delay: 120ms">
            {{ t('site.heroBodyLead') }}{{ countCopy }}{{ t('site.heroBodyTail') }}
          </p>

          <div class="hero-enter mt-8 flex w-full flex-col gap-3 min-[400px]:w-auto min-[400px]:flex-row" style="animation-delay: 180ms">
            <UiButton to="/auth?mode=register" size="lg" class="hero-action group justify-center">
              {{ t('site.heroPrimary') }}
              <ArrowRight class="hero-arrow size-4" aria-hidden="true" />
            </UiButton>
            <UiButton to="/models" variant="secondary" size="lg" class="hero-action justify-center">{{ t('site.heroSecondary') }}</UiButton>
          </div>

          <div class="hero-enter mt-6 flex flex-wrap gap-x-5 gap-y-2 text-xs text-muted" style="animation-delay: 220ms">
            <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-clay" aria-hidden="true" />{{ t('site.featureProtocolTitle') }}</span>
            <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-clay" aria-hidden="true" />{{ t('site.featureFailoverTitle') }}</span>
          </div>
        </div>

        <div class="hero-enter relative min-w-0 lg:pl-3" style="animation-delay: 240ms">
          <div class="hero-console relative overflow-hidden rounded-[1.5rem] border border-line-strong bg-sunken/80 p-2 sm:p-3">
            <div class="flex items-center justify-between gap-3 px-2 pt-1 pb-4 sm:px-3">
              <div class="flex min-w-0 items-center gap-2.5">
                <span class="flex size-8 shrink-0 items-center justify-center rounded-control border border-clay/20 bg-clay-soft text-clay">
                  <Code2 class="size-4" aria-hidden="true" />
                </span>
                <span class="text-sm font-medium text-ink">{{ t('site.heroIntegration') }}</span>
              </div>
              <span class="rounded-full border border-line bg-surface px-2.5 py-1 text-2xs text-muted">{{ t('site.heroExample') }}</span>
            </div>
            <SiteCodeSample />
            <div class="flex items-center gap-2 px-3 pt-3 pb-1 text-2xs text-muted">
              <KeyRound class="size-3.5 shrink-0 text-clay" aria-hidden="true" />
              {{ t('site.heroIntegrationHint') }}
            </div>
          </div>
          <NuxtLink to="/models" class="hero-model-link group mt-3 flex items-center justify-between gap-3 rounded-card border border-line bg-surface/80 px-5 py-4 hover:border-clay/40 hover:bg-surface focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-clay">
            <span class="flex items-center gap-3 text-sm text-muted">
              <Layers class="size-4 shrink-0 text-clay" aria-hidden="true" />
              {{ t('site.featureMultiTitle') }}
            </span>
            <ArrowRight class="hero-arrow size-4 shrink-0 text-faint" aria-hidden="true" />
          </NuxtLink>
        </div>
      </div>

      <div class="hero-enter flex flex-col gap-6 border-t border-line py-6 sm:flex-row sm:items-center sm:justify-between" style="animation-delay: 320ms">
        <div class="flex flex-wrap gap-x-7 gap-y-3">
          <span v-for="capability in capabilities" :key="capability.key" class="inline-flex items-center gap-2.5 text-xs text-muted">
            <component :is="capability.icon" class="size-4 text-clay" aria-hidden="true" />
            {{ t(`site.${capability.key}`) }}
          </span>
        </div>
        <a href="#features" class="hero-explore inline-flex shrink-0 items-center gap-2 rounded-control text-xs text-muted transition-colors duration-150 ease-out hover:text-clay focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-clay">
          {{ t('site.heroExplore') }}
          <ArrowDown class="hero-arrow size-3.5" aria-hidden="true" />
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero-surface {
  background: radial-gradient(ellipse at 85% 25%, var(--clay-soft), transparent 55%), linear-gradient(180deg, var(--paper), var(--surface));
}

.hero-grid {
  background-image: linear-gradient(var(--line) 1px, transparent 1px), linear-gradient(90deg, var(--line) 1px, transparent 1px);
  background-size: 80px 80px;
  mask-image: linear-gradient(90deg, transparent 30%, var(--ink));
  opacity: 0.25;
}

.hero-title-accent::after {
  position: absolute;
  right: 0;
  bottom: 0.04em;
  left: 0;
  height: 0.12em;
  border-radius: 100%;
  background: var(--clay);
  opacity: 0.12;
  content: '';
}

.hero-console::before {
  position: absolute;
  inset: 0 12% auto;
  height: 1px;
  background: linear-gradient(90deg, transparent, var(--clay), transparent);
  opacity: 0.65;
  content: '';
}

.hero-action,
.hero-model-link,
.hero-arrow {
  transition: transform 150ms ease-out, color 150ms ease-out, background-color 150ms ease-out, border-color 150ms ease-out;
}

.hero-action:focus-visible .hero-arrow,
.hero-model-link:focus-visible .hero-arrow {
  transform: translateX(3px);
}

@media (hover: hover) and (pointer: fine) {
  .hero-action:hover,
  .hero-model-link:hover {
    transform: translateY(-2px);
  }

  .hero-action:hover .hero-arrow,
  .hero-model-link:hover .hero-arrow {
    transform: translateX(3px);
  }

  .hero-explore:hover .hero-arrow {
    transform: translateY(3px);
  }
}

@media (prefers-reduced-motion: no-preference) {
  .hero-enter {
    animation: hero-reveal 640ms cubic-bezier(0.22, 1, 0.36, 1) both;
  }

  .hero-orbit {
    animation: hero-orbit-reveal 1100ms ease-out both;
  }
}

@keyframes hero-reveal {
  from {
    opacity: 0;
    transform: translateY(16px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes hero-orbit-reveal {
  from { opacity: 0; }
  to { opacity: 1; }
}

@media (prefers-reduced-motion: reduce) {
  .hero-action,
  .hero-model-link,
  .hero-arrow {
    transition: none;
    transform: none !important;
  }
}
</style>
