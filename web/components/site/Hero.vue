<script setup lang="ts">
import { ArrowDown, ArrowRight, ChevronDown, Code2, KeyRound, Layers, Route } from 'lucide-vue-next'
import type { SquareModel } from '~/src/marketplace'

const props = withDefaults(defineProps<{
  models?: SquareModel[]
  catalogLoading?: boolean
}>(), { models: () => [], catalogLoading: false })

const { t, locale } = useI18n()
const modelCount = computed(() => props.models.length)
const countCopy = computed(() =>
  modelCount.value
    ? t('site.heroBodyCount', { count: modelCount.value })
    : t('site.heroBodyNoCount'))
const heroBody = computed(() => [
  t('site.heroBodyLead'),
  countCopy.value,
  t('site.heroBodyTail'),
].join(locale.value === 'en' ? ' ' : ''))

const capabilities = [
  { icon: KeyRound, key: 'featureMultiTitle' },
  { icon: Code2, key: 'featureProtocolTitle' },
  { icon: Route, key: 'featureFailoverTitle' },
]
</script>

<template>
  <section class="hero-surface relative isolate overflow-hidden border-b border-line">
    <div class="hero-grid pointer-events-none absolute inset-0 -z-20" aria-hidden="true" />

    <div class="shell">
      <div class="grid items-center gap-12 py-16 sm:py-20 lg:grid-cols-[minmax(0,0.92fr)_minmax(30rem,1.08fr)] lg:gap-14 lg:py-24 xl:gap-20">
        <div class="flex min-w-0 flex-col items-start">
          <p class="animate-hero-enter inline-flex items-center gap-2.5 rounded-full border border-clay/20 bg-clay-soft/80 px-3.5 py-1.5 text-xs font-medium text-clay">
            <span class="relative flex size-2" aria-hidden="true">
              <span class="absolute inline-flex size-full animate-ping rounded-full bg-clay opacity-40 motion-reduce:animate-none" />
              <span class="relative inline-flex size-2 rounded-full bg-clay" />
            </span>
            {{ t('site.heroBadge') }}
          </p>

          <h1 class="animate-hero-enter mt-7 max-w-2xl text-balance font-sans text-5xl font-semibold leading-[0.98] tracking-normal text-ink sm:text-6xl xl:text-7xl [animation-delay:80ms]">
            <span class="block">{{ t('site.heroTitleLine1') }}</span>
            <span class="relative mt-2 inline-block text-clay">
              {{ t('site.heroTitleLine2') }}<span aria-hidden="true">.</span>
              <span class="absolute right-0 -bottom-2 left-1 h-px bg-gradient-to-r from-clay/70 to-transparent" aria-hidden="true" />
            </span>
          </h1>

          <p class="animate-hero-enter mt-8 max-w-xl text-pretty text-base leading-8 text-muted sm:text-[17px] [animation-delay:160ms]">
            {{ heroBody }}
          </p>

          <div class="animate-hero-enter mt-8 flex w-full flex-col gap-3 sm:w-auto sm:flex-row [animation-delay:240ms]">
            <UiButton to="/auth?mode=register" size="lg" class="group justify-center px-7">
              {{ t('site.heroPrimary') }}
              <ArrowRight class="size-4 transition-transform duration-150 ease-out group-hover:translate-x-0.5 motion-reduce:transform-none" aria-hidden="true" />
            </UiButton>
            <UiButton to="/models" variant="secondary" size="lg" class="justify-center px-7">
              <Layers class="size-4 text-clay" aria-hidden="true" />
              {{ t('site.heroSecondary') }}
            </UiButton>
          </div>

          <ul class="animate-hero-enter mt-8 flex flex-wrap gap-x-5 gap-y-3 [animation-delay:320ms]">
            <li v-for="capability in capabilities" :key="capability.key" class="inline-flex items-center gap-2 text-xs text-muted">
              <span class="flex size-5 items-center justify-center rounded-full bg-clay-soft text-clay">
                <component :is="capability.icon" class="size-3" aria-hidden="true" />
              </span>
              {{ t(`site.${capability.key}`) }}
            </li>
          </ul>
        </div>

        <div class="animate-hero-enter relative min-w-0 [animation-delay:180ms]">
          <div class="pointer-events-none absolute -inset-5 -z-10 rounded-[32px] border border-clay/10 bg-clay-soft/25" aria-hidden="true" />
          <SiteGatewayPreview :models="models" :loading="catalogLoading" />

          <details class="group mt-3 overflow-hidden rounded-card border border-line bg-surface/75">
            <summary class="flex min-h-12 cursor-pointer list-none items-center justify-between gap-3 px-4 py-3 text-xs font-medium text-muted transition-colors duration-150 ease-out hover:text-ink [&::-webkit-details-marker]:hidden">
              <span class="inline-flex items-center gap-2.5">
                <Code2 class="size-4 text-clay" aria-hidden="true" />
                {{ t('site.heroIntegration') }}
              </span>
              <span class="inline-flex items-center gap-2 text-clay">
                {{ t('site.heroViewCode') }}
                <ChevronDown class="size-3.5 transition-transform duration-150 ease-out group-open:rotate-180" aria-hidden="true" />
              </span>
            </summary>
            <div class="border-t border-line p-3">
              <SiteCodeSample />
            </div>
          </details>
        </div>
      </div>

      <div class="flex flex-col gap-3 border-t border-line/80 py-5 sm:flex-row sm:items-center sm:justify-between">
        <p class="flex items-center gap-2.5 text-xs text-muted">
          <KeyRound class="size-3.5 text-clay" aria-hidden="true" />
          {{ t('site.heroIntegrationHint') }}
        </p>
        <a href="#features" class="group inline-flex min-h-11 shrink-0 items-center gap-2 self-start rounded-control text-xs font-medium text-muted transition-colors duration-150 ease-out hover:text-clay sm:self-auto">
          {{ t('site.heroExplore') }}
          <ArrowDown class="size-3.5 transition-transform duration-150 ease-out group-hover:translate-y-0.5 motion-reduce:transform-none" aria-hidden="true" />
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero-surface {
  background: linear-gradient(135deg, var(--paper) 20%, color-mix(in srgb, var(--clay-soft) 42%, var(--paper)) 100%);
}

.hero-grid {
  background-image:
    linear-gradient(color-mix(in srgb, var(--line) 45%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in srgb, var(--line) 45%, transparent) 1px, transparent 1px);
  background-size: 56px 56px;
  mask-image: linear-gradient(to bottom, transparent, var(--ink) 18%, transparent 88%);
  opacity: 0.55;
}
</style>
