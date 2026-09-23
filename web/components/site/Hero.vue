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

    <div class="shell relative">
      <div class="grid items-center gap-12 py-20 md:py-24 lg:min-h-[42rem] lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:gap-14">
        <div class="flex min-w-0 flex-col items-start">
          <p class="animate-hero-enter inline-flex items-center gap-2.5 rounded-full border border-clay/20 bg-clay-soft px-3.5 py-1.5 text-xs font-medium text-clay">
            <span class="size-1.5 shrink-0 rounded-full bg-clay" aria-hidden="true" />
            {{ t('site.heroBadge') }}
          </p>

          <h1 class="display animate-hero-enter mt-7 text-[clamp(2.75rem,5.5vw,4.75rem)] leading-[1.08] tracking-[-0.045em] text-ink" style="animation-delay: 60ms">
            {{ t('site.heroTitleLine1') }}<br>
            <span class="text-clay">{{ t('site.heroTitleLine2') }}</span><span class="text-clay" aria-hidden="true">.</span>
          </h1>

          <p class="animate-hero-enter mt-7 max-w-lg text-base leading-8 text-muted" style="animation-delay: 120ms">
            {{ t('site.heroBodyLead') }}{{ countCopy }}{{ t('site.heroBodyTail') }}
          </p>

          <div class="animate-hero-enter mt-8 flex w-full flex-col gap-3 min-[400px]:w-auto min-[400px]:flex-row" style="animation-delay: 180ms">
            <UiButton to="/auth?mode=register" size="lg" class="group justify-center">
              {{ t('site.heroPrimary') }}
              <ArrowRight class="size-4 transition-transform duration-150 ease-out group-hover:translate-x-0.5" aria-hidden="true" />
            </UiButton>
            <UiButton to="/models" variant="secondary" size="lg" class="justify-center">{{ t('site.heroSecondary') }}</UiButton>
          </div>

          <div class="animate-hero-enter mt-6 flex flex-wrap gap-x-5 gap-y-2 text-xs text-muted" style="animation-delay: 220ms">
            <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-clay" aria-hidden="true" />{{ t('site.featureProtocolTitle') }}</span>
            <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-clay" aria-hidden="true" />{{ t('site.featureFailoverTitle') }}</span>
          </div>
        </div>

        <div class="animate-hero-enter relative min-w-0" style="animation-delay: 240ms">
          <div class="rounded-[1.5rem] border border-line-strong bg-sunken/70 p-2 sm:p-3">
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
          <NuxtLink to="/models" class="group mt-3 flex items-center justify-between gap-3 rounded-card border border-line bg-surface/80 px-5 py-4 transition-colors duration-150 ease-out hover:border-clay/40 hover:bg-surface">
            <span class="flex items-center gap-3 text-sm text-muted">
              <Layers class="size-4 shrink-0 text-clay" aria-hidden="true" />
              {{ t('site.featureMultiTitle') }}
            </span>
            <ArrowRight class="size-4 shrink-0 text-faint transition-transform duration-150 ease-out group-hover:translate-x-0.5" aria-hidden="true" />
          </NuxtLink>
        </div>
      </div>

      <div class="flex flex-col gap-6 border-t border-line py-6 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-wrap gap-x-7 gap-y-3">
          <span v-for="capability in capabilities" :key="capability.key" class="inline-flex items-center gap-2 text-xs text-muted">
            <component :is="capability.icon" class="size-4 text-faint" aria-hidden="true" />
            {{ t(`site.${capability.key}`) }}
          </span>
        </div>
        <a href="#features" class="inline-flex shrink-0 items-center gap-2 text-xs text-muted transition-colors duration-150 ease-out hover:text-clay">
          {{ t('site.heroExplore') }}
          <ArrowDown class="size-3.5" aria-hidden="true" />
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero-surface {
  background: radial-gradient(ellipse at 85% 25%, var(--clay-soft), transparent 55%);
}

.hero-grid {
  background-image: linear-gradient(var(--line) 1px, transparent 1px), linear-gradient(90deg, var(--line) 1px, transparent 1px);
  background-size: 64px 64px;
  mask-image: linear-gradient(90deg, transparent 30%, var(--ink));
  opacity: 0.35;
}
</style>
