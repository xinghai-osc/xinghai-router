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
    <div
      class="pointer-events-none absolute -top-44 -right-40 h-[42rem] w-[42rem] rounded-full opacity-80 blur-3xl"
      style="background: radial-gradient(circle at 35% 45%, var(--clay-soft), transparent 64%)"
      aria-hidden="true"
    />
    <div
      class="pointer-events-none absolute -bottom-64 -left-40 h-[38rem] w-[38rem] rounded-full opacity-60 blur-3xl"
      style="background: radial-gradient(circle at 60% 35%, var(--clay-soft), transparent 66%)"
      aria-hidden="true"
    />
    <div class="pointer-events-none absolute inset-0 bg-sunken/35" aria-hidden="true" />
    <div
      class="pointer-events-none absolute inset-0 opacity-[0.045]"
      style="background-image: linear-gradient(var(--ink) 1px, transparent 1px), linear-gradient(90deg, var(--ink) 1px, transparent 1px); background-size: 72px 72px; mask-image: linear-gradient(to bottom, black, transparent 90%); -webkit-mask-image: linear-gradient(to bottom, black, transparent 90%)"
      aria-hidden="true"
    />

    <div class="shell relative grid min-h-[calc(100svh-4rem)] items-center gap-16 py-20 md:py-24 lg:grid-cols-[minmax(0,0.94fr)_minmax(30rem,1.06fr)] lg:gap-20">
      <div class="flex flex-col items-start">
        <p class="animate-rise text-[13px] font-medium tracking-wide text-muted">{{ t('site.heroBadge') }}</p>

        <h1 class="display animate-rise mt-5 max-w-2xl text-[3.5rem] leading-[0.96] tracking-[-0.04em] text-ink sm:text-7xl lg:text-[5.5rem]" style="animation-delay: 60ms">
          {{ t('site.heroTitleLine1') }}<br>{{ t('site.heroTitleLine2') }}
        </h1>

        <p class="animate-rise mt-8 max-w-xl text-base leading-7 text-muted md:text-lg" style="animation-delay: 120ms">
          {{ t('site.heroBodyLead') }}{{ countCopy }}{{ t('site.heroBodyTail') }}
        </p>

        <div class="animate-rise mt-9 flex flex-wrap items-center gap-3" style="animation-delay: 180ms">
          <UiButton to="/auth?mode=register" size="lg" class="group">
            {{ t('site.heroPrimary') }}
            <ArrowRight class="size-4 transition-transform duration-150 group-hover:translate-x-0.5" />
          </UiButton>
          <UiButton to="/models" variant="secondary" size="lg">{{ t('site.heroSecondary') }}</UiButton>
        </div>

        <div class="animate-rise mt-7 flex flex-wrap gap-x-5 gap-y-2 text-[13px] text-muted" style="animation-delay: 220ms">
          <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-success" />{{ t('site.featureProtocolTitle') }}</span>
          <span class="inline-flex items-center gap-1.5"><Check class="size-3.5 text-success" />{{ t('site.featureFailoverTitle') }}</span>
        </div>
      </div>

      <div class="animate-rise relative mx-auto w-full max-w-2xl lg:mx-0 lg:translate-y-8" style="animation-delay: 240ms">
        <div class="pointer-events-none absolute -inset-12 rounded-full bg-clay-soft/40 blur-3xl" aria-hidden="true" />
        <div class="relative rounded-[1.25rem] border border-line bg-surface/70 p-2 backdrop-blur-xl">
          <div class="flex items-center gap-1 px-3 pt-1 pb-3 text-2xs text-muted">
            <span class="rounded-control bg-surface px-3 py-1.5 font-medium text-ink">{{ t('site.featureMultiTitle') }}</span>
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
