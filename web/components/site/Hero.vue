<script setup lang="ts">
import { ArrowDown, ArrowRight, Code2, KeyRound, Layers, Route } from 'lucide-vue-next'

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
  <section class="hero-surface border-b border-line">
    <div class="shell">
      <div class="grid items-center gap-12 py-20 md:py-24 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] lg:gap-16">
        <div class="flex min-w-0 flex-col items-start">
          <p class="inline-flex items-center gap-2.5 rounded-full border border-clay/20 bg-clay-soft px-3.5 py-1.5 text-xs font-medium text-clay">
            <span class="size-1.5 shrink-0 rounded-full bg-clay" aria-hidden="true" />
            {{ t('site.heroBadge') }}
          </p>

          <h1 class="mt-7 max-w-xl text-balance font-sans text-[clamp(2.5rem,5.3vw,4.5rem)] font-semibold leading-[1.08] tracking-[-0.05em] text-ink">
            {{ t('site.heroTitleLine1') }}<br>
            <span class="text-clay">{{ t('site.heroTitleLine2') }}</span><span class="text-clay" aria-hidden="true">.</span>
          </h1>

          <p class="mt-6 max-w-lg text-pretty text-base leading-8 text-muted">
            {{ t('site.heroBodyLead') }}{{ countCopy }}{{ t('site.heroBodyTail') }}
          </p>

          <div class="mt-8 flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
            <UiButton to="/auth?mode=register" size="lg" class="group justify-center">
              {{ t('site.heroPrimary') }}
              <ArrowRight class="size-4 transition-transform duration-150 ease-out group-hover:translate-x-0.5 motion-reduce:transform-none" aria-hidden="true" />
            </UiButton>
            <UiButton to="/models" variant="secondary" size="lg" class="justify-center">{{ t('site.heroSecondary') }}</UiButton>
          </div>
        </div>

        <div class="min-w-0">
          <div class="rounded-card border border-line bg-surface p-3 sm:p-4">
            <div class="flex items-center justify-between gap-3 px-1 pb-4">
              <div class="flex min-w-0 items-center gap-2.5">
                <span class="flex size-9 shrink-0 items-center justify-center rounded-control bg-clay-soft text-clay">
                  <Code2 class="size-4" aria-hidden="true" />
                </span>
                <span class="text-sm font-medium text-ink">{{ t('site.heroIntegration') }}</span>
              </div>
              <span class="shrink-0 text-2xs text-muted">{{ t('site.heroExample') }}</span>
            </div>
            <SiteCodeSample />
            <p class="flex items-start gap-2 px-1 pt-4 text-xs leading-5 text-muted">
              <KeyRound class="mt-0.5 size-3.5 shrink-0 text-clay" aria-hidden="true" />
              {{ t('site.heroIntegrationHint') }}
            </p>
          </div>
          <NuxtLink to="/models" class="group mt-3 flex min-h-12 items-center justify-between gap-3 rounded-control px-3 py-2 text-sm text-muted transition-colors duration-150 ease-out hover:bg-clay-soft hover:text-clay focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-clay">
            <span class="flex items-center gap-3">
              <Layers class="size-4 shrink-0 text-clay" aria-hidden="true" />
              {{ t('site.featureMultiTitle') }}
            </span>
            <ArrowRight class="size-4 shrink-0 transition-transform duration-150 ease-out group-hover:translate-x-0.5 motion-reduce:transform-none" aria-hidden="true" />
          </NuxtLink>
        </div>
      </div>

      <div class="flex flex-col gap-4 border-t border-line py-5 md:flex-row md:items-center md:justify-between">
        <ul class="flex flex-wrap gap-x-7 gap-y-3">
          <li v-for="capability in capabilities" :key="capability.key" class="inline-flex items-center gap-2.5 text-xs text-muted">
            <component :is="capability.icon" class="size-4 shrink-0 text-clay" aria-hidden="true" />
            {{ t(`site.${capability.key}`) }}
          </li>
        </ul>
        <a href="#features" class="inline-flex min-h-11 shrink-0 items-center gap-2 self-start rounded-control text-xs font-medium text-muted transition-colors duration-150 ease-out hover:text-clay focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-clay md:self-auto">
          {{ t('site.heroExplore') }}
          <ArrowDown class="size-3.5" aria-hidden="true" />
        </a>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero-surface {
  background: radial-gradient(ellipse at 90% 15%, var(--clay-soft), transparent 55%), var(--paper);
}
</style>
