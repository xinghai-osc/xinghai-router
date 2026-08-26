<script setup lang="ts">
import { ArrowRight, Check } from 'lucide-vue-next'

const props = defineProps<{ modelCount?: number }>()
const { t } = useI18n()

const countCopy = computed(() =>
  props.modelCount
    ? t('site.heroBodyCount', { count: props.modelCount })
    : t('site.heroBodyNoCount'))
</script>

<template>
  <section class="relative isolate overflow-hidden border-b border-line">
    <div class="pointer-events-none absolute inset-0 bg-sunken/25" aria-hidden="true" />
    <div
      class="pointer-events-none absolute inset-0 opacity-[0.06]"
      style="background-image: linear-gradient(var(--ink) 1px, transparent 1px), linear-gradient(90deg, var(--ink) 1px, transparent 1px); background-size: 90px 90px; mask-image: radial-gradient(ellipse at center, black 20%, transparent 78%); -webkit-mask-image: radial-gradient(ellipse at center, black 20%, transparent 78%)"
      aria-hidden="true"
    />

    <div class="shell relative grid min-h-[calc(100svh-4rem)] items-center gap-16 py-20 md:py-24 lg:grid-cols-[minmax(0,0.94fr)_minmax(30rem,1.06fr)] lg:gap-20">
      <div class="flex flex-col items-start">
        <p class="animate-hero-enter inline-flex items-center gap-2 rounded-full border border-line-strong bg-surface/60 px-3 py-1.5 text-[13px] font-medium tracking-wide text-muted backdrop-blur-md">
          <span class="size-1.5 rounded-full bg-clay" aria-hidden="true" />
          {{ t('site.heroBadge') }}
        </p>

        <h1 class="display animate-hero-enter mt-5 max-w-2xl text-[2.75rem] leading-[1.08] text-ink sm:text-6xl lg:text-[4rem]" style="animation-delay: 60ms">
          {{ t('site.heroTitleLine1') }}<br>{{ t('site.heroTitleLine2') }}
        </h1>

        <p class="animate-hero-enter mt-8 max-w-xl text-base leading-7 text-muted md:text-lg" style="animation-delay: 120ms">
          {{ t('site.heroBodyLead') }}{{ countCopy }}{{ t('site.heroBodyTail') }}
        </p>

        <div class="animate-hero-enter mt-9 flex flex-wrap items-center gap-3" style="animation-delay: 180ms">
          <UiButton to="/auth?mode=register" size="lg" class="group">
            {{ t('site.heroPrimary') }}
            <ArrowRight class="size-4 transition-transform duration-150 group-hover:translate-x-0.5" />
          </UiButton>
          <UiButton to="/models" variant="secondary" size="lg">{{ t('site.heroSecondary') }}</UiButton>
        </div>

        <div class="animate-hero-enter mt-7 flex flex-wrap gap-x-5 gap-y-2 text-[13px] text-muted" style="animation-delay: 220ms">
          <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-success" />{{ t('site.featureProtocolTitle') }}</span>
          <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-success" />{{ t('site.featureFailoverTitle') }}</span>
        </div>
      </div>

      <div class="animate-hero-enter relative mx-auto w-full max-w-2xl lg:mx-0 lg:translate-y-8" style="animation-delay: 240ms">
        <div class="relative rounded-[1.5rem] border border-line-strong bg-surface/60 p-2 shadow-pop backdrop-blur-xl">
          <div class="flex items-center gap-1 px-3 pt-1 pb-3 text-2xs text-muted">
            <span class="rounded-full bg-surface px-3 py-1.5 font-medium text-ink shadow-sm">{{ t('site.featureMultiTitle') }}</span>
            <span class="px-3 py-1.5">{{ t('site.featureProtocolTitle') }}</span>
          </div>
          <SiteCodeSample />
        </div>
        <div class="relative mt-4 flex items-center justify-end gap-2 pr-2 text-2xs text-muted">
          <span class="size-2 rounded-full bg-success" />
          {{ t('site.featureFailoverTitle') }}
        </div>
      </div>
    </div>
  </section>
</template>
